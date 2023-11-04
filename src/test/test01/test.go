package main

import (
	"fmt"
	"net"
)

func main() {
	remoteIp := "192.168.1.5"
	remotePort := 9091
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
		fmt.Println("Synchronize monitor data", conn.RemoteAddr().String())

	}
}
