package server

import (
	"Monitor/config"
	"common/function"
	"common/logger"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// SynchronizeMonitorReceiveData 接收来自主监控节点数据
func SynchronizeMonitorReceiveData() {
	func() {
		//TODO:当前IP和监控主IP一样不处理
		//使用选举主服务器，判断主服务器ip和当前机器Ip相同后执行监听服务
		configSynchronizeHostAddr, _ := config.TwoConfigParam{}.GetConfigParam("./config/MonitorConfig.yaml", "monitor", "MasterHostAddr")
		SynchronizeHostPort, _ := config.TwoConfigParam{}.GetConfigParam("./config/MonitorConfig.yaml", "monitor", "HostPort")
		hostIp, e := function.GetNativeIP()
		if e != nil {
			fmt.Println("ip不存在 port")
		}

		remoteIp := hostIp
		remotePort, _ := strconv.ParseUint(SynchronizeHostPort, 10, 32)
		// 目标IP和端口
		remoteAddress := fmt.Sprintf("%s:%d", remoteIp, remotePort)
		configSynchronizeHostAddrArr := strings.Split(configSynchronizeHostAddr, ",")
		for _, v := range configSynchronizeHostAddrArr {
			if v != remoteAddress {
				return
			}
		}

		listener, err := net.Listen("tcp", remoteAddress) // 假设第一个节点作为心跳响应的接收者

		// 监听端口
		if err != nil {
			fmt.Println("Failed to listen on port", remotePort)
			return
		}
		defer listener.Close()
		fmt.Println("Server listening on port", remoteIp, remotePort)

		for i := 0; ; i++ {
			//接收心跳
			fmt.Printf("%d accept heartbeat", i)
			// 接受客户端连接请求
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("Failed to accept connection")
				continue
			}
			//defer conn.Close()
			fmt.Println("Synchronize monitor data", conn.RemoteAddr().String())
			// 处理接收到的消息并打印出来
			handleHeartbeat(conn)
		}
	}()
}

// 处理接收到的心跳消息并打印出来
func handleHeartbeat(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 1024)
	for {
		// 读取数据
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Printf("Failed to read data: %s", err)
			return
		}
		// 将读取到的数据转换为字符串并打印出来
		message := string(buffer[:n])

		//接收处理监控集群同步数据
		SynchronizeProcessData(message)
		fmt.Println("Received SynchronizeMonitorReceiveData message:", message)
	}
}

// SynchronizeProcessData 接收处理监控集群同步数据
func SynchronizeProcessData(message string) {
	jsonData := make(map[string]interface{})
	err := json.Unmarshal([]byte(message), &jsonData)
	if err != nil {
		logger.Error.Println("Unmarshal error ", err)
		return
	}
	operate := jsonData["Operate"]
	//typeString := jsonData["Type"]
	messageKey := jsonData["MessageKey"].(string)
	messageValue := jsonData["MessageValue"].(NodeData)
	nodeHeartData := GETNodeHeartInstance()
	switch operate {
	case "UPDATE":
		nodeHeartData.AddNode(messageKey, messageValue)
	case "INSERT":
		nodeHeartData.AddNode(messageKey, messageValue)
	}
}
