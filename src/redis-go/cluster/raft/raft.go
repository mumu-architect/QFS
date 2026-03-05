package raft

import (
	"fmt"
	"log"
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
func NewHashiCorpRaftNode(nodeID int, addr string, peers map[int]string) *HashiCorpRaftNode {
	node := &HashiCorpRaftNode{
		nodeID:   fmt.Sprintf("node%d", nodeID),
		nodeAddr: addr,
		raftDir:  filepath.Join("/tmp/raft", fmt.Sprintf("node%d", nodeID)),
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

	// 创建网络层
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
			ID:      raft.ServerID(fmt.Sprintf("node%d", peerID)),
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
