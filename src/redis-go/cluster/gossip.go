package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// gossipLoop Gossip协议循环（节点状态同步）
func (c *Cluster) gossipLoop() {
	ticker := time.NewTicker(GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			// 随机选择3个节点发送Gossip消息（简化：随机抽样）
			nodes := make([]*Node, 0, len(c.Nodes))
			for _, node := range c.Nodes {
				if node.ID != c.LocalNode.ID && node.Status == Online {
					nodes = append(nodes, node)
				}
			}
			c.mu.RUnlock()

			// 发送Gossip消息
			var wg sync.WaitGroup
			sampleSize := 3
			if len(nodes) < sampleSize {
				sampleSize = len(nodes)
			}
			for i := 0; i < sampleSize; i++ {
				wg.Add(1)
				go func(targetNode *Node) {
					defer wg.Done()
					c.sendGossip(targetNode)
				}(nodes[i])
			}
			wg.Wait()
		}
	}
}

// sendGossip 发送Gossip消息到目标节点
func (c *Cluster) sendGossip(targetNode *Node) {
	// 构建集群状态消息
	c.mu.RLock()
	gossipMsg := ClusterState{
		Nodes:    c.Nodes,
		SlotMap:  c.slotMap,
		Replicas: c.replicas,
	}
	c.mu.RUnlock()

	// 序列化消息
	msgData, err := json.Marshal(gossipMsg)
	if err != nil {
		log.Printf("Gossip消息序列化失败：%v", err)
		return
	}

	// 发送HTTP请求
	//url := fmt.Sprintf("http://%s/receiveGossip", targetNode.Addr)
	url := fmt.Sprintf("http://%s/receiveGossip", targetNode.Addr)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(msgData)))
	if err != nil {
		log.Printf("发送Gossip到 %s 失败：%v", targetNode.Addr, err)
		// 标记节点为离线
		c.markNodeOffline(targetNode.ID)
		return
	}
	defer resp.Body.Close()

	log.Printf("发送Gossip到 %s 成功", targetNode.Addr)
}

// receiveGossip 接收Gossip消息并合并集群状态
func (c *Cluster) receiveGossip(msg ClusterState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 合并节点信息
	for nodeID, node := range msg.Nodes {
		// 本地无该节点则添加
		if _, exists := c.Nodes[nodeID]; !exists {
			c.Nodes[nodeID] = node
			// 如果是从节点，添加到主节点的副本列表
			if node.Type == Slave && node.MasterID != "" {
				c.replicas[node.MasterID] = append(c.replicas[node.MasterID], node)
			}
			log.Printf("从Gossip消息添加新节点：%s（%s）", node.Addr, node.Type)
		} else {
			// 更新节点状态（只在本地节点状态为离线且收到的状态为在线时更新）
			// 注意：如果本地节点已经标记为离线，不应该被Gossip消息重新标记为在线
			// 因为可能是其他节点的状态信息滞后
			localNode := c.Nodes[nodeID]
			if localNode.Status == Offline && node.Status == Online {
				// 检查节点是否真的在线，避免错误的状态更新
				url := fmt.Sprintf("http://localhost%s/ping", node.Addr)
				resp, err := http.Get(url)
				if err == nil && resp.StatusCode == http.StatusOK {
					localNode.Status = Online
					log.Printf("节点 %s 状态更新为在线", node.Addr)
					resp.Body.Close()
				}
			}
		}
		// 更新节点最后活跃时间
		c.lastActive[nodeID] = time.Now()
	}

	// 合并槽位映射（以主节点的映射为准）
	for slot, masterID := range msg.SlotMap {
		if _, exists := c.slotMap[slot]; !exists {
			c.slotMap[slot] = masterID
		}
	}

	// 合并副本列表
	for masterID, replicas := range msg.Replicas {
		for _, replica := range replicas {
			// 本地副本列表中无该节点则添加
			found := false
			for _, localReplica := range c.replicas[masterID] {
				if localReplica.ID == replica.ID {
					found = true
					break
				}
			}
			if !found {
				c.replicas[masterID] = append(c.replicas[masterID], replica)
			}
		}
	}

	// 更新从节点的 MasterID
	if c.LocalNode.Type == Slave {
		// 检查是否存在新的主节点
		var newMaster *Node
		for _, node := range c.Nodes {
			if node.Type == Master && node.Status == Online {
				newMaster = node
				break
			}
		}
		// 如果找到新的主节点，更新本地节点的 MasterID
		if newMaster != nil && c.LocalNode.MasterID != newMaster.ID {
			c.LocalNode.MasterID = newMaster.ID
			log.Printf("从节点 %s 更新 MasterID 为 %s", c.LocalNode.Addr, newMaster.ID)
		}
		// 遍历所有主节点，看看是否有主节点的地址与 MasterID 匹配
		if c.LocalNode.MasterID != "" {
			for _, node := range c.Nodes {
				if node.Type == Master && node.Addr == c.LocalNode.MasterID {
					if c.LocalNode.MasterID != node.ID {
						c.LocalNode.MasterID = node.ID
						log.Printf("从节点 %s 更新 MasterID 为 %s", c.LocalNode.Addr, node.ID)
					}
					break
				}
			}
		}
	}
}

// failureDetectionLoop 故障检测循环
func (c *Cluster) failureDetectionLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			for nodeID, node := range c.Nodes {
				if node.ID == c.LocalNode.ID || node.Status == Offline {
					continue
				}

				// 初始化最后活跃时间
				if _, exists := c.lastActive[nodeID]; !exists {
					// 对于从节点初始化时创建的主节点，设置一个较早的时间
					// 这样，在接下来的检查中，会直接检查是否超时
					if c.LocalNode.Type == Slave && node.Type == Master && node.ID == c.LocalNode.MasterID {
						log.Printf("从节点 %s 检测到主节点 %s，开始监控其状态", c.LocalNode.Addr, node.Addr)
						// 设置一个较早的时间，确保第一次检查就会超时
						c.lastActive[nodeID] = time.Now().Add(-FailTimeout - time.Second)
					} else {
						c.lastActive[nodeID] = time.Now()
						continue
					}
				}

				// 检查是否超时（超过FailTimeout未收到Gossip则标记为离线）
				if time.Since(c.lastActive[nodeID]) > FailTimeout {
					node.Status = Offline
					log.Printf("节点 %s 超时未活跃，标记为离线", node.Addr)
					// 如果离线节点是主节点，触发从节点选举
					if node.Type == Master {
						log.Printf("主节点 %s 离线，等待从节点故障转移", node.Addr)
					}
				}
			}
			c.mu.Unlock()
		}
	}
}

// markNodeOffline 标记节点为离线
func (c *Cluster) markNodeOffline(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.Nodes[nodeID]; exists && node.Status == Online {
		node.Status = Offline
		log.Printf("节点 %s 连接失败，标记为离线", node.Addr)
	}
}

// broadcastClusterState 广播集群状态（主动同步给所有节点）
func (c *Cluster) broadcastClusterState() {
	c.mu.RLock()
	nodes := make([]*Node, 0, len(c.Nodes))
	for _, node := range c.Nodes {
		if node.ID != c.LocalNode.ID && node.Status == Online {
			nodes = append(nodes, node)
		}
	}
	c.mu.RUnlock()

	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(targetNode *Node) {
			defer wg.Done()
			c.sendGossip(targetNode)
		}(node)
	}
	wg.Wait()
}

// registerGossipHandlers 注册Gossip相关HTTP接口
func (c *Cluster) registerGossipHandlers() {
	// 接收Gossip消息
	c.serveMux.HandleFunc("/receiveGossip", func(w http.ResponseWriter, r *http.Request) {
		var gossipMsg ClusterState
		if err := json.NewDecoder(r.Body).Decode(&gossipMsg); err != nil {
			http.Error(w, fmt.Sprintf("消息解析失败：%v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 合并集群状态
		c.receiveGossip(gossipMsg)
		json.NewEncoder(w).Encode("接收成功")
	})
}
