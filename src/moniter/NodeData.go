package main

import (
	"github.com/cornelk/hashmap"
	"sync"
)

// NodeHeartData 存储节点数据的map
type NodeHeartData struct {
	nodeMap *hashmap.Map[string, NodeData]
}

var instance *NodeHeartData
var once sync.Once

func GetInstance() *NodeHeartData {
	once.Do(func() {
		instance = &NodeHeartData{
			nodeMap: hashmap.New[string, NodeData](),
		}
	})
	return instance
}

// 判断节点是否存在
func (nhd *NodeHeartData) isNodeExist(key string) bool {
	_, e := nhd.nodeMap.Get(key)
	return e
}

// AddNode 添加节点
func (nhd *NodeHeartData) AddNode(key string, val NodeData) {
	nhd.nodeMap.Set(key, val)
}

// GetNode 获取节点数据
func (nhd *NodeHeartData) GetNode(key string) (NodeData, bool) {
	nodeData, e := nhd.nodeMap.Get(key)
	return nodeData, e
}

// NodeData 节点数据类型
type NodeData struct {
	Host         string
	Port         uint32
	Weight       uint8
	NewHeartTime uint64
	Alive        bool
}

// NewNodeData 定义一个结构体类型的构造函数
func NewNodeData(host string, port uint32) NodeData {
	return NodeData{
		Host:         host,
		Port:         port,
		NewHeartTime: 0,
		Weight:       16,
		Alive:        true,
	}
}

// SetNewHeartTime 设置最新心跳时间
func (n *NodeData) SetNewHeartTime(time uint64) {
	n.NewHeartTime = time
}

// SetWeight 设置权重
func (n *NodeData) SetWeight(weight uint8) {
	n.Weight = weight
}

// DecrementWeight 自减权重
func (n *NodeData) DecrementWeight() {
	n.Weight--
}

// SetAlive 设置节点是否存活
func (n *NodeData) SetAlive(alive bool) {
	n.Alive = alive
}

// ToString 转字符串
func (n *NodeData) ToString() NodeData {
	//objString := fmt.Sprintf("{'host':%s,'port':%d,'newHeartTime':%d,'weight':%d,'alive':%t}", n.host, n.port, n.newHeartTime, n.weight, n.alive)
	obj := NodeData{
		Host:         n.Host,
		Port:         n.Port,
		NewHeartTime: n.NewHeartTime,
		Weight:       n.Weight,
		Alive:        n.Alive,
	}
	return obj
}
