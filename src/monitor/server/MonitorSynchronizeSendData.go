package server

import (
	"Monitor/config"
	"common/logger"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// MonitorSynchronizeSendData 监控同步数据
type MonitorSynchronizeSendData struct {
	Operate      string
	Type         string //消息类型 json
	MessageKey   string
	MessageValue NodeData
}

func NewMonitorSynchronizeSendData(operate string, typeString string, messageKey string, messageValue NodeData) MonitorSynchronizeSendData {
	return MonitorSynchronizeSendData{
		Operate:      operate,
		Type:         typeString,
		MessageKey:   messageKey,
		MessageValue: messageValue,
	}
}

type NodeInfo struct {
	Ip   string
	Port uint32
}

// TcpCoonPool 链接池
func TcpCoonPool(addresses []string) map[string]net.Conn {
	// 创建一个TCP连接池
	connPool := make(map[string]net.Conn)

	// 遍历从节点地址列表，建立连接
	for _, address := range addresses {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Printf("Failed to connect to %s: %s\n", address, err)
			continue
		}
		connPool[address] = conn
	}
	return connPool
}

func SendMessage(message MonitorSynchronizeSendData, connPool map[string]net.Conn) {
	// 发送消息到所有从节点
	for address, conn := range connPool {
		go func(address string, conn net.Conn) {
			defer conn.Close()
			//message := "Hello from master node!"
			//message := NewMonitorSynchronizeSendData('')
			// 将JSON数据编码为字节流
			jsonData, err := json.Marshal(message)
			if err != nil {
				logger.Error.Println("JSON编码错误:", err)
				return
			}
			_, err = conn.Write(jsonData)
			if err != nil {
				logger.Error.Printf("Failed to send message to %s: %s\n", address, err)
			} else {
				logger.Info.Printf("Message sent to %s successfully\n", address)
			}
		}(address, conn)
	}
}

func SynchronizeMonitorData(message MonitorSynchronizeSendData) {
	// // 定义从节点的地址列表
	//addresses := []string{
	//	"127.0.0.1:8001",
	//	"127.0.0.1:8002",
	//	"127.0.0.1:8003",
	//}
	configSynchronizeHostAddr, _ := config.TwoConfigParam{}.GetConfigParam("./config/MonitorConfig.yaml", "monitor", "SynchronizeHostAddr")
	addresses := strings.Split(configSynchronizeHostAddr, ",")
	connPool := TcpCoonPool(addresses)
	SendMessage(message, connPool)
}
