package cluster

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
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
	ID       string     `json:"id"`        // 节点唯一ID
	Addr     string     `json:"addr"`      // 地址（IP:Port）
	RaftAddr string     `json:"raftAddr"`  // 地址（IP:Port）
	Type     NodeType   `json:"type"`      // 主/从
	Status   NodeStatus `json:"status"`    // 在线/离线
	Slots    []int      `json:"slots"`     // 负责的哈希槽（主节点有效）
	MasterID string     `json:"master_id"` // 从节点对应的主节点ID
}

// ClusterState 集群状态结构体（用于节点间同步）
type ClusterState struct {
	Nodes    map[string]*Node   `json:"nodes"`
	SlotMap  map[int]string     `json:"slotMap"`
	Replicas map[string][]*Node `json:"replicas"`
}

// Cluster 集群核心结构体
type Cluster struct {
	mu             sync.RWMutex
	LocalNode      *Node               // 本地节点
	Nodes          map[string]*Node    // 集群所有节点（ID->Node）
	RaftNodes      map[string]*Node    //Raft所有节点，（ID->Node）
	slotMap        map[int]string      // 哈希槽->主节点ID映射
	dataStore      interface{}         // 本地存储（主节点读写，从节点同步）
	replicas       map[string][]*Node  // 主节点ID->从节点列表映射
	gossipConn     map[string]net.Conn // Gossip协议连接（Addr->Conn）
	ctx            context.Context
	cancel         context.CancelFunc
	currentTerm    int                  // Raft当前任期
	votedFor       string               // 本任期已投票的节点ID
	electionTicker *time.Ticker         // Raft选举定时器
	reshardingLock sync.Mutex           // 重分片锁（避免并发冲突）
	migratingSlots map[int]string       // 迁移中槽位：槽位->目标主节点ID
	importingSlots map[int]string       // 导入中槽位：槽位->原主节点ID
	recentChanges  map[string]time.Time // 最近数据变更（用于增量同步）
	dataFile       string               // 数据落盘文件路径
	lastSyncOffset int64                // 从节点同步偏移量（断点续传）
	serveMux       *http.ServeMux       // 每个节点自己的HTTP路由
	lastActive     map[string]time.Time // 节点最后活跃时间（用于故障检测）
	httpServer     *http.Server         // HTTP服务器实例
	raftPort       int                  // Raft专用端口
}

var (
	globalNodes   = make(map[string]*Node) // 存储所有节点
	registryMutex sync.Mutex               // 保证并发安全
)

// NewCluster 创建集群节点
func NewCluster(addr string, nodeType NodeType, masterAddr string, raftAddr string) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	nodeID := generateNodeID()
	dataFile := getDataFilePath(addr)
	// 1. 创建当前集群的本地节点
	localNode := &Node{
		ID:       nodeID,
		Addr:     addr,
		RaftAddr: raftAddr,
		Type:     nodeType,
		Status:   Online,
		Slots:    []int{},
	}
	cluster := &Cluster{
		LocalNode: localNode,
		Nodes:     make(map[string]*Node),
		RaftNodes: make(map[string]*Node),
		slotMap:   make(map[int]string),
		// 初始化RedisData（替换原dataStore := make(map[string]string)）
		dataStore:      nil,
		replicas:       make(map[string][]*Node),
		gossipConn:     make(map[string]net.Conn),
		ctx:            ctx,
		cancel:         cancel,
		currentTerm:    0,
		votedFor:       "",
		migratingSlots: make(map[int]string),
		importingSlots: make(map[int]string),
		recentChanges:  make(map[string]time.Time),
		dataFile:       dataFile,
		lastSyncOffset: 0,
		serveMux:       http.NewServeMux(),
		lastActive:     make(map[string]time.Time),
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
		cluster.LocalNode.MasterID = masterAddr // 暂时使用地址作为 MasterID
		// 从节点也需要将自己添加到节点列表中
		cluster.Nodes[nodeID] = localNode
		log.Printf("从节点 %s 添加自己到节点列表中，节点 ID：%s", addr, nodeID)
		// 根据地址创建主节点的信息，确保节点列表中始终有主节点的信息
		masterNodeID := "master-" + masterAddr
		masterNode := &Node{
			ID:     masterNodeID,
			Addr:   masterAddr,
			Type:   Master,
			Status: Online,
			Slots:  []int{},
		}
		cluster.Nodes[masterNodeID] = masterNode
		// 更新本地节点的 MasterID 为新创建的主节点的 ID
		cluster.LocalNode.MasterID = masterNodeID
		log.Printf("从节点 %s 初始化 MasterID 为 %s，主节点地址：%s", cluster.LocalNode.Addr, masterNodeID, masterAddr)
		// 尝试从主节点获取集群状态，以便更新主节点的信息
		go func() {
			time.Sleep(2 * time.Second) // 等待主节点启动完成
			log.Printf("从节点 %s 开始从主节点 %s 获取集群状态", addr, masterAddr)
			state, err := FetchClusterState(masterAddr)
			if err != nil {
				log.Printf("从主节点 %s 获取集群状态失败：%v", masterAddr, err)
				return
			}
			// 合并集群状态到本地
			cluster.mu.Lock()
			//TODO:
			// jsonstring, _ := json.Marshal(state.Nodes)
			//log.Printf("从节点 %s 开始合并集群状态，获取到 %d 个节点的信息,%v", addr, len(state.Nodes), string(jsonstring))
			for id, node := range state.Nodes {
				if _, exists := cluster.Nodes[id]; !exists {
					cluster.Nodes[id] = node
					log.Printf("从集群状态添加新节点：%s（%s），ID：%s", node.Addr, node.Type, id)
				}
				// 如果节点是主节点，并且地址与 MasterID 匹配，更新 MasterID
				if node.Type == Master && node.Addr == masterAddr {
					oldMasterID := cluster.LocalNode.MasterID
					cluster.LocalNode.MasterID = id
					log.Printf("从节点 %s 更新 MasterID 为 %s（主节点 %s 的实际 ID），旧 MasterID：%s", cluster.LocalNode.Addr, id, masterAddr, oldMasterID)
					// 更新节点列表中的主节点信息
					delete(cluster.Nodes, masterNodeID)
					log.Printf("从节点 %s 删除旧的主节点信息，ID：%s", cluster.LocalNode.Addr, masterNodeID)
				}
			}
			cluster.mu.Unlock()
			log.Printf("从节点 %s 合并集群状态完成", addr)
		}()
		log.Printf("从节点 %s 开始同步从主节点 %s", addr, masterAddr)
		// 将同步操作放到 goroutine 中，避免阻塞初始化过程
		go cluster.syncFromMaster(masterAddr)

	}
	cluster.Nodes = globalNodes
	// 启动选举循环
	cluster.electionTicker = time.NewTicker(randomElectionTimeout())
	timeout := randomElectionTimeout()
	log.Printf("从节点 %s 启动选举循环，选举超时时间：%v", addr, timeout)
	go cluster.electionLoop()
	log.Printf("=== 从节点 %s 初始化完成 ===", addr)

	cluster.registerAllHandlers()
	go cluster.gossipLoop()
	go cluster.failureDetectionLoop()
	go cluster.startHTTPAPI()

	log.Printf("节点 %s 启动成功（类型：%s）", addr, nodeType)
	return cluster
}

// getNodeIDByAddr 通过地址获取节点ID（简化：假设地址唯一）
// 注意：该方法目前未被使用，保留以备后续扩展
func (c *Cluster) getNodeIDByAddr(addr string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for id, node := range c.Nodes {
		if node.Addr == addr {
			return id
		}
		// 检查地址的端口部分是否匹配
		if len(addr) > 0 && addr[0] == ':' {
			// 提取 addr 中的端口号
			addrPort := addr[1:]
			// 提取 node.Addr 中的端口号
			portStart := strings.LastIndex(node.Addr, ":")
			if portStart != -1 {
				nodePort := node.Addr[portStart+1:]
				if nodePort == addrPort {
					return id
				}
			}
		}
	}
	// 新节点未加入集群时，临时用地址作为ID占位
	return addr
}

// persistDataLoop 数据持久化后台循环
func (c *Cluster) persistDataLoop() {
	ticker := time.NewTicker(10 * time.Second) // 每10秒持久化一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.LocalNode.Type == Master {
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
