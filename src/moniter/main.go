package main

import (
	"fmt"
	"moniter/server"
	"net"
	"strconv"
	"strings"
	"time"
)

func main() {
	//监控节点数据服务
	go server.MoniterServer()
	//ScanMoniter 扫描监控
	server.ScanMoniter()
	// 监听心跳响应
	receiveHeartbeat()

}

type Node struct {
	address string
	port    uint8
}

// 监听来自节点的心跳响应
func receiveHeartbeat() chan Node {
	ch := make(chan Node)
	func() {

		remoteIp := "127.0.0.1"
		remotePort := 8081
		// 目标IP和端口
		remoteAddress := fmt.Sprintf("%s:%d", remoteIp, remotePort)

		listener, err := net.Listen("tcp", remoteAddress) // 假设第一个节点作为心跳响应的接收者

		// 监听端口
		if err != nil {
			fmt.Println("Failed to listen on port", remotePort)
			return
		}
		defer listener.Close()
		fmt.Println("Server listening on port", remotePort)

		for i := 0; ; i++ {
			//接收心跳
			fmt.Printf("%d 接受心跳", i)
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

			if isBool == true {
				oldNodeData.SetNewHeartTime(uint64(time.Now().UnixMicro()))
				oldNodeData.SetWeight(16)
				oldNodeData.SetAlive(true)
				nodeHeartData.AddNode(addrString, oldNodeData)
			} else {
				nodeData := server.NewNodeData(addrInfo[0], uint32(port))
				nodeData.SetNewHeartTime(uint64(time.Now().UnixMicro()))
				nodeHeartData.AddNode(addrString, nodeData)
			}
			fmt.Printf("[mumu]%+v\n", nodeHeartData)

			// 处理接收到的消息并打印出来
			handleHeartbeat(conn)

			//ch <- Node{address: conn.RemoteAddr().String()}
		}
	}()
	return ch
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
