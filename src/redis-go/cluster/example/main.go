package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mumu.com/redis-go/cluster"
)

type Node struct {
	ShardID    int
	NodeID     int
	IP         string
	Port       int
	MasterId   string
	MasterAddr string
	ShardIDS   string
	//Type       string
	Peers    string // shard下所有集群节点 a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
	NodeInfo string //所有集群节点a1=127.0.0.1:9001,a2=127.0.0.1:9002,a3=127.0.0.1:9003
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
	// go run main.go --shardID 128 --id 1 --ip 127.0.0.1 --port 19001 --shardIDS "128,129" --peers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	// go run main.go --shardID 128 --id 2 --ip 127.0.0.1 --port 19002 --shardIDS "128,129" --peers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	// go run main.go --shardID 128 --id 3 --ip 127.0.0.1 --port 19003 --shardIDS "128,129" --peers "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	// go run main.go --shardID 129 --id 4 --ip 127.0.0.1 --port 19004 --shardIDS "128,129" --peers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	// go run main.go --shardID 129 --id 5 --ip 127.0.0.1 --port 19005 --shardIDS "128,129" --peers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	// go run main.go --shardID 129 --id 6 --ip 127.0.0.1 --port 19006 --shardIDS "128,129" --peers "4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006" --nodeInfo "1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003,4=127.0.0.1:19004,5=127.0.0.1:19005,6=127.0.0.1:19006"
	//

	/*
		var node Node
		flag.IntVar(&node.ShardID, "shardID", 0, "端口 1/2/3")
		flag.IntVar(&node.NodeID, "id", 0, "1/2/3")
		flag.StringVar(&node.IP, "ip", "127.0.0.1", "IP")
		flag.IntVar(&node.Port, "port", 0, "端口 9001/9002/9003")
		flag.StringVar(&node.ShardIDS, "shardIDS", "128,129", "ShardIDS")
		flag.StringVar(&node.Peers, "peers", "", "集群所有节点 1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003")
		flag.StringVar(&node.NodeInfo, "nodeInfo", "", "集群所有节点 1=127.0.0.1:19001,2=127.0.0.1:19002,3=127.0.0.1:19003")

		flag.Parse()
		if node.ShardID == 0 || node.NodeID == 0 || node.IP == "" || node.Port == 0 || node.ShardIDS == "" || node.Peers == "" || node.NodeInfo == "" {
			log.Fatal("必须输入--ShardID --id --ip --port --masterAddr --type --raftAddr --peers")
		}

		nh := dragonboatRaft.NewDragonBoatRaftNode(node.ShardID, node.NodeID, node.Peers, node.NodeInfo)
		nodeMeta := dragonboatRaft.InitMeta(node.ShardID, node.NodeID, node.IP, node.Port, node.ShardIDS, node.Peers, node.NodeInfo)
		//TODO: 初始化槽信息，首次平均分配槽
		//TODO: qfs_meta:9999:NodeSlotMetas
		go dragonboatRaft.StartSlotWrite(nh, nodeMeta)
		//TODO:存储每个shardID: qfs_meta:9999:128 qfs_meta:9999:129
		go nodeMeta.StartMetaWrite(nh, nodeMeta)
		//TODO:leader变更，修改槽对应新的leaderID
		//TODO:存储leaderID: get qfs_meta:9999:129:ShardNodeLeaderID get qfs_meta:9999:128:ShardNodeLeaderID [已测]
		go dragonboatRaft.Start(nh, nodeMeta)

	*/

	//TODO:集合真实数据存储cluster
	// 初始化数据目录（在storage.go中实现）
	cluster.InitDataDir()
	//node.ShardID, node.NodeID, node.IP, node.Port
	go cluster.NewCluster(128, 1, "127.0.0.1", 9001, ":9001", "127.0.0.1:9001")
	go cluster.NewCluster(128, 2, "127.0.0.1", 9002, ":9002", "127.0.0.1:9001")
	go cluster.NewCluster(128, 3, "127.0.0.1", 9003, ":9003", "127.0.0.1:9001")
	//go cluster.NewCluster(128, 2, "127.0.0.1", 9002, 1, ":9002", cluster.Master, "127.0.0.1:9001")
	//go cluster.NewCluster(128, 3, "127.0.0.1", 9003, 1, ":9003", cluster.Master, "127.0.0.1:9001")

	//LeaderID赋值和修改
	//go func() {
	//	tt := time.NewTicker(10 * time.Second)
	//	for range tt.C {
	//		fmt.Printf("11=================leaderId=%v", 1)
	//		cl.LocalNode.LeaderID = 1
	//		cl.LocalNode.MasterID = 1
	//		cl1.LocalNode.LeaderID = 1
	//		cl1.LocalNode.MasterID = 1
	//		cl2.LocalNode.LeaderID = 1
	//		cl2.LocalNode.MasterID = 1
	//	}
	//}()
	// 保持主线程运行
	log.Println("=== 主线程开始运行 ===")
	for i := 0; i < 100; i++ {
		log.Printf("主线程运行中，第 %d 秒", i)
		time.Sleep(1 * time.Second)
	}
	log.Println("=== 主线程运行结束 ===")
	//// 启动1主2从集群，用于测试从节点选举
	//log.Println("=== 开始启动集群节点 ===")
	//log.Println("启动主节点 :9001")
	//go cluster.NewCluster(":9001", cluster.Master, "", "127.0.0.1:19001") // 主1
	//log.Println("启动从节点 :9002")
	//go cluster.NewCluster(":9002", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19002") // 从1（主1）
	//log.Println("启动从节点 :9003")
	//go cluster.NewCluster(":9003", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19003") // 从2（主1）
	//log.Println("=== 集群节点启动完成 ===")

	/*
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

	*/
}

/*
func main() {
	// 初始化数据目录（在storage.go中实现）
	cluster.InitDataDir()
	// 启动1主2从集群，用于测试从节点选举
	log.Println("=== 开始启动集群节点 ===")
	log.Println("启动主节点 :9001")
	go cluster.NewCluster(":9001", cluster.Master, "", "127.0.0.1:19001") // 主1
	log.Println("启动从节点 :9002")
	go cluster.NewCluster(":9002", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19002") // 从1（主1）
	log.Println("启动从节点 :9003")
	go cluster.NewCluster(":9003", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19003") // 从2（主1）
	log.Println("=== 集群节点启动完成 ===")

	// 保持主线程运行
	log.Println("=== 主线程开始运行 ===")
	for i := 0; i < 100; i++ {
		log.Printf("主线程运行中，第 %d 秒", i)
		time.Sleep(1 * time.Second)
	}
	log.Println("=== 主线程运行结束 ===")
}
*/
