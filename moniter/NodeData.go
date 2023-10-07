package main

// NodeHeartData 存储节点数据的map
type NodeHeartData struct {
	nodeMap map[uint64]NodeData
}

// NodeData 节点数据类型
type NodeData struct {
	host         string
	port         uint32
	weight       uint8
	newHeartTime uint64
	alive        bool
}

// NewNodeData 定义一个结构体类型的构造函数
func NewNodeData(host string, port uint32, weight uint8, newHeartTime uint64, alive bool) *NodeData {
	return &NodeData{
		host:         host,
		port:         port,
		weight:       weight,
		newHeartTime: newHeartTime,
		alive:        alive,
	}
}
