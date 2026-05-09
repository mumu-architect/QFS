package cluster

import (
	"fmt"
	"net"
	"net/rpc"
)

// StartRPCServer 启动RPC服务 所有节点都要开启
func (cl *Cluster) StartRPCServer() {
	rpcAddr := fmt.Sprintf(":%d", cl.LocalNode.RpcPort)
	_ = rpc.Register(new(KeyRPCService))
	listener, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go rpc.ServeConn(conn)
		}
	}()
}
