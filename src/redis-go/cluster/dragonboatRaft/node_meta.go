package dragonboatRaft

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lni/dragonboat/v4"
)

// ====================== 数据结构 ======================
const QFS_META_KEY string = "qfs_meta"

type NodeShardLeaderMeta struct {
	ShardID  int `json:"shardID"`
	LeaderID int `json:"leaderID"`
}
type Slot struct {
	StartSlotID int //开始槽
	EndSlotID   int //结束槽
}
type NodeInfo struct {
	NodeID   int
	NodeIP   string
	NodePort int
	Status   string //在线:Online下线:Offline
}
type NodeMeta struct {
	GlobalShardID int               `json:"globalShardID"`
	ShardIDS      string            `json:"shardIDS"`
	AllNodeInfo   string            `json:"allNodeInfo"`
	ShardID       int               `json:"shardID"`
	LocalNodeInfo *NodeInfo         `json:"localNodeInfo"`
	ShardNodeInfo map[int]*NodeInfo `json:"shardNodeInfo"`
	Slots         map[int]*Slot     `json:"slots"`
}

func InitMeta(shardID int, nodeID int, nodeIP string, nodePort int, ShardIDS string, peers string, allNodeInfo string) *NodeMeta {
	nodeInfo := &NodeInfo{
		NodeID:   nodeID,
		NodeIP:   nodeIP,
		NodePort: nodePort,
		Status:   "Online",
	}
	nodeMeta := &NodeMeta{
		GlobalShardID: GlobalShard,
		ShardIDS:      ShardIDS,
		AllNodeInfo:   allNodeInfo,
		ShardID:       shardID,
		LocalNodeInfo: nodeInfo,
		ShardNodeInfo: make(map[int]*NodeInfo),
		Slots:         make(map[int]*Slot),
	}
	// 解析 peers 构建集群配置
	//TODO:存储集群shard一个组的所有节点
	//var shardIDSArr []string
	var shardNodeInfo = make(map[int]*NodeInfo)
	peerList := strings.Split(peers, ",")
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
		peerNodeInfo := &NodeInfo{
			NodeID:   peerNodeID,
			NodeIP:   peerNodeIP,
			NodePort: peerNodePort,
			Status:   "Online",
		}
		//fmt.Printf("111====%d======%v \n", peerNodeID, peerNodeInfo)
		shardNodeInfo[peerNodeID] = peerNodeInfo
		//shardIDSArr = append(shardIDSArr, strconv.Itoa(peerNodeID))
	}
	//fmt.Printf("shardIDSArr:%v \n", strings.Join(shardIDSArr, ","))
	nodeMeta.ShardNodeInfo = shardNodeInfo
	//nodeMeta.ShardIDS = strings.Join(shardIDSArr, ",")
	//fmt.Printf("nodeMeta.ShardIDS:%v \n", nodeMeta.ShardIDS)
	return nodeMeta
}

// Set 写入槽到Raft磁盘状态机
func (nm *NodeMeta) Set(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, key string, val string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	kv := &KVData{
		Key: key,
		Val: val,
	}
	data, _ := json.Marshal(kv)
	// 2. 获取 session
	session := nh.GetNoOPSession(uint64(nodeMeta.GlobalShardID))
	_, err := nh.SyncPropose(ctx, session, data)
	defer cancel()
	if err != nil {
		fmt.Printf("2======%v \n", err.Error())
		return false, err
	}
	return true, nil
}

// SetMeta 写入槽到Raft磁盘状态机
func (nm *NodeMeta) SetMeta(nh *dragonboat.NodeHost, nodeMeta *NodeMeta) (bool, error) {
	cmd := &NodeMeta{
		GlobalShardID: nodeMeta.GlobalShardID,
		AllNodeInfo:   nodeMeta.AllNodeInfo,
		ShardIDS:      nodeMeta.ShardIDS,
		ShardID:       nodeMeta.ShardID,
		ShardNodeInfo: nodeMeta.ShardNodeInfo,
		Slots:         nodeMeta.Slots,
	}
	data, _ := json.Marshal(cmd)
	key := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + strconv.Itoa(nodeMeta.ShardID)
	fmt.Printf("222=======%s \n", key)
	_, err := nm.Set(nh, nodeMeta, key, string(data))
	if err != nil {
		return false, err
	}
	return true, nil
}

// 查询槽
func (nm *NodeMeta) Get(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, key string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := nh.SyncRead(ctx, uint64(nodeMeta.GlobalShardID), []byte(key))

	defer cancel()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// 查询槽
func (nm *NodeMeta) GetMeta(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, key string) (*NodeMeta, error) {
	result, err := nm.Get(nh, nodeMeta, key)
	if err != nil {
		return nil, err
	}
	data, ok := result.([]byte)
	if !ok {
		fmt.Println("invalid data")
	}
	var meta NodeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (nm *NodeMeta) SetShardNodeLeaderID(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, shardID int, nodeShardLeaderMeta *NodeShardLeaderMeta) (bool, error) {
	key := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + strconv.Itoa(shardID) + ":" + "ShardNodeLeaderID"
	data, _ := json.Marshal(nodeShardLeaderMeta)
	_, err := nodeMeta.Set(nh, nodeMeta, key, string(data))
	if err != nil {
		return false, err
	}
	return true, nil
}
func (nm *NodeMeta) GetShardNodeLeaderID(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, shardID int) (*NodeShardLeaderMeta, error) {
	key := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + strconv.Itoa(shardID) + ":" + "ShardNodeLeaderID"
	result, err := nm.Get(nh, nodeMeta, key)
	if err != nil {
		return nil, err
	}
	data, ok := result.([]byte)
	if !ok {
		fmt.Println("invalid data")
	}
	var meta NodeShardLeaderMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// StartMetaWrite   元数据写入
func (nm *NodeMeta) StartMetaWrite(nh *dragonboat.NodeHost, nodeMeta *NodeMeta) {
	//tt := time.NewTicker(10 * time.Second)
	//for range tt.C {
	//	_, err := nodeMeta.SetMeta(nh, nodeMeta)
	//	if err != nil {
	//		fmt.Printf("1======%v", err.Error())
	//	}
	//}

	// 替换成这句（等待集群真正启动成功）
	WaitShardReady(nh, uint64(GlobalShard))
	_, err := nodeMeta.SetMeta(nh, nodeMeta)
	if err != nil {
		fmt.Printf("1======%v", err.Error())
	}
}
