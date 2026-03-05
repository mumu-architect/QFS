package cluster

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	TotalSlots = 16384
)

type ReplicationRole string

const (
	RoleMaster ReplicationRole = "master"
	RoleSlave  ReplicationRole = "slave"
)

type NodeState string

const (
	NodeStateOnline NodeState = "online"
	NodeStatePFail  NodeState = "pfail"
	NodeStateFail   NodeState = "fail"
)

type ClusterNode struct {
	ID           string
	Addr         string
	Role         ReplicationRole
	Slots        []int // 简化为存储所有槽位，而非范围
	MasterID     string
	LastSeen     time.Time
	State        NodeState
	PfailReports int
}

type ClusterState struct {
	sync.RWMutex
	Self           *ClusterNode
	Nodes          map[string]*ClusterNode
	SlotMap        [TotalSlots]string // slot -> nodeID
	migratingSlots map[int]string     // slot -> target node.go ID
	importingSlots map[int]string     // slot -> source node.go ID
}

func NewClusterState(selfID, selfAddr string) *ClusterState {
	self := &ClusterNode{
		ID:       selfID,
		Addr:     selfAddr,
		Role:     RoleMaster,
		State:    NodeStateOnline,
		LastSeen: time.Now(),
	}
	return &ClusterState{
		Self:           self,
		Nodes:          map[string]*ClusterNode{selfID: self},
		migratingSlots: make(map[int]string),
		importingSlots: make(map[int]string),
	}
}

func (cs *ClusterState) AddNode(node *ClusterNode) {
	cs.Lock()
	defer cs.Unlock()
	node.LastSeen = time.Now()
	cs.Nodes[node.ID] = node
	// 如果是主节点，更新槽位映射
	if node.Role == RoleMaster {
		for _, slot := range node.Slots {
			if slot >= 0 && slot < TotalSlots {
				cs.SlotMap[slot] = node.ID
			}
		}
	}
	fmt.Printf("[CLUSTER] Added new node.go: ID=%s, Addr=%s, Role=%s\n", node.ID, node.Addr, node.Role)
}

func (cs *ClusterState) UpdateNodeLastSeen(nodeID string) {
	cs.Lock()
	defer cs.Unlock()
	if node, ok := cs.Nodes[nodeID]; ok {
		node.LastSeen = time.Now()
		cs.Nodes[nodeID] = node
	}
}

func (cs *ClusterState) AssignSlotToNode(slot int, nodeID string) {
	cs.Lock()
	defer cs.Unlock()
	if _, ok := cs.Nodes[nodeID]; !ok {
		fmt.Printf("[CLUSTER] Cannot assign slot %d to unknown node.go %s\n", slot, nodeID)
		return
	}
	if slot >= 0 && slot < TotalSlots {
		cs.SlotMap[slot] = nodeID
	}
}

func (cs *ClusterState) GetSlotOwner(slot int) (string, bool) {
	cs.RLock()
	defer cs.RUnlock()
	if slot < 0 || slot >= TotalSlots {
		return "", false
	}
	nodeID := cs.SlotMap[slot]
	return nodeID, nodeID != ""
}

// SlotForKey 计算一个键对应的槽位 (简化版，不处理hash tag)
func SlotForKey(key string) int {
	if key == "" {
		return 0
	}
	h := 2166136261
	for _, c := range key {
		h ^= int(c)
		h *= 16777619
	}
	return h % TotalSlots
}

func (cs *ClusterState) MarkAsPFail(nodeID string) {
	cs.Lock()
	defer cs.Unlock()
	if node, ok := cs.Nodes[nodeID]; ok && node.State != NodeStateFail {
		node.State = NodeStatePFail
		node.PfailReports++
		cs.Nodes[nodeID] = node
		fmt.Printf("[CLUSTER] Node %s marked as PFAIL. Reports: %d\n", nodeID, node.PfailReports)
	}
}

func (cs *ClusterState) MarkAsFail(nodeID string) {
	cs.Lock()
	defer cs.Unlock()
	if node, ok := cs.Nodes[nodeID]; ok {
		node.State = NodeStateFail
		cs.Nodes[nodeID] = node
		fmt.Printf("[CLUSTER] Node %s marked as FAIL\n", nodeID)
	}
}

func (cs *ClusterState) GetNodeByID(nodeID string) (*ClusterNode, bool) {
	cs.RLock()
	defer cs.RUnlock()
	node, ok := cs.Nodes[nodeID]
	return node, ok
}

func (cs *ClusterState) GetNodeByAddr(addr string) (*ClusterNode, bool) {
	cs.RLock()
	defer cs.RUnlock()
	for _, node := range cs.Nodes {
		if node.Addr == addr {
			return node, true
		}
	}
	return nil, false
}

func (cs *ClusterState) GetAllNodes() []*ClusterNode {
	cs.RLock()
	defer cs.RUnlock()
	nodes := make([]*ClusterNode, 0, len(cs.Nodes))
	for _, node := range cs.Nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (cs *ClusterState) CountMasterNodes() int {
	cs.RLock()
	defer cs.RUnlock()
	count := 0
	for _, node := range cs.Nodes {
		if node.Role == RoleMaster {
			count++
		}
	}
	return count
}

func (cs *ClusterState) GetMigratingSlotOwner(slot int) (string, bool) {
	cs.RLock()
	defer cs.RUnlock()
	targetID, ok := cs.migratingSlots[slot]
	return targetID, ok
}

func (cs *ClusterState) SetMigratingSlot(slot int, targetID string) {
	cs.Lock()
	defer cs.Unlock()
	cs.migratingSlots[slot] = targetID
}

func (cs *ClusterState) RemoveMigratingSlot(slot int) {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.migratingSlots, slot)
}

func (cs *ClusterState) GetImportingSlotSource(slot int) (string, bool) {
	cs.RLock()
	defer cs.RUnlock()
	sourceID, ok := cs.importingSlots[slot]
	return sourceID, ok
}

func (cs *ClusterState) SetImportingSlot(slot int, sourceID string) {
	cs.Lock()
	defer cs.Unlock()
	cs.importingSlots[slot] = sourceID
}

func (cs *ClusterState) RemoveImportingSlot(slot int) {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.importingSlots, slot)
}

// AssignSlotRange 用于将一个连续的槽位范围分配给一个主节点
func (cs *ClusterState) AssignSlotRange(start, end int, nodeID string) {
	cs.Lock()
	defer cs.Unlock()
	if _, ok := cs.Nodes[nodeID]; !ok {
		fmt.Printf("[CLUSTER] Cannot assign slots to unknown node.go %s\n", nodeID)
		return
	}
	if start < 0 || end >= TotalSlots || start > end {
		fmt.Println("[CLUSTER] Invalid slot range")
		return
	}
	for slot := start; slot <= end; slot++ {
		cs.SlotMap[slot] = nodeID
	}
	fmt.Printf("[CLUSTER] Assigned slots %d-%d to node.go %s\n", start, end, nodeID)
}

// MeetNode 处理新节点加入集群的请求
func (cs *ClusterState) MeetNode(newNodeID, newNodeAddr string) {
	if existingNode, ok := cs.GetNodeByID(newNodeID); ok {
		fmt.Printf("[CLUSTER] Node %s is already in the cluster.\n", newNodeID)
		existingNode.Addr = newNodeAddr // 更新地址以防变化
		cs.AddNode(existingNode)        // 重新添加以更新
		return
	}
	newNode := &ClusterNode{
		ID:       newNodeID,
		Addr:     newNodeAddr,
		Role:     RoleMaster, // 默认作为主节点加入
		State:    NodeStateOnline,
		LastSeen: time.Now(),
	}
	cs.AddNode(newNode)
}

// PickRandomNode 随机选择一个除自己以外的节点
func (cs *ClusterState) PickRandomNode() *ClusterNode {
	cs.RLock()
	defer cs.RUnlock()
	nodes := make([]*ClusterNode, 0, len(cs.Nodes)-1)
	for _, node := range cs.Nodes {
		if node.ID != cs.Self.ID {
			nodes = append(nodes, node)
		}
	}
	if len(nodes) == 0 {
		return nil
	}
	return nodes[rand.Intn(len(nodes))]
}
