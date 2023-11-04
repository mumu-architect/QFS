package main

import (
	"encoding/json"
	"fmt"
	"mumu.com/common/config"
	"mumu.com/common/function"
	"mumu.com/common/logger"
	"mumu.com/monitor/server"
	"net"
	"strconv"
	"strings"
	"time"
)

func main() {
	//接收处理监控主节点的数据
	go server.SynchronizeMonitorReceiveData()
	//监控节点数据服务
	go server.MonitorServer()

	// 选主
	go func() {
		monitorRaftSelectionObject, err := server.MonitorRaftSelectionRun()
		if err == nil {
			logger.Error.Println(err)
			return
		}
		//masterIpAddr := monitorRaftSelectionObject.GetLeaderIpAddr()
		//发送主节点ip到数据节点
		nodeHeartData := server.GETNodeHeartInstance()
		nodeDataMap := nodeHeartData.GetNodeMap()
		message := server.NewMonitorSynchronizeSendData("UPDATE", "json", "master", monitorRaftSelectionObject)
		nodeDataMap.Range(func(k string, v server.NodeData) bool {
			nodeData := v
			if nodeData.Alive == true {
				address := fmt.Sprintf("%s:%d", nodeData.Host, nodeData.Port)
				conn, err := net.Dial("tcp", address)
				if err != nil {
					fmt.Printf("Failed to connect to %s: %s\n", address, err)
					return false
				}
				defer conn.Close()
				//message := "Hello from master node!"
				//message := NewMonitorSynchronizeSendData('')
				// 将JSON数据编码为字节流
				jsonData, err := json.Marshal(message)
				if err != nil {
					logger.Error.Println("JSON编码错误:", err)
					return false
				}
				_, err = conn.Write(jsonData)
				if err != nil {
					logger.Error.Printf("Failed to send message to %s: %s\n", address, err)
				} else {
					logger.Info.Printf("Message sent to %s successfully\n", address)
				}
			}
			return true
		})

	}()
	//ScanMonitor 扫描监控
	server.ScanMonitor()
	// 监听心跳响应
	receiveHeartbeat()

}

type Node struct {
	address string
	port    uint8
}

// 监听来自节点的心跳响应
func receiveHeartbeat() {
	func() {

		//使用选举主服务器，判断主服务器ip和当前机器Ip相同后执行监听服务
		configMasterAddr, _ := config.TwoConfigParam{}.GetConfigParam("./config/MonitorConfig.yaml", "monitor", "MasterHostAddr")
		HostPort, _ := config.TwoConfigParam{}.GetConfigParam("./config/MonitorConfig.yaml", "monitor", "HostPort")
		hostIp, e := function.GetNativeIP()
		if e != nil {
			fmt.Println("ip不存在 port")
		}
		remoteIp := "127.0.0.1"
		remotePort, _ := strconv.ParseUint(HostPort, 10, 32)
		address := fmt.Sprintf("%s:%d", hostIp, remotePort)
		if configMasterAddr == address {
			remoteIp = hostIp
		} else {
			return
		}
		//remoteIp := hostIp
		//remotePort := 8081
		// 目标IP和端口
		remoteAddress := fmt.Sprintf("%s:%d", remoteIp, remotePort)

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
			fmt.Println("Accepted connection from", conn.RemoteAddr().String())

			//获取ip,端口
			addrString := conn.RemoteAddr().String()
			addrInfo := strings.Split(addrString, ":")
			port, err := strconv.ParseUint(addrInfo[1], 10, 32)
			if err != nil {
				fmt.Println("Port conversion number error")
			}
			nodeHeartData := server.GETNodeHeartInstance()
			oldNodeData, isBool := nodeHeartData.GetNode(addrString)
			var message server.MonitorSynchronizeSendData
			if isBool == true {
				oldNodeData.SetNewHeartTime(uint64(time.Now().UnixMicro()))
				oldNodeData.SetWeight(16)
				oldNodeData.SetAlive(true)
				nodeHeartData.AddNode(addrString, oldNodeData)
				message = server.NewMonitorSynchronizeSendData("UPDATE", "json", addrString, oldNodeData)
			} else {
				nodeData := server.NewNodeData(addrInfo[0], uint32(port))
				nodeData.SetNewHeartTime(uint64(time.Now().UnixMicro()))
				nodeHeartData.AddNode(addrString, nodeData)
				message = server.NewMonitorSynchronizeSendData("INSERT", "json", addrString, nodeData)
			}
			//同步监控集群数据
			server.SynchronizeMonitorData(message)
			fmt.Printf("[mumu]%+v\n", nodeHeartData)

			// 处理接收到的消息并打印出来
			handleHeartbeat(conn)

			//ch <- Node{address: conn.RemoteAddr().String()}
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
		fmt.Println("Received heartbeat message:", message)
	}
}
