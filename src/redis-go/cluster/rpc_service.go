package cluster

import (
	"fmt"
	"net"
	"net/rpc"

	"mumu.com/redis-go/cluster/fileManager"
)

// StartRPCServer 启动RPC服务 所有节点都要开启
func (cl *Cluster) StartRPCServer() {
	rpcAddr := fmt.Sprintf(":%d", cl.LocalNode.RpcPort)

	syncManager := fileManager.NewSlaveSyncManager(cl.NodeFile, fileManager.DefaultWorkerNum, fileManager.DefaultQueueCap)
	syncRPCService := &fileManager.SyncRPCService{
		SyncMgr:  syncManager,
		NodeFile: cl.NodeFile,
	}

	// 强制清空默认RPC，防止冲突,必须注释掉
	//rpc.DefaultServer = rpc.NewServer()
	// 注册服务
	_ = rpc.Register(syncRPCService)
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
