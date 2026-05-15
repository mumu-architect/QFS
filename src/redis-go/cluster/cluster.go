package cluster

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lni/dragonboat/v4"
	"mumu.com/redis-go/cluster/dragonboatRaft"
	"mumu.com/redis-go/cluster/fileManager"
	"mumu.com/redis-go/cluster/logManager"
	"mumu.com/redis-go/distributedId/snowflake"
)

// 常量定义
const (
	TotalSlots    = 16384 // 哈希槽总数（与Redis一致）
	SyncBatchSize = 100   // 主从同步批次大小
)

// NodeStatus 节点状态
type NodeStatus string

const (
	Online  NodeStatus = "online"
	Offline NodeStatus = "offline"
)

// 分批读取阈值 控制IO压力
const batchSize = 3 //线上500
// Node 集群节点信息
type Node struct {
	ShardID  int        `json:"shardID"`
	NodeID   int        `json:"id"`   // 节点唯一ID
	IP       string     `json:"ip"`   // 地址（IP）
	Port     int        `json:"port"` // 地址（Port）
	RpcPort  int        `json:"rpcPort"`
	LeaderID int        `json:"leaderID"`
	Addr     string     `json:"addr"`   // 地址（IP:Port）//TODO:使用的地方比较多
	Status   NodeStatus `json:"status"` // 在线/离线
	//Slots    []int      `json:"slots"`  // 负责的哈希槽（主节点有效）
}

// Cluster 集群核心结构体
type Cluster struct {
	mu                 sync.RWMutex
	ShardID            int                           `json:"shardID"`
	LeaderID           int                           `json:"leaderID"`
	NodeAllSlotMetas   *dragonboatRaft.NodeSlotMetas `json:"nodeAllSlotMetas"`
	LocalNode          *Node                         `json:"localNode"`
	DragonBoatNodeHost *dragonboat.NodeHost          `json:"dragonBoatNodeHost"`
	NodeMeta           *dragonboatRaft.NodeMeta      `json:"nodeMeta"`
	NodeLog            *logManager.NodeLog           `json:"nodeLog"`
	NodeFile           *fileManager.NodeFile         `json:"nodeFile"`
	ShardNodeInfo      map[int]*Node                 `json:"shardNodeInfo"`
	ShardRpcNodeInfo   map[int]*Node                 `json:"shardRpcNodeInfo"`
	AllNodeInfos       string                        `json:"allNodeInfos"`
	RpcNodeInfo        string                        `json:"rpcNodeInfo"`
	SnowflakeGenerate  *snowflake.SnowFlakeGenerate  `json:"snowflakeGenerate"`
	//Nodes          map[string]*Node    // 集群所有节点（ID->Node）
	//Nodes          map[int]*Node       // 集群所有节点（ID->Node）
	//slotMap        map[int]string      // 哈希槽->主节点ID映射
	dataStore      interface{}         // 本地存储（主节点读写，从节点同步）
	replicas       map[int][]*Node     // 主节点ID->从节点列表映射
	gossipConn     map[string]net.Conn // Gossip协议连接（Addr->Conn）
	ctx            context.Context
	cancel         context.CancelFunc
	currentTerm    int                  // Raft当前任期
	votedFor       string               // 本任期已投票的节点ID
	electionTicker *time.Ticker         // Raft选举定时器
	reshardingLock sync.Mutex           // 重分片锁（避免并发冲突）
	migratingSlots map[int]int          // 迁移中槽位：槽位->目标主节点ID
	importingSlots map[int]string       // 导入中槽位：槽位->原主节点ID
	recentChanges  map[string]time.Time // 最近数据变更（用于增量同步）
	dataFile       string               // 数据落盘文件路径
	lastSyncOffset int64                // 从节点同步偏移量（断点续传）
	ServeMux       *http.ServeMux       // 每个节点自己的HTTP路由
	lastActive     map[int]time.Time    // 节点最后活跃时间（用于故障检测）
	HttpServer     *http.Server         // HTTP服务器实例
}

// NewCluster 创建集群节点
func NewCluster(shardID int, leaderID int, nodeID int, ip string, port int, rpcPort int, rpcPeers string, rpcNodeInfo string, peers string, nodeInfo string, nodeSlotMetas *dragonboatRaft.NodeSlotMetas, nh *dragonboat.NodeHost, nodeMeta *dragonboatRaft.NodeMeta, nodeLog *logManager.NodeLog, nodeFile *fileManager.NodeFile) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	addr := fmt.Sprintf(":%d", port)
	dataFile := getDataFilePath(nodeID, addr)
	shardNodeInfo := stringToMapNodes(shardID, peers)
	shardRpcNodeInfo := stringToMapNodes(shardID, rpcPeers)
	snowflakeGenerate := snowflake.NewSnowFlakeGenerate()
	// 1. 创建当前集群的本地节点
	localNode := &Node{
		ShardID:  shardID,
		NodeID:   nodeID,
		IP:       ip,
		Port:     port,
		RpcPort:  rpcPort,
		Addr:     addr,
		LeaderID: leaderID,
		Status:   Online,
		//Slots:    []int{},
	}
	cl := &Cluster{
		ShardID:            shardID,
		LeaderID:           leaderID,
		NodeAllSlotMetas:   nodeSlotMetas,
		LocalNode:          localNode,
		DragonBoatNodeHost: nh,
		NodeMeta:           nodeMeta,
		NodeLog:            nodeLog,
		NodeFile:           nodeFile,
		//Nodes:     make(map[string]*Node),
		//Nodes:         shardNodeInfo,
		ShardNodeInfo:     shardNodeInfo,
		ShardRpcNodeInfo:  shardRpcNodeInfo,
		AllNodeInfos:      nodeInfo,
		RpcNodeInfo:       rpcNodeInfo,
		SnowflakeGenerate: snowflakeGenerate,
		//slotMap:       make(map[int]string),
		// 初始化RedisData（替换原dataStore := make(map[string]string)）
		dataStore:      nil,
		replicas:       make(map[int][]*Node),
		gossipConn:     make(map[string]net.Conn),
		ctx:            ctx,
		cancel:         cancel,
		currentTerm:    0,
		votedFor:       "",
		migratingSlots: make(map[int]int),
		importingSlots: make(map[int]string),
		recentChanges:  make(map[string]time.Time),
		dataFile:       dataFile,
		lastSyncOffset: 0,
		ServeMux:       http.NewServeMux(),
		lastActive:     make(map[int]time.Time),
	}

	//TODO: 初始化数据结构
	cl.initRedisData()

	//// 主节点初始化（加载数据+槽分配+落盘任务）
	//if nodeID == leaderID {
	//	//err := cl.loadPersistedData();
	//	//if  err != nil {
	//	//	log.Printf("主节点 %s 加载持久化数据失败：%v", addr, err)
	//	//} else {
	//	//	log.Printf("主节点 %s 加载持久化数据成功", addr)
	//	//}
	//} else {
	//	// 从节点初始化
	//	log.Printf("=== 从节点 %s 开始初始化，主节点地址：%s ===", addr, masterAddr)
	//	// 将同步操作放到 goroutine 中，避免阻塞初始化过程
	//	//go cl.syncFromMaster(masterAddr)
	//}
	//TODO:数据持久化，主从都可以
	//go cl.persistDataLoop()
	fmt.Printf("11=======%v\n", cl.ShardNodeInfo)

	//TODO:重启分批加载本地log数据到内存
	err := cl.RestartBatchLoadLog()
	if err != nil {
		fmt.Printf("RestartBatchLoadLog:%v\n", err)
	}

	//TODO: 注册所有助手方法
	cl.registerAllHandlers()

	//TODO:启动rpc服务，用于保从节点的雪花key,存到key槽对应的主节点
	go cl.StartRPCServer()
	//TODO: 启动http服务器
	go cl.startHTTPAPI()

	//TODO:增量文件同步,先拉取一次增量日志文件的边界，根据边界同步增量的图片文件
	go func() {
		initOnce := false
		tt := time.NewTicker(500 * time.Millisecond)
		for range tt.C {
			if !initOnce {
				//shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, shardID)
				//if err != nil {
				//	fmt.Printf("111====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
				//}
				//cl.DynamicallyModifyClusterNodeLeaderID(shardNodeLeaderID)
				incrementLogMap, err := cl.PullLeaderIncrementLogOnce()
				fmt.Printf("PullLeaderIncrementLogOnce:%v\n", incrementLogMap)
				if err == nil {
					initOnce = true
				} else {
					continue
				}
				go cl.PullLeaderIncrementLog(0, incrementLogMap.IncrementFileMaxIdx)
				//TODO:拉去主节点实时的的增量数据，并写入内存
				go cl.PullSyncLoop(incrementLogMap.IncrementFileMaxIdx)
			}
		}

	}()

	return cl
}

// persistDataLoop 数据持久化后台循环
func (cl *Cluster) persistDataLoop() {
	ticker := time.NewTicker(10 * time.Second) // 每10秒持久化一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cl.LocalNode.LeaderID == cl.LocalNode.NodeID {
				if err := cl.persistData(); err != nil {
					log.Printf("主节点 %s 数据持久化失败：%v", cl.LocalNode.Addr, err)
				}
			} else {
				if err := cl.persistSlaveData(); err != nil {
					log.Printf("从节点 %s 数据持久化失败：%v", cl.LocalNode.Addr, err)
				}
			}
		case <-cl.ctx.Done():
			return
		}
	}
}

// 字符串转map的node节点
func stringToMapNodes(shardID int, peers string) map[int]*Node {
	var shardNodeInfo = make(map[int]*Node)
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
		addr := fmt.Sprintf(":%d", peerNodePort)
		peerNodeInfo := &Node{
			ShardID: shardID,
			NodeID:  peerNodeID,
			IP:      peerNodeIP,
			Port:    peerNodePort,
			Addr:    addr,
		}
		//fmt.Printf("111====%d======%v \n", peerNodeID, peerNodeInfo)
		shardNodeInfo[peerNodeID] = peerNodeInfo
	}
	return shardNodeInfo
}

// GetSlotToLeaderID 根据槽获取leaderID
func (cl *Cluster) GetSlotToLeaderID(keySolt int) int {
	leaderId := dragonboatRaft.GetLeaderID(cl.DragonBoatNodeHost, cl.NodeMeta, keySolt)
	if leaderId > 0 {
		return leaderId
	} else {
		fmt.Printf("leaderID 不存在返回-1")
		return -1
	}
}

// 通过节点nodeID获取节点地址
func (cl *Cluster) GetNodeIdToNodeArr(nodeId int) string {
	peerList := strings.Split(cl.AllNodeInfos, ",")
	for _, peer := range peerList {
		parts := strings.Split(peer, "=")
		if len(parts) != 2 {
			continue
		}
		peerNodeID, _ := strconv.Atoi(parts[0])
		nodeAddr := parts[1]
		if nodeId == peerNodeID {
			return nodeAddr
		}
	}
	return ""
}

// DynamicallyModifyClusterNodeLeaderID 动态修改leaderID
func (cl *Cluster) DynamicallyModifyClusterNodeLeaderID(shardNodeLeaderID *dragonboatRaft.NodeShardLeaderMeta) {

	if cl.ShardID == shardNodeLeaderID.ShardID {
		cl.LeaderID = shardNodeLeaderID.LeaderID
		cl.LocalNode.LeaderID = shardNodeLeaderID.LeaderID
	}
}
