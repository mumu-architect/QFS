package fileManager

import (
	"net/rpc"
)

func LeaderPushSyncToSlaves(slaveAddrs []string, task *SyncTask) {
	for _, addr := range slaveAddrs {
		go func(peer string) {
			_ = CallSlaveRPC(peer, "SyncRPCService.ReceiveSyncTask", task, nil)
		}(addr)
	}
}

// 纯 rpc.Dial() 实现
func CallSlaveRPC(addr, method string, args, reply interface{}) error {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Call(method, args, reply)
}
