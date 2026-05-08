package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lni/goutils/syncutil"
	"mumu.com/redis-go/cluster"
	"mumu.com/redis-go/cluster/dragonboatRaft"
	"mumu.com/redis-go/cluster/logManager"
)

type Node struct {
	ShardID      int
	NodeID       int
	ShardIDS     string
	IP           string
	Port         int
	Peers        string // shard下所有cluster集群节点 a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
	NodeInfo     string //所有集群节点a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
	RaftPort     int
	RaftPeers    string // shard下所有raft集群节点 a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
	RaftNodeInfo string //所有集群节点a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
	LogPort      int
	LogPeers     string // shard下所有log集群节点 a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
}
type RequestType uint64

const (
	PUT RequestType = iota
	GET
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
	// go run main.go --shardID 128 --id 1 --shardIDS "128,129" --ip 127.0.0.1 --port 9001 --peers "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19001  --raftPeers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8081 --logPeers "1=127.0.0.1:8081,2=127.0.0.1:8082,3=127.0.0.1:8083"
	// go run main.go --shardID 128 --id 2 --shardIDS "128,129" --ip 127.0.0.1 --port 9002 --peers "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19002  --raftPeers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8082 --logPeers "1=127.0.0.1:8081,2=127.0.0.1:8082,3=127.0.0.1:8083"
	// go run main.go --shardID 128 --id 3 --shardIDS "128,129" --ip 127.0.0.1 --port 9003 --peers "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19003  --raftPeers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8083 --logPeers "1=127.0.0.1:8081,2=127.0.0.1:8082,3=127.0.0.1:8083"
	// go run main.go --shardID 129 --id 4 --shardIDS "128,129" --ip 127.0.0.1 --port 9004 --peers "4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19004  --raftPeers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8084 --logPeers "4=127.0.0.1:8084,5=127.0.0.1:8085,6=127.0.0.1:8086"
	// go run main.go --shardID 129 --id 5 --shardIDS "128,129" --ip 127.0.0.1 --port 9005 --peers "4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19005  --raftPeers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8085 --logPeers "4=127.0.0.1:8084,5=127.0.0.1:8085,6=127.0.0.1:8086"
	// go run main.go --shardID 129 --id 6 --shardIDS "128,129" --ip 127.0.0.1 --port 9006 --peers "4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --nodeInfo "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006" --raftPort 19006  --raftPeers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --raftNodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --logPort 8086 --logPeers "4=127.0.0.1:8084,5=127.0.0.1:8085,6=127.0.0.1:8086"
	//
	/*
	 */
	var node Node
	flag.IntVar(&node.ShardID, "shardID", 0, "端口 1/2/3")
	flag.IntVar(&node.NodeID, "id", 0, "1/2/3")
	flag.StringVar(&node.ShardIDS, "shardIDS", "128,129", "ShardIDS")
	flag.StringVar(&node.IP, "ip", "127.0.0.1", "IP")
	flag.IntVar(&node.Port, "port", 0, "端口 9001/9002/9003")
	flag.StringVar(&node.Peers, "peers", "", "集群所有节点 1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003")
	flag.StringVar(&node.NodeInfo, "nodeInfo", "", "集群所有节点 1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003")

	flag.IntVar(&node.RaftPort, "raftPort", 0, "端口 19001/19002/19003")
	flag.StringVar(&node.RaftPeers, "raftPeers", "", "集群所有节点 1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003")
	flag.StringVar(&node.RaftNodeInfo, "raftNodeInfo", "", "集群所有节点 1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003")

	flag.IntVar(&node.LogPort, "logPort", 0, "端口 19001/19002/19003")
	flag.StringVar(&node.LogPeers, "logPeers", "", "集群所有节点 1=127.0.0.1:8081,2=127.0.0.1:8082,3=127.0.0.1:8083")

	flag.Parse()
	if node.ShardID == 0 || node.NodeID == 0 || node.ShardIDS == "" || node.IP == "" || node.Port == 0 || node.Peers == "" || node.NodeInfo == "" || node.RaftPort == 0 || node.RaftPeers == "" || node.RaftNodeInfo == "" || node.LogPort == 0 || node.LogPeers == "" {
		log.Fatal("必须输入--ShardID --NodeID --ShardIDS  --ip --port --peers  --NodeInfo --RaftPort --RaftPeers --raftNodeInfo --LogPort --LogPeers ")
	}

	nh := dragonboatRaft.NewDragonBoatRaftNode(node.ShardID, node.NodeID, node.RaftPeers, node.RaftNodeInfo)
	nodeMeta := dragonboatRaft.InitMeta(node.ShardID, node.NodeID, node.IP, node.RaftPort, node.ShardIDS, node.RaftPeers, node.RaftNodeInfo)
	//TODO: 初始化槽信息，首次平均分配槽
	//TODO: get qfs_meta:9999:NodeSlotMetas
	go dragonboatRaft.StartSlotWrite(nh, nodeMeta)
	//TODO:存储每个shardID: qfs_meta:9999:128 qfs_meta:9999:129
	go nodeMeta.StartMetaWrite(nh, nodeMeta)
	//TODO:leader变更，修改槽对应新的leaderID
	//TODO:存储leaderID: get qfs_meta:9999:129:ShardNodeLeaderID get qfs_meta:9999:128:ShardNodeLeaderID [已测]
	go dragonboatRaft.Start(nh, nodeMeta)

	//TODO:去leaderID
	//get qfs_meta:9999:129:ShardNodeLeaderID
	//query key: qfs_meta:9999:129:ShardNodeLeaderID, result: {"shardID":129,"leaderID":4}
	//GetShardNodeLeaderID
	//nodeMeta.GetShardNodeLeaderID(nh,nodeMeta,node.ShardID)
	//TODO:取槽信息
	//get qfs_meta:9999:NodeSlotMetas
	//query key: qfs_meta:9999:NodeSlotMetas, result: {"nodeSlotMetas":{"0":{"shardID":128,"slots":{"0":{"StartSlotID":0,"EndSlotID":8191}}},"1":{"shardID":129,"slots":{"0":{"StartSlotID":8192,"EndSlotID":16383}}}}}

	go func() {
		time.Sleep(5 * time.Second)
		dragonboatRaft.WaitShardReady(nh, uint64(node.ShardID))
		nodeSlotMetas, err := dragonboatRaft.GetSolt(nh, nodeMeta)
		if err != nil {
			fmt.Printf("1111====dragonboatRaft.GetSolt error:%v \n", err)
		}
		fmt.Printf("1111====dragonboatRaft.GetSolt data:%v \n", nodeSlotMetas)

		shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, node.ShardID)
		if err != nil {
			fmt.Printf("222====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
		}
		fmt.Printf("222====nodeMeta.GetShardNodeLeaderID data:%v \n", shardNodeLeaderID)

		//TODO:暂时的leaderID
		leaderId := 0
		masterAddr := ""
		if node.NodeID <= 3 {
			leaderId = 1
			masterAddr = "127.0.0.1:9001"
		}
		if node.NodeID >= 4 {
			leaderId = 3
			masterAddr = "127.0.0.1:9004"
		}
		//TODO:本地log管理
		nl, _ := logManager.NewNodeLog(node.ShardID, leaderId, node.NodeID, node.IP, node.LogPort, node.LogPeers)

		//TODO:集群管理
		cl := cluster.NewCluster(node.ShardID, leaderId, node.NodeID, node.IP, node.Port, node.Peers, node.NodeInfo, nodeSlotMetas, nh, nodeMeta, nl, masterAddr)

		//fmt.Printf("333====data: %v =======%v \n", cl.NodeAllSlotMetas.NodeSlotMetas[0].Slots, cl.NodeAllSlotMetas.NodeSlotMetas[1].Slots)

		//11524  5
		//TODO:根据槽取shardID,再根据shardID 动态取leaderID
		keySolt := 1232
		leaderId2 := dragonboatRaft.GetLeaderID(nh, nodeMeta, keySolt)
		fmt.Printf("4444====data: %v \n", leaderId2)
		keySolt = 16354
		leaderId2 = dragonboatRaft.GetLeaderID(nh, nodeMeta, keySolt)
		fmt.Printf("4444====data: %v \n", leaderId2)

		//TODO:重启分批加载本地log数据到内存
		err = cl.RestartBatchLoadLog()
		if err != nil {
			fmt.Printf("RestartBatchLoadLog:%v\n", err)
		}
		//TODO:拉去主节点的增量数据，并写入内存
		go cl.PullSyncLoop()

		//TODO:动态修改cluster中所有的leaderID
		go func() {
			tt := time.NewTicker(3 * time.Second)
			for range tt.C {
				//TODO:动态修改cluster中所有的leaderID
				shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, node.ShardID)
				if err != nil {
					fmt.Printf("222====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
				}
				fmt.Printf("222====nodeMeta.GetShardNodeLeaderID data:%v \n", shardNodeLeaderID)
				if shardNodeLeaderID == nil {
					continue
				}
				if cl.ShardID == node.ShardID {
					cl.LeaderID = shardNodeLeaderID.LeaderID
					cl.LocalNode.LeaderID = shardNodeLeaderID.LeaderID
				}

			}
		}()
		go func() {
			tt := time.NewTicker(3 * time.Second)
			for range tt.C {
				//TODO:动态修改cluster中所有的leaderID
				shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, node.ShardID)
				if err != nil {
					fmt.Printf("222====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
				}
				fmt.Printf("222====nodeMeta.GetShardNodeLeaderID data:%v \n", shardNodeLeaderID)
				if shardNodeLeaderID == nil {
					continue
				}
				if nl.ShardId == node.ShardID {
					nl.LeaderId = shardNodeLeaderID.LeaderID
				}
			}
		}()

	}()

	//根据槽获取leaderID
	//dragonboatRaft.GetLeaderID(nh, nodeMeta, 123)

	/*

		//TODO:重点，多个对象调用，global在对象内通用

		//TODO:集合真实数据存储cluster
		// 初始化数据目录（在storage.go中实现）
		cluster.InitDataDir()
		//TODO:leaderID从9999shard中获取  nodeMeta  GetShardNodeLeaderID   | qfs_meta:9999:129:ShardNodeLeaderID, result: {"shardID":129,"leaderID":4}
		//TODO:槽信息从9999shard中获取  nodeMeta GetSolt |  qfs_meta:9999:NodeSlotMetas, result: {"nodeSlotMetas":{"0":{"shardID":128,"slots":{"0":{"StartSlotID":0,"EndSlotID":8191}}},"1":{"shardID":129,"slots":{"0":{"StartSlotID":8192,"EndSlotID":16383}}}}}
		//node.ShardID, node.NodeID, node.IP, node.Port
		go cluster.NewCluster(128, 1, 1, "127.0.0.1", 9001, "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003", "127.0.0.1:9001")
		go cluster.NewCluster(128, 1, 2, "127.0.0.1", 9002, "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003", "127.0.0.1:9001")
		go cluster.NewCluster(128, 1, 3, "127.0.0.1", 9003, "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003", "127.0.0.1:9001")
		//go cluster.NewCluster(128, 2, "127.0.0.1", 9002, 1, ":9002", cluster.Master, "127.0.0.1:9001")
		//go cluster.NewCluster(128, 3, "127.0.0.1", 9003, 1, ":9003", cluster.Master, "127.0.0.1:9001")

		//LeaderID赋值和修改
		//go func() {
		//	tt := time.NewTicker(10 * time.Second)
		//	for range tt.C {
		//		fmt.Printf("11=================leaderId=%v", 1)
		//		cl.LocalNode.LeaderID = 1
		//		cl.LeaderID = 1
		//		cl1.LocalNode.LeaderID = 1
		//		cl2.LocalNode.LeaderID = 1
		//	}
		//}()
		// 保持主线程运行
		log.Println("=== 主线程开始运行 ===")
		for i := 0; i < 100; i++ {
			log.Printf("主线程运行中，第 %d 秒", i)
			time.Sleep(1 * time.Second)
		}
		log.Println("=== 主线程运行结束 ===")

		/*
	*/

	//TODO:测试数据写入

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
		cs3 := nh.GetNoOPSession(uint64(dragonboatRaft.GlobalShard))
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
					result, err := nh.SyncRead(ctx, uint64(dragonboatRaft.GlobalShard), []byte(key))
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
	raftStopper.Wait()

}
