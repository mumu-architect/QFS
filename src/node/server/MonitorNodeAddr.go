package server

import (
	"common/config"
	"common/logger"
	"fmt"
	"github.com/cornelk/hashmap"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MonitorNodeData struct {
	monitorNodeMap *hashmap.Map[string, MonitorNodeAddr]
}

var monitorNodeInstance *MonitorNodeData
var monitorNonce sync.Once

func GetMonitorNodeInstance() *MonitorNodeData {
	monitorNonce.Do(func() {
		monitorNodeInstance = &MonitorNodeData{
			monitorNodeMap: hashmap.New[string, MonitorNodeAddr](),
		}
	})
	return monitorNodeInstance
}

// 判断节点是否存在
func (mnd *MonitorNodeData) isNodeExist(key string) bool {
	_, e := mnd.monitorNodeMap.Get(key)
	return e
}

// AddNode   添加所有节点
func (mnd *MonitorNodeData) AddNode(key string, val MonitorNodeAddr) {
	mnd.monitorNodeMap.Set(key, val)
}

// GetNode 获取节点数据
func (mnd *MonitorNodeData) GetNode(key string) (MonitorNodeAddr, bool) {
	nodeData, e := mnd.monitorNodeMap.Get(key)
	return nodeData, e
}

// MonitorNodeAddr 监控器是否存活
type MonitorNodeAddr struct {
	Ip       string
	Port     uint32
	IsMaster bool
	Alive    bool
	DieNum   int
}

// NewMonitorNodeAddr  定义一个结构体类型的构造函数
func NewMonitorNodeAddr(host string, port uint32) MonitorNodeAddr {
	return MonitorNodeAddr{
		Ip:       host,
		Port:     port,
		IsMaster: false,
		Alive:    false,
		DieNum:   50,
	}
}

// SetDieNum  设置死亡权重
func (n *MonitorNodeAddr) SetDieNum(dieNum int) {
	n.DieNum = dieNum
}

// DecrementDieNum  自减死亡权重
func (n *MonitorNodeAddr) DecrementDieNum() {
	n.DieNum--
}

// SetIsMaster 设置节点是否存活
func (n *MonitorNodeAddr) SetIsMaster(isMaster bool) {
	n.IsMaster = isMaster
}

// SetAlive 设置节点是否存活
func (n *MonitorNodeAddr) SetAlive(alive bool) {
	n.Alive = alive
}

// ToString 转字符串
func (n *MonitorNodeAddr) ToString() MonitorNodeAddr {
	//objString := fmt.Sprintf("{'host':%s,'port':%d,'newHeartTime':%d,'weight':%d,'alive':%t}", n.host, n.port, n.newHeartTime, n.weight, n.alive)
	obj := MonitorNodeAddr{
		Ip:       n.Ip,
		Port:     n.Port,
		IsMaster: n.IsMaster,
		Alive:    n.Alive,
		DieNum:   n.DieNum,
	}
	return obj
}

// GetMonitorNodeAddr 获取监控节点地址
func GetMonitorNodeAddr() ([]MonitorNodeAddr, error) {
	//读取yml
	HostAddrList, err := config.GetMonitorHostAddr("../monitor/config/MonitorConfig.yaml")
	MasterHostAddr, err := config.TwoConfigParam{}.GetConfigParam("../monitor/config/MonitorConfig.yaml", "monitor", "MasterHostAddr")

	if err != nil {
		logger.Error.Printf("%s", err)
		return nil, err
	}
	var monitorNodeAdds []MonitorNodeAddr
	monitorNodeData := GetMonitorNodeInstance()
	for _, value := range HostAddrList {
		HostAddr := strings.Split(value, ":")
		port, err := strconv.ParseUint(HostAddr[1], 10, 32)
		if err != nil {
			logger.Error.Printf("Port conversion number error:%s", err)
			return nil, err
		}

		addrString := value
		monitorServer := NewMonitorNodeAddr(HostAddr[0], uint32(port))
		if addrString == MasterHostAddr {
			monitorServer.SetIsMaster(true)
		}
		monitorNodeData.AddNode(addrString, monitorServer)

		//添加对象
		monitorNodeAdds = append(monitorNodeAdds, monitorServer)

		//logger.Info.Printf("%s", monitorServer.ToString())
	}
	//logger.Info.Printf(monitorNodeData.monitorNodeMap.String())
	return monitorNodeAdds, nil
}

// SendHeartbeats 发送心跳包给指定的节点
func SendHeartbeats(method func(string, uint32) bool) {
	//GetMonitorNodeAddr 获取监控服务器节点地址
	monitorNodeAdds, err := GetMonitorNodeAddr()
	if err != nil {
		logger.Error.Printf("%s", err)
	}

	for _, node := range monitorNodeAdds {
		func(node MonitorNodeAddr) {
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
				MonitorNodeData := GetMonitorNodeInstance()
				addrString := fmt.Sprintf("%s:%d", node.Ip, node.Port)
				oldNodeData, isBool := MonitorNodeData.GetNode(addrString)
				if isBool {
					oldNodeData.SetDieNum(node.DieNum)
					oldNodeData.SetAlive(node.Alive)
					MonitorNodeData.AddNode(addrString, oldNodeData)
				} else {
					nodeData := NewMonitorNodeAddr(node.Ip, node.Port)
					MonitorNodeData.AddNode(addrString, nodeData)
				}
				if node.Alive == false {
					break
				}
				//logger.Info.Println(MonitorNodeData.monitorNodeMap.String())
				// 等待一段时间再发送下一个心跳包
				time.Sleep(time.Second)
			}
		}(node)
	}
}
