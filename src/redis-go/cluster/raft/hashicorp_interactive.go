package raft

import (
	"log"
	"testing"
	"time"
)

// TestRaftLeaderElection 测试Raft集群主节点故障后的重新选举功能
func TestRaftLeaderElection(t *testing.T) {
	// 1. 配置3个节点的地址信息（Raft推荐奇数节点）
	nodeMap := map[int]string{
		1: "localhost:9001",
		2: "localhost:9002",
		3: "localhost:9003",
	}

	// 存储节点实例的切片
	var nodes []*HashiCorpRaftNode
	// 记录当前Leader节点信息
	var currentLeader *HashiCorpRaftNode
	var leaderIndex int

	// 2. 启动所有节点并构建集群
	log.Println("=== 开始启动3个Raft节点 ===")
	for nodeID, addr := range nodeMap {
		// 构建当前节点的对等节点列表（排除自身）
		peers := make(map[int]string)
		for pid, paddr := range nodeMap {
			if pid != nodeID {
				peers[pid] = paddr
			}
		}

		// 创建并启动节点
		node := NewHashiCorpRaftNode(nodeID, addr, peers)
		if node == nil {
			t.Fatalf("节点%d创建失败，地址：%s", nodeID, addr)
		}
		node.Start()
		nodes = append(nodes, node)
		log.Printf("节点%d已启动，地址：%s", nodeID, addr)
	}

	// 3. 等待集群完成初始Leader选举（Raft选举需要一定时间）
	log.Println("=== 等待集群选举初始Leader ===")
	time.Sleep(6 * time.Second) // 给足够时间完成选举

	// 4. 查找并确认初始Leader
	for i, node := range nodes {
		if node.IsLeader() {
			currentLeader = node
			leaderIndex = i + 1 // 节点ID从1开始
			log.Printf("=== 初始Leader已选出：节点%d ===", leaderIndex)
			break
		}
	}

	// 打印所有节点状态
	log.Println("=== 初始选举后所有节点状态 ===")
	for i, node := range nodes {
		nodeID := i + 1
		log.Printf("节点%d | 角色：%s", nodeID, node.GetState())
	}

	// 检查是否成功选出Leader
	if currentLeader == nil {
		t.Fatal("集群启动后未选举出Leader，测试失败")
	}

	// 5. 模拟Leader节点故障（停止Leader）
	log.Printf("=== 停止当前Leader节点%d ===", leaderIndex)
	currentLeader.Stop()
	log.Printf("=== Leader节点%d已停止 ===", leaderIndex)

	// 6. 等待集群重新选举新Leader
	log.Println("=== 等待集群重新选举新Leader ===")
	time.Sleep(6 * time.Second)

	// 7. 验证是否选举出了新的Leader（排除已停止的原Leader）
	var newLeaderFound bool
	var newLeaderID int
	for i, node := range nodes {
		nodeID := i + 1
		// 跳过已停止的原Leader节点
		if nodeID == leaderIndex {
			continue
		}

		// 检查剩余节点是否有新Leader
		if node.IsLeader() {
			newLeaderFound = true
			newLeaderID = nodeID
			log.Printf("=== 新Leader已选出：节点%d ===", newLeaderID)
			break
		}
	}

	// 打印所有节点状态
	log.Println("=== 重新选举后所有节点状态 ===")
	for i, node := range nodes {
		nodeID := i + 1
		if nodeID == leaderIndex {
			log.Printf("节点%d | 角色：已停止", nodeID)
		} else {
			log.Printf("节点%d | 角色：%s", nodeID, node.GetState())
		}
	}

	// 8. 断言验证：必须选出新Leader
	if !newLeaderFound {
		t.Fatal("原Leader停止后，集群未选举出新高可用Leader，测试失败")
	}

	log.Println("=== 测试完成：Raft主从切换功能正常 ===")

	// 9. 清理资源：停止剩余节点
	for i, node := range nodes {
		nodeID := i + 1
		if nodeID != leaderIndex { // 只停止还在运行的节点
			node.Stop()
			log.Printf("=== 清理资源：停止节点%d ===", nodeID)
		}
	}
}
