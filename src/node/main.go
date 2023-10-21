package main

import (
	"fmt"
	"net"
	"node/server"
	"time"
)

func main() {
	//监控监控服务器数据服务
	go server.MonitorServer()
	//nodes := []MonitorNodeAddr{
	//	{address: "127.0.0.1", port: 8081}, // 替换为你的节点地址和端口
	//}
	sendHeartbeats()
}

// 发送心跳包给指定的节点
func sendHeartbeat(ip string, port uint32) bool {
	// 目标IP和端口
	remoteAddress := fmt.Sprintf("%s:%d", ip, port)

	//便于调试,修改ip
	a := "192.168.1.4:0"
	b, err := net.ResolveTCPAddr("tcp", a)
	d := &net.Dialer{
		LocalAddr: b,
		Timeout:   30 * time.Second,
	}

	//真正的发送心跳
	conn, err := d.Dial("tcp", remoteAddress)

	//conn, err := net.Dial("tcp", remoteAddress)

	if err != nil {
		fmt.Printf("无法连接到 %s:%d %s\n", ip, port, err)
		return false
	}
	// 关闭连接
	defer conn.Close()
	// 获取本地的IP和端口号
	localAddr := conn.LocalAddr().String()
	fmt.Println("Local address:", localAddr)
	// 发送心跳包数据
	_, err = conn.Write([]byte("heartbeat" + localAddr))
	if err != nil {
		fmt.Printf("send heartbeat %s:%d %s\n", ip, port, err)
		return false
	}
	fmt.Printf("node send heartbeat %s:%d成功\n", ip, port)
	return true
}

// 发送心跳包给指定的节点
func sendHeartbeats() {
	server.SendHeartbeats(sendHeartbeat)
}
