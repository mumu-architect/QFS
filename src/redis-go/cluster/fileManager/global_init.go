package fileManager

import (
	"net"
	"net/rpc"
)

var GlobalSyncManager *SlaveSyncManager
var GlobalSyncRPCService *SyncRPCService

func InitSyncModule() {
	GlobalSyncManager = NewSlaveSyncManager(DefaultWorkerNum, DefaultQueueCap)
	GlobalSyncRPCService = &SyncRPCService{SyncMgr: GlobalSyncManager}

	// 强制清空默认RPC，防止冲突
	rpc.DefaultServer = rpc.NewServer()
	// 注册服务
	rpc.Register(GlobalSyncRPCService)
}

func StartRPCServer(listenAddr string) {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go rpc.DefaultServer.ServeConn(conn)
		}
	}()
}
