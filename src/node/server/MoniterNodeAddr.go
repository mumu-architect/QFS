package server

import (
	"common/logger"
	"fmt"
	"github.com/cornelk/hashmap"
	"moniter/config"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MoniterNodeData struct {
	moniterNodeMap *hashmap.Map[string, MoniterNodeAddr]
}

var moniterNodeInstance *MoniterNodeData
var moniterNodeonce sync.Once

func GetMoniterNodeInstance() *MoniterNodeData {
	moniterNodeonce.Do(func() {
		moniterNodeInstance = &MoniterNodeData{
			moniterNodeMap: hashmap.New[string, MoniterNodeAddr](),
		}
	})
	return moniterNodeInstance
}

// 判断节点是否存在
func (mnd *MoniterNodeData) isNodeExist(key string) bool {
	_, e := mnd.moniterNodeMap.Get(key)
	return e
}

// AddNode   添加所有节点
func (mnd *MoniterNodeData) AddNode(key string, val MoniterNodeAddr) {
	mnd.moniterNodeMap.Set(key, val)
}

// GetNode 获取节点数据
func (mnd *MoniterNodeData) GetNode(key string) (MoniterNodeAddr, bool) {
	nodeData, e := mnd.moniterNodeMap.Get(key)
	return nodeData, e
}

// MoniterNodeAddr 监控器是否存活
type MoniterNodeAddr struct {
	Ip     string
	Port   uint32
	Alive  bool
	DieNum int
}

// NewMoniterNodeAddr  定义一个结构体类型的构造函数
func NewMoniterNodeAddr(host string, port uint32) MoniterNodeAddr {
	return MoniterNodeAddr{
		Ip:     host,
		Port:   port,
		Alive:  false,
		DieNum: 50,
	}
}

// SetDieNum  设置死亡权重
func (n *MoniterNodeAddr) SetDieNum(dieNum int) {
	n.DieNum = dieNum
}

// DecrementDieNum  自减死亡权重
func (n *MoniterNodeAddr) DecrementDieNum() {
	n.DieNum--
}

// SetAlive 设置节点是否存活
func (n *MoniterNodeAddr) SetAlive(alive bool) {
	n.Alive = alive
}

// ToString 转字符串
func (n *MoniterNodeAddr) ToString() MoniterNodeAddr {
	//objString := fmt.Sprintf("{'host':%s,'port':%d,'newHeartTime':%d,'weight':%d,'alive':%t}", n.host, n.port, n.newHeartTime, n.weight, n.alive)
	obj := MoniterNodeAddr{
		Ip:     n.Ip,
		Port:   n.Port,
		Alive:  n.Alive,
		DieNum: n.DieNum,
	}
	return obj
}

// GetMoniterNodeAddr 获取监控节点地址
func GetMoniterNodeAddr() ([]MoniterNodeAddr, error) {
	//读取yml
	HostAddrList, err := config.GetMonitorHostAddr("../moniter/config/MoniterConfig.yaml")
	if err != nil {
		logger.Error.Printf("%s", err)
		return nil, err
	}
	var moniterNodeAddrs []MoniterNodeAddr
	moniterNodeData := GetMoniterNodeInstance()
	for _, value := range HostAddrList {
		HostAddr := strings.Split(value, ":")
		port, err := strconv.ParseUint(HostAddr[1], 10, 32)
		if err != nil {
			logger.Error.Printf("Port conversion number error:%s", err)
			return nil, err
		}

		addrString := value
		moniterServer := NewMoniterNodeAddr(HostAddr[0], uint32(port))
		moniterNodeData.AddNode(addrString, moniterServer)

		//添加对象
		moniterNodeAddrs = append(moniterNodeAddrs, moniterServer)

		//logger.Info.Printf("%s", moniterServer.ToString())
	}
	//logger.Info.Printf(moniterNodeData.moniterNodeMap.String())
	return moniterNodeAddrs, nil
}

// SendHeartbeats 发送心跳包给指定的节点
func SendHeartbeats(method func(string, uint32) bool) {
	//GetMoniterNodeAddr 获取监控服务器节点地址
	moniterNodeAddrs, err := GetMoniterNodeAddr()
	if err != nil {
		logger.Error.Printf("%s", err)
	}

	for _, node := range moniterNodeAddrs {
		func(node MoniterNodeAddr) {
			for i := 1; ; i++ {
				//fmt.Printf("%d", i)
				res := method(node.Ip, node.Port)
				if res {
					node.Alive = true
					node.DieNum = 50
				} else {
					node.DieNum--
					if node.DieNum <= 0 {
						node.Alive = false
						node.DieNum = 0
					}
				}

				//修改监控服务节点信息
				moniterNodeData := GetMoniterNodeInstance()
				addrString := fmt.Sprintf("%s:%d", node.Ip, node.Port)
				oldNodeData, isBool := moniterNodeData.GetNode(addrString)
				if isBool {
					oldNodeData.SetDieNum(node.DieNum)
					oldNodeData.SetAlive(node.Alive)
					moniterNodeData.AddNode(addrString, oldNodeData)
				} else {
					nodeData := NewMoniterNodeAddr(node.Ip, node.Port)
					moniterNodeData.AddNode(addrString, nodeData)
				}
				if node.Alive == false {
					continue
				}
				//logger.Info.Println(moniterNodeData.moniterNodeMap.String())
				// 等待一段时间再发送下一个心跳包
				time.Sleep(time.Second)
			}
		}(node)
	}
}
