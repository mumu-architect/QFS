package cluster

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// 常量定义
const (
	TotalSlots     = 16384           // 哈希槽总数（与Redis一致）
	GossipInterval = 2 * time.Second // Gossip协议心跳间隔
	FailTimeout    = 6 * time.Second // 节点故障判定超时
	ReplicaCount   = 1               // 每个主节点默认从节点数
	SyncBatchSize  = 100             // 主从同步批次大小
)

// NodeType 节点类型（主/从）
type NodeType string

const (
	Master NodeType = "master"
	Slave  NodeType = "slave"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	Online  NodeStatus = "online"
	Offline NodeStatus = "offline"
)

// Node 集群节点信息
type Node struct {
	ShardID  int        `json:"shardID"` // 节点唯一ID
	NodeID   int        `json:"id"`      // 节点唯一ID
	IP       string     `json:"ip"`      // 地址（IP）
	Port     int        `json:"port"`    // 地址（Port）
	LeaderID int        `json:"leaderID"`
	Addr     string     `json:"addr"`   // 地址（IP:Port）
	Type     NodeType   `json:"type"`   // 主/从
	Status   NodeStatus `json:"status"` // 在线/离线
	Slots    []int      `json:"slots"`  // 负责的哈希槽（主节点有效）
	//MasterID int        `json:"master_id"` // 从节点对应的主节点ID
}

// ClusterState 集群状态结构体（用于节点间同步）
type ClusterState struct {
	Nodes    map[int]*Node   `json:"nodes"`
	SlotMap  map[int]string  `json:"slotMap"`
	Replicas map[int][]*Node `json:"replicas"`
}

// Cluster 集群核心结构体
type Cluster struct {
	mu        sync.RWMutex
	LocalNode *Node // 本地节点
	//Nodes          map[string]*Node    // 集群所有节点（ID->Node）
	Nodes          map[int]*Node       // 集群所有节点（ID->Node）
	RaftNodes      map[string]*Node    //Raft所有节点，（ID->Node）
	slotMap        map[int]string      // 哈希槽->主节点ID映射
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
	serveMux       *http.ServeMux       // 每个节点自己的HTTP路由
	lastActive     map[int]time.Time    // 节点最后活跃时间（用于故障检测）
	httpServer     *http.Server         // HTTP服务器实例
	raftPort       int                  // Raft专用端口
}

var (
	globalNodes = make(map[int]*Node) // 存储所有节点
)

// NewCluster 创建集群节点
func NewCluster(shardID int, nodeID int, ip string, port int, addr string, nodeType NodeType, masterAddr string) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	dataFile := getDataFilePath(addr)
	// 1. 创建当前集群的本地节点
	localNode := &Node{
		ShardID:  shardID,
		NodeID:   nodeID,
		IP:       ip,
		Port:     port,
		Addr:     addr,
		LeaderID: 1,
		Type:     nodeType,
		Status:   Online,
		Slots:    []int{},
		//MasterID: 1,
	}
	cluster := &Cluster{
		LocalNode: localNode,
		//Nodes:     make(map[string]*Node),
		Nodes:     make(map[int]*Node),
		RaftNodes: make(map[string]*Node),
		slotMap:   make(map[int]string),
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
		serveMux:       http.NewServeMux(),
		lastActive:     make(map[int]time.Time),
	}

	// 初始化数据结构
	cluster.initRedisData()
	globalNodes[nodeID] = localNode
	// 主节点初始化（加载数据+槽分配+落盘任务）
	if nodeType == Master {
		if err := cluster.loadPersistedData(); err != nil {
			log.Printf("主节点 %s 加载持久化数据失败：%v", addr, err)
		} else {
			log.Printf("主节点 %s 加载持久化数据成功", addr)
		}
		cluster.initSlots()
		cluster.Nodes[nodeID] = localNode
		go cluster.persistDataLoop()
	} else {
		// 从节点初始化
		log.Printf("=== 从节点 %s 开始初始化，主节点地址：%s ===", addr, masterAddr)
		//cluster.LocalNode.MasterID = masterAddr // 暂时使用地址作为 MasterID
		//cluster.LocalNode.LeaderID = masterID // 暂时使用地址作为 MasterID
		// 从节点也需要将自己添加到节点列表中
		cluster.Nodes[nodeID] = localNode
		log.Printf("从节点 %s 添加自己到节点列表中，节点 ID：%s", addr, nodeID)
		// 根据地址创建主节点的信息，确保节点列表中始终有主节点的信息
		//masterNodeID := "master-" + masterAddr

		masterNode := &Node{
			ShardID:  shardID,
			NodeID:   nodeID,
			IP:       ip,
			Port:     port,
			Addr:     addr,
			LeaderID: 1,
			Type:     nodeType,
			Status:   Online,
			Slots:    []int{},
		}
		cluster.Nodes[nodeID] = masterNode

		// 将同步操作放到 goroutine 中，避免阻塞初始化过程
		go cluster.syncFromMaster(masterAddr)

	}
	cluster.Nodes = globalNodes
	//TODO: 启动选举循环

	cluster.registerAllHandlers()
	go cluster.startHTTPAPI()

	log.Printf("节点 %s 启动成功（类型：%s）", addr, nodeType)
	return cluster
}

// persistDataLoop 数据持久化后台循环
func (c *Cluster) persistDataLoop() {
	ticker := time.NewTicker(10 * time.Second) // 每10秒持久化一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			//if c.LocalNode.Type == Master {
			if c.LocalNode.LeaderID == c.LocalNode.NodeID {
				if err := c.persistData(); err != nil {
					log.Printf("主节点 %s 数据持久化失败：%v", c.LocalNode.Addr, err)
				}
			} else {
				if err := c.persistSlaveData(); err != nil {
					log.Printf("从节点 %s 数据持久化失败：%v", c.LocalNode.Addr, err)
				}
			}
		case <-c.ctx.Done():
			return
		}
	}
}
