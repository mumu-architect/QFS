package cluster

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type Gossip struct {
	state          *ClusterState
	gossipInterval time.Duration
	nodeTimeout    time.Duration
	transport      *GossipTransport
}

type GossipTransport struct {
	// 可以包含连接池等，但为简化，这里不实现
}

func NewGossip(state *ClusterState, interval, timeout time.Duration) *Gossip {
	return &Gossip{
		state:          state,
		gossipInterval: interval,
		nodeTimeout:    timeout,
		transport:      &GossipTransport{},
	}
}

// Start 启动Gossip协议的后台循环
func (g *Gossip) Start() {
	go g.gossipLoop()
	go g.failureDetectionLoop()
}

// gossipLoop 定期向随机节点发送Gossip消息
func (g *Gossip) gossipLoop() {
	ticker := time.NewTicker(g.gossipInterval)
	defer ticker.Stop()
	for range ticker.C {
		// 随机选择一个节点发送Gossip
		targetNode := g.state.PickRandomNode()
		if targetNode == nil {
			continue // 集群中只有自己
		}
		g.sendPing(targetNode.Addr)
	}
}

// failureDetectionLoop 定期检查节点是否超时
func (g *Gossip) failureDetectionLoop() {
	ticker := time.NewTicker(g.nodeTimeout / 2) // 每半次超时检查一次
	defer ticker.Stop()
	for range ticker.C {
		nodes := g.state.GetAllNodes()
		now := time.Now()
		for _, node := range nodes {
			if node.ID == g.state.Self.ID {
				continue
			}
			// 如果节点长时间未响应，标记为PFAIL
			if now.Sub(node.LastSeen) > g.nodeTimeout {
				g.state.MarkAsPFail(node.ID)
			}
		}
	}
}

// sendPing 向目标地址发送一个简单的PING命令作为心跳
func (g *Gossip) sendPing(targetAddr string) {
	conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)
	if err != nil {
		fmt.Printf("[GOSSIP] Failed to dial %s: %v\n", targetAddr, err)
		// 连接失败，尝试标记为PFAIL
		if node, ok := g.state.GetNodeByAddr(targetAddr); ok {
			g.state.MarkAsPFail(node.ID)
		}
		return
	}
	defer conn.Close()

	// 发送一个简单的PING命令
	pingCmd := "*1\r\n$4\r\nPING\r\n"
	_, err = conn.Write([]byte(pingCmd))
	if err != nil {
		fmt.Printf("[GOSSIP] Failed to send ping to %s: %v\n", targetAddr, err)
		return
	}

	// 设置读超时
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// 读取响应
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("[GOSSIP] Failed to read pong from %s: %v\n", targetAddr, err)
		return
	}

	// 简单处理响应
	if strings.HasPrefix(strings.TrimSpace(response), "+PONG") {
		// 更新目标节点的最后活跃时间
		if node, ok := g.state.GetNodeByAddr(targetAddr); ok {
			g.state.UpdateNodeLastSeen(node.ID)
		}
		fmt.Printf("[GOSSIP] Ping to %s successful.\n", targetAddr)
	}
}

// HandleGossipMessage 处理收到的Gossip相关消息
func (g *Gossip) HandleGossipMessage(msg []byte) {
	// 在真实实现中，这里会解析复杂的Gossip消息
	// 例如，处理 CLUSTER MEET, PING, FAIL 等包
	// 并相应地更新 ClusterState
	fmt.Printf("[GOSSIP] Received raw message: %s\n", string(msg))
	// 简化处理：如果收到任何来自其他节点的有效通信，就更新它的 LastSeen
	// 这需要消息中包含发送者的ID和地址，这里做了简化假设
}
