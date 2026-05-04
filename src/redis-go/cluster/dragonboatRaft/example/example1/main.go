package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/goutils/syncutil"
	"mumu.com/redis-go/cluster/dragonboatRaft"
)

type RequestType uint64

const (
	exampleShardID  uint64 = 128
	exampleShardID2 uint64 = 129
	GlobalShard     uint64 = 9999
)

const (
	PUT RequestType = iota
	GET
)

var (
	// initial nodes count is fixed to three, their addresses are also fixed
	//addresses = []string{
	//	"127.0.0.1:63001",
	//	"127.0.0.1:63002",
	//	"127.0.0.1:63003",
	//}
	addresses = map[uint64]string{
		1: "127.0.0.1:19001",
		2: "127.0.0.1:19002",
		3: "127.0.0.1:19003",
		4: "127.0.0.1:19004",
		5: "127.0.0.1:19005",
		6: "127.0.0.1:19006",
	}
)

func parseCommand(msg string) (RequestType, string, string, bool) {
	parts := strings.Split(strings.TrimSpace(msg), " ")
	if len(parts) == 0 || (parts[0] != "put" && parts[0] != "get") {
		return PUT, "", "", false
	}
	if parts[0] == "put" {
		if len(parts) != 3 {
			return PUT, "", "", false
		}
		return PUT, parts[1], parts[2], true
	}
	if len(parts) != 2 {
		return GET, "", "", false
	}
	return GET, parts[1], "", true
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage - \n")
	fmt.Fprintf(os.Stdout, "put key value\n")
	fmt.Fprintf(os.Stdout, "get key\n")
}

func main() {
	//go run main.go --replicaid 1
	//go run main.go --replicaid 2
	//go run main.go --replicaid 3
	//go run main.go --replicaid 4
	//go run main.go --replicaid 5
	//go run main.go --replicaid 6
	replicaID := flag.Int("replicaid", 1, "ReplicaID to use")
	addr := flag.String("addr", "", "Nodehost address")
	join := flag.Bool("join", false, "Joining a new node")
	flag.Parse()
	if len(*addr) == 0 && *replicaID != 1 && *replicaID != 2 && *replicaID != 3 && *replicaID != 4 && *replicaID != 5 && *replicaID != 6 {
		fmt.Fprintf(os.Stderr, "replica id must be 1, 2 or 3 when address is not specified\n")
		os.Exit(1)
	}
	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}
	initialMembers := make(map[uint64]string)
	if !*join {
		for idx, v := range addresses {
			initialMembers[idx] = v
		}
	}
	var nodeAddr string
	if len(*addr) != 0 {
		nodeAddr = *addr
	} else {
		nodeAddr = initialMembers[uint64(*replicaID)]
	}
	fmt.Fprintf(os.Stdout, "node address: %s\n", nodeAddr)
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)

	datadir := filepath.Join(
		"example-data",
		"helloworld-data",
		fmt.Sprintf("node%d", *replicaID))
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
		EnableMetrics:  true,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}
	//TODO:启动监控
	//dragonboatRaft.StartMonitor(*replicaID)
	// ======================
	// 启动：全局元数据 Shard 9999（所有6个节点都加入）
	// ======================
	go func() {
		for {
			initial := map[uint64]string{}
			if *replicaID == 1 {
				initial = addresses // 节点1初始化全部6个节点
			}
			err := nh.StartOnDiskReplica(
				initial,
				*replicaID != 1,
				dragonboatRaft.NewDiskKV,
				config.Config{
					ShardID:            GlobalShard,
					ReplicaID:          uint64(*replicaID),
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

	// ========== 启动 Shard1 (节点1,2,3) ==========
	if *replicaID <= 3 {
		rc := config.Config{
			ReplicaID:          uint64(*replicaID),
			ShardID:            exampleShardID,
			ElectionRTT:        10,
			HeartbeatRTT:       1,
			CheckQuorum:        true,
			SnapshotEntries:    10,
			CompactionOverhead: 5,
		}
		members := map[uint64]string{}
		members[1] = initialMembers[1]
		members[2] = initialMembers[2]
		members[3] = initialMembers[3]

		fmt.Printf("members======%v \n", members)
		if err := nh.StartOnDiskReplica(members, *join, dragonboatRaft.NewDiskKV, rc); err != nil {
			fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
			os.Exit(1)
		}
	}
	// ========== 启动 Shard2 (节点4,5,6) ==========
	if *replicaID >= 4 {
		rc2 := config.Config{
			ReplicaID:          uint64(*replicaID),
			ShardID:            exampleShardID2,
			ElectionRTT:        10,
			HeartbeatRTT:       1,
			CheckQuorum:        true,
			SnapshotEntries:    10,
			CompactionOverhead: 5,
		}
		members := map[uint64]string{}
		members[4] = initialMembers[4]
		members[5] = initialMembers[5]
		members[6] = initialMembers[6]
		fmt.Printf("members4======%v \n", members)
		if err := nh.StartOnDiskReplica(members, *join, dragonboatRaft.NewDiskKV, rc2); err != nil {
			fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
			os.Exit(1)
		}
	}
	raftStopper := syncutil.NewStopper()
	consoleStopper := syncutil.NewStopper()
	ch := make(chan string, 16)
	consoleStopper.RunWorker(func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			s, err := reader.ReadString('\n')
			if err != nil {
				close(ch)
				return
			}
			if s == "exit\n" {
				raftStopper.Stop()
				nh.Close()
				return
			}
			ch <- s
		}
	})
	printUsage()
	raftStopper.RunWorker(func() {
		//cs := nh.GetNoOPSession(exampleShardID)
		//cs2 := nh.GetNoOPSession(exampleShardID2)
		cs3 := nh.GetNoOPSession(GlobalShard)
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				msg := strings.Replace(v, "\n", "", 1)
				// input message must be in the following formats -
				// put key value
				// get key
				rt, key, val, ok := parseCommand(msg)
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid input\n")
					printUsage()
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				if rt == PUT {
					kv := &dragonboatRaft.KVData{
						Key: key,
						Val: val,
					}
					data, err := json.Marshal(kv)
					if err != nil {
						panic(err)
					}
					//_, err = nh.SyncPropose(ctx, cs, data)
					//if err != nil {
					//	fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
					//}
					//_, err = nh.SyncPropose(ctx, cs2, data)
					//if err != nil {
					//	fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
					//}
					_, err = nh.SyncPropose(ctx, cs3, data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
					}
				} else {
					//fmt.Printf("=====%v \n", key)
					//result, err := nh.SyncRead(ctx, exampleShardID, []byte(key))
					//if err != nil {
					//	fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
					//} else {
					//	fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
					//}
					//result, err = nh.SyncRead(ctx, exampleShardID2, []byte(key))
					//if err != nil {
					//	fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
					//} else {
					//	fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
					//}
					result, err := nh.SyncRead(ctx, GlobalShard, []byte(key))
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
					} else {
						fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
					}
				}
				cancel()
			case <-raftStopper.ShouldStop():
				return
			}
		}
	})

	//TODO:数据写入与查询
	go func() {
		// 等待 500ms 再试
		time.Sleep(5 * time.Second)
		dragonboatRaft.InitMeta(128, 1, "127.0.0.1", 19001, "128,129", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")
		dragonboatRaft.InitMeta(128, 2, "127.0.0.1", 19002, "128,129", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")
		nodeMeta2 := dragonboatRaft.InitMeta(128, 3, "127.0.0.1", 19003, "128,129", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")
		dragonboatRaft.InitMeta(129, 4, "127.0.0.1", 19004, "128,129", "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")
		dragonboatRaft.InitMeta(129, 5, "127.0.0.1", 19005, "128,129", "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")
		nodeMeta := dragonboatRaft.InitMeta(129, 6, "127.0.0.1", 19006, "128,129", "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006", "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006")

		//TODO:leader变更，修改槽对应新的leaderID
		go Start(nh, nodeMeta)
		//TODO: 初始化槽信息，首次平均分配槽
		// qfs_meta:9999:NodeSlotMetas
		dragonboatRaft.InitFullSlots(nh, nodeMeta)
		//TODO:测试数据写入
		//TODO:存储每个shardID: qfs_meta:9999:128 qfs_meta:9999:129
		//TODO:存储leaderID: qfs_meta:9999:129:ShardNodeLeaderID  qfs_meta:9999:128:ShardNodeLeaderID
		_, err := nodeMeta.Set(nh, nodeMeta, "qfs_meta:LeaderCount", "12345678")
		result, err := nodeMeta.Get(nh, nodeMeta, "qfs_meta:LeaderCount")
		if err != nil {
			fmt.Printf("1错误：%v \n", err)
		} else {
			fmt.Printf("query key: %s, result: %s\n", "qfs_meta:LeaderCount", result)
		}
		_, err = nodeMeta2.Set(nh, nodeMeta, "qfs_meta:aaaa", "1,2,3")
		//_, err = nodeMeta.Set(nh, nodeMeta, "qfs_meta:LeaderIDS", "1,2,3")
		//_, err = nodeMeta.Set(nh, nodeMeta, "qfs_meta:LeaderInfo:1", "127.0.0.1:10001")
		//if err != nil {
		//	fmt.Printf("2错误%v \n", err)
		//} else {
		//	fmt.Printf("正确%v \n", set)
		//}
		//TODO:这里会覆盖全局分槽信息，128,129
		//set, err = nodeMeta2.SetMeta(nh, nodeMeta2)
		//if err != nil {
		//	fmt.Printf("55错误%v \n", err)
		//} else {
		//	fmt.Printf("正确%v \n", set)
		//}
		//set, err = nodeMeta.SetMeta(nh, nodeMeta)
		//if err != nil {
		//	fmt.Printf("5错误%v \n", err)
		//} else {
		//	fmt.Printf("正确%v \n", set)
		//}

		result1, err1 := nodeMeta.GetMeta(nh, nodeMeta, "qfs_meta:9999:128")
		if err1 != nil {
			fmt.Printf("6错误：%v \n", err)
		} else {
			fmt.Printf("query key: %s, result: %v result1: %v \n", "qfs_meta:9999:128", result1.ShardID, result1.Slots)
		}
		result1, err1 = nodeMeta.GetMeta(nh, nodeMeta, "qfs_meta:9999:129")
		if err1 != nil {
			fmt.Printf("7错误：%v \n", err)
		} else {
			fmt.Printf("query key: %s, result: %v result1: %v \n", "qfs_meta:9999:129", result1.ShardID, result1.Slots)
		}
	}()

	//TODO 6. 打印角色
	//go func() {
	//	t := time.NewTicker(2 * time.Second)
	//	i := 0
	//	for range t.C {
	//		i++
	//		leaderID, term, isLeader, _ := nh.GetLeaderID(128)
	//		if isLeader {
	//			fmt.Printf("\n节点%d 【LEADER1】term=%d count=%d \n", leaderID, term, i)
	//		} else {
	//			fmt.Printf("\n 节点%d 【FOLLOWER1】| LeaderID=%d term=%d count=%d \n", leaderID, term, i)
	//		}
	//
	//		leaderID, term, isLeader, _ = nh.GetLeaderID(129)
	//		if isLeader {
	//			fmt.Printf("\n节点%d 【LEADER2】term=%d count=%d \n", leaderID, term, i)
	//		} else {
	//			fmt.Printf("\n 节点%d 【FOLLOWER2】| LeaderID=%d term=%d count=%d \n", leaderID, term, i)
	//		}
	//		leaderID, term, isLeader, _ = nh.GetLeaderID(9999)
	//		if isLeader {
	//			fmt.Printf("\n节点%d 【LEADER3】term=%d count=%d \n", leaderID, term, i)
	//		} else {
	//			fmt.Printf("\n 节点%d 【FOLLOWER3】| LeaderID=%d term=%d count=%d \n", leaderID, term, i)
	//		}
	//	}
	//
	//}()
	raftStopper.Wait()

}

// Start 修改leaderID
func Start(nh *dragonboat.NodeHost, nm *dragonboatRaft.NodeMeta) {
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
					nodeShardLeaderMeta := &dragonboatRaft.NodeShardLeaderMeta{
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
