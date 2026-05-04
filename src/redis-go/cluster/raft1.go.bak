package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
)

// HashiCorpRaftNode 基于 HashiCorp Raft 库的节点实现
type HashiCorpRaftNode struct {
	raft     *raft.Raft
	raftDir  string
	nodeID   string
	nodeAddr string
}

// NewHashiCorpRaftNode 创建一个新的 HashiCorp Raft 节点
func NewHashiCorpRaftNode(nodeID string, addr string, peers map[string]string) *HashiCorpRaftNode {
	node := &HashiCorpRaftNode{
		nodeID:   fmt.Sprintf("node%s", nodeID),
		nodeAddr: addr,
		raftDir:  filepath.Join("/tmp/raft", fmt.Sprintf("node%s", nodeID)),
	}

	// 确保 raft 目录存在
	if err := os.MkdirAll(node.raftDir, 0755); err != nil {
		log.Fatalf("创建 raft 目录失败: %v", err)
	}

	// 配置 raft
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(node.nodeID)

	// 创建存储
	logs := raft.NewInmemStore()
	stable := raft.NewInmemStore()
	raftsnapshots, err := raft.NewFileSnapshotStore(node.raftDir, 2, os.Stderr)
	if err != nil {
		log.Fatalf("创建快照存储失败: %v", err)
	}

	//TODO: 创建网络层
	transport, err := raft.NewTCPTransport(addr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("创建 TCP 传输失败: %v", err)
	}

	// 初始服务器配置
	servers := []raft.Server{
		{
			ID:      raft.ServerID(node.nodeID),
			Address: transport.LocalAddr(),
		},
	}

	// 添加其他节点
	for peerID, peerAddr := range peers {
		servers = append(servers, raft.Server{
			ID:      raft.ServerID(fmt.Sprintf("node%s", peerID)),
			Address: raft.ServerAddress(peerAddr),
		})
	}

	// 创建 raft 实例
	node.raft, err = raft.NewRaft(config, nil, logs, stable, raftsnapshots, transport)
	if err != nil {
		log.Fatalf("创建 raft 实例失败: %v", err)
	}

	// 启动集群
	if len(servers) > 1 {
		configuration := raft.Configuration{
			Servers: servers,
		}
		node.raft.BootstrapCluster(configuration)
	}

	return node
}

// Start 启动节点
func (n *HashiCorpRaftNode) Start() {
	log.Printf("节点 %s 启动成功 | 地址：%s\n", n.nodeID, n.nodeAddr)

	// 监听状态变化
	go func() {
		for range n.raft.LeaderCh() {
			if n.raft.State() == raft.Leader {
				log.Printf("===== 节点 %s 成为主节点 =====\n", n.nodeID)
			}
		}
	}()
}

// Stop 停止节点
func (n *HashiCorpRaftNode) Stop() {
	if n.raft != nil {
		n.raft.Shutdown()
		log.Printf("节点 %s 停止成功\n", n.nodeID)
	}
}

// GetState 获取节点状态
func (n *HashiCorpRaftNode) GetState() string {
	switch n.raft.State() {
	case raft.Leader:
		return "Leader"
	case raft.Follower:
		return "Follower"
	case raft.Candidate:
		return "Candidate"
	default:
		return "Unknown"
	}
}

// IsLeader 检查是否为主节点
func (n *HashiCorpRaftNode) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// TransferLeadership 转移领导权到指定节点
func (n *HashiCorpRaftNode) TransferLeadership(targetNodeID string, targetNodeAddr string) error {
	if n.raft == nil {
		return fmt.Errorf("节点%d Raft实例未初始化", n.nodeID)
	}
	// 发起领导权转移
	f := n.raft.LeadershipTransferToServer(
		raft.ServerID(fmt.Sprintf("%s", targetNodeID)),
		raft.ServerAddress(targetNodeAddr),
	)
	return f.Error()
}

// 集成到 Cluster 结构中
func (c *Cluster) initRaftNode(nodeID string, addr string, peers map[string]string) *HashiCorpRaftNode {
	return NewHashiCorpRaftNode(nodeID, addr, peers)
}

// 替换原来的选举循环，使用 HashiCorp Raft 的内置选举机制
func (c *Cluster) electionLoop() {
	//  HashiCorp Raft 会自动处理选举，这里可以添加监控逻辑
	log.Printf("节点 %s 使用 HashiCorp Raft 进行主从管理", c.LocalNode.Addr)

	// 2. 配置对等节点列表（排除自身）
	peers := make(map[string]string)
	for id, node := range c.Nodes {
		if id != c.LocalNode.ID {
			// 使用节点的完整地址作为Raft地址
			// 为对等节点计算Raft地址（基于它们的raftPort）
			// 这里需要获取对等节点的raftPort，暂时使用简单的方式
			peers[id] = node.RaftAddr
		}
	}
	// 3. 创建并启动当前节点的Raft实例
	log.Printf("=== 开始启动Raft节点 %s ===", c.LocalNode.ID)
	raftNode := NewHashiCorpRaftNode(c.LocalNode.ID, c.LocalNode.RaftAddr, peers)
	//TODO: raftNode := NewHashiCorpRaftNode(c.LocalNode.ID, strings.Split(c.LocalNode.Addr, ":")[0]+":"+"1"+strings.Split(c.LocalNode.Addr, ":")[1], peers)
	if raftNode == nil {
		log.Printf("节点 %s 创建失败，地址：%s", c.LocalNode.ID, c.LocalNode.RaftAddr)
		return
	}
	raftNode.Start()
	log.Printf("节点 %s 已启动，地址：%s", c.LocalNode.ID, c.LocalNode.RaftAddr)

	// 4. 持续监控Raft状态
	for {
		time.Sleep(5 * time.Second)
		if raftNode.IsLeader() {
			log.Printf("节点 %s port[%s] 当前状态：Leader", c.LocalNode.ID, c.LocalNode.Addr)
		} else {
			log.Printf("节点 %s port[%s] 当前状态：%s", c.LocalNode.ID, c.LocalNode.Addr, raftNode.GetState())
		}
	}
}

// 检查主节点是否在线（保留原有功能，用于兼容性）
func (c *Cluster) checkMasterOnline(masterNode *Node) bool {
	// 尝试向主节点发送请求，检查是否在线
	url := fmt.Sprintf("http://%s/ping", masterNode.Addr)
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// 注册Raft相关HTTP接口（保留原有功能，用于兼容性）
func (c *Cluster) registerRaftHandlers() {
	// 处理投票请求（HashiCorp Raft 会通过 TCP 协议处理投票，这里保留接口用于兼容性）
	c.serveMux.HandleFunc("/requestVote", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"term":        1,
			"voteGranted": false,
		})
	})
}

// randomElectionTimeout 生成随机选举超时（150-300ms，避免同时选举）
func randomElectionTimeout() time.Duration {
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
}
