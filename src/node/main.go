package main

import (
	"fmt"
	"net"
	"time"
)

func main() {

	nodes := []Node{
		{address: "127.0.0.1", port: 8081}, // 替换为你的节点地址和端口
	}
	sendHeartbeats(nodes)
}

type Node struct {
	address string
	port    uint32
}

// 发送心跳包给指定的节点
func sendHeartbeat(ip string, port uint32) {
	// 目标IP和端口
	remoteAddress := fmt.Sprintf("%s:%d", ip, port)

	//便于调试,修改ip
	a := "127.0.0.2:0"
	b, err := net.ResolveTCPAddr("tcp", a)
	d := &net.Dialer{
		LocalAddr: b,
		Timeout:   30 * time.Second,
	}

	//真正的发送心跳
	conn, err := d.Dial("tcp", remoteAddress)

	//conn, err := net.Dial("tcp", remoteAddress)

	if err != nil {
		fmt.Printf("无法连接到 %s：%d %s\n", ip, port, err)
		return
	}
	// 关闭连接
	defer conn.Close()
	// 获取本地的IP和端口号
	localAddr := conn.LocalAddr().String()
	fmt.Println("Local address:", localAddr)
	// 发送心跳包数据
	_, err = conn.Write([]byte("心跳包" + localAddr))
	if err != nil {
		fmt.Printf("无法发送心跳包到 %s:%d %s\n", ip, port, err)
		return
	}
	fmt.Printf("发送心跳包到 %s:%d成功\n", ip, port)

}

// 发送心跳包给指定的节点
func sendHeartbeats(nodes []Node) {
	for _, node := range nodes {
		func(node Node) {
			for i := 1; ; i++ {
				fmt.Printf("%d", i)
				sendHeartbeat(node.address, node.port)
				// 等待一段时间再发送下一个心跳包
				time.Sleep(time.Second)
			}
		}(node)
	}
}
