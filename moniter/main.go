package main

import (
	"fmt"
	"net"
)

func main() {
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
			defer conn.Close()
			fmt.Println("Accepted connection from", conn.RemoteAddr().String())

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
			fmt.Println("Failed to read data")
			return
		}
		// 将读取到的数据转换为字符串并打印出来
		message := string(buffer[:n])
		fmt.Println("Received heartbeat message:", message)
	}
}
