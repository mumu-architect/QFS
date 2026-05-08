package dragonboatRaft

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
)

const (
	GlobalShard int = 9999
)

// NewDragonBoatRaftNode 创建一个新的 HashiCorp Raft 节点
func NewDragonBoatRaftNode(shardID int, nodeID int, shardNodeInfo string, allNodeInfo string) *dragonboat.NodeHost {
	allNodeInfos := formatNodeInfo(allNodeInfo)
	nh := setConfig(nodeID, allNodeInfos)
	//TODO:启动监控,生产环境必须开启
	StartMonitor(nodeID)
	// TODO:启动：全局元数据 Shard 9999（所有6个节点都加入）
	setGlobalShard(nh, nodeID, allNodeInfos)

	shardNodeInfos := formatNodeInfo(shardNodeInfo)
	// ========== 启动 Shard1 (节点1,2,3) ==========
	//========== 启动 Shard2 (节点4,5,6) ==========
	rc := config.Config{
		ReplicaID:          uint64(nodeID),
		ShardID:            uint64(shardID),
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	fmt.Printf("shardNodeInfos======%v \n", shardNodeInfos)
	if err := nh.StartOnDiskReplica(shardNodeInfos, false, NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}
	return nh
}

// 设置启动配置信息
func setConfig(nodeID int, nodeAddr map[uint64]string) *dragonboat.NodeHost {
	datadir := filepath.Join(
		"example-data",
		"helloworld-data",
		fmt.Sprintf("node%d", nodeID))
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr[uint64(nodeID)],
		EnableMetrics:  true,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}
	return nh
}

// 格式化所有节点
func formatNodeInfo(allNodeInfo string) map[uint64]string {
	allNodeInfos := make(map[uint64]string)
	peerList := strings.Split(allNodeInfo, ",")
	for _, peer := range peerList {
		parts := strings.Split(peer, "=")
		if len(parts) != 2 {
			continue
		}
		peerNodeID, _ := strconv.Atoi(parts[0])
		nodeAddr := parts[1]
		parts2 := strings.Split(nodeAddr, ":")
		if len(parts) != 2 {
			continue
		}
		peerNodeIP := parts2[0]
		peerNodePort, _ := strconv.Atoi(parts2[1])
		allNodeInfos[uint64(peerNodeID)] = peerNodeIP + ":" + strconv.Itoa(peerNodePort)
		//fmt.Printf("111====%d======%v \n", peerNodeID, peerNodeInfo)
	}
	return allNodeInfos
}

func setGlobalShard(nh *dragonboat.NodeHost, nodeID int, allNodeInfos map[uint64]string) {
	// TODO:启动：全局元数据 Shard 9999（所有6个节点都加入）
	minNodeID := MinMapKey(allNodeInfos)
	go func() {
		for {
			initial := map[uint64]string{}
			if nodeID == int(minNodeID) {
				initial = allNodeInfos // 节点1初始化全部6个节点
			}
			err := nh.StartOnDiskReplica(
				initial,
				nodeID != int(minNodeID),
				NewDiskKV,
				config.Config{
					ShardID:            uint64(GlobalShard),
					ReplicaID:          uint64(nodeID),
					ElectionRTT:        10,
					HeartbeatRTT:       1,
					CheckQuorum:        true,
					SnapshotEntries:    10,
					CompactionOverhead: 5,
				},
			)
			if err == nil {
				fmt.Println(" 全局元数据分片 999 启动成功")
				break
			}
			time.Sleep(1 * time.Second)
		}
	}()
}

// Start 修改leaderID
func Start(nh *dragonboat.NodeHost, nm *NodeMeta) {
	//go func() {
	tt := time.NewTicker(10 * time.Second)
	for range tt.C {
		parts := strings.Split(nm.ShardIDS, ",")
		fmt.Printf("parts============%v \n", parts)
		for _, id := range parts {
			shardID, _ := strconv.Atoi(id)
			leaderID, _, isLeader, _ := nh.GetLeaderID(uint64(shardID))
			if isLeader {
				//TODO:因为nm的shardID固定所有不会产生新的，全部客户端生成可解决
				if shardID == nm.ShardID {
					nodeShardLeaderMeta := &NodeShardLeaderMeta{
						ShardID:  shardID,
						LeaderID: int(leaderID),
					}
					_, err := nm.SetShardNodeLeaderID(nh, nm, shardID, nodeShardLeaderMeta)
					if err != nil {
						fmt.Printf("全局存储shardID的leaderID=======%s", err.Error())
					}
				}

			}
		}
	}
	//}()
}

// IsShardReady 检查 shard 是否真正启动成功（可读写）
func IsShardReady(nh *dragonboat.NodeHost, shardID uint64) bool {
	// 1. 先检查是否有 leader
	_, _, hasLeader, _ := nh.GetLeaderID(shardID)
	if !hasLeader {
		return false
	}
	// 2. 你当前版本的正确 ReadIndex 接口
	timeout := 1 * time.Second
	rs, err := nh.ReadIndex(shardID, timeout)
	if err != nil {
		return false
	}
	defer rs.Release()

	// 3. 等待 ReadIndex 完成（v4 正确方式：ResultC()）
	select {
	case <-rs.ResultC():
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitShardReady 阻塞等待直到 shard 启动成功（生产用）
func WaitShardReady(nh *dragonboat.NodeHost, shardID uint64) {
	for {
		if IsShardReady(nh, shardID) {
			fmt.Printf("shard %d 已启动成功，可正常读写\n", shardID)
			return
		}
		fmt.Printf("⏳ waiting shard %d ready...\n", shardID)
		time.Sleep(1 * time.Second)
	}
}
