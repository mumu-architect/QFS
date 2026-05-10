package cluster

import (
	"fmt"
	"sync"
)

// 全局内存KV：仅Leader使用，存放route_key
var keySyncMap = new(sync.Map)

// RPC 服务对象
type KeyRPCService struct{}

// RPC请求体
type PutKeyArgs struct {
	RouteKey string
}

type EmptyReply struct{}

// 存入Key：给从节点远程调用
func (k *KeyRPCService) PutKey(args *PutKeyArgs, reply *EmptyReply) error {
	fmt.Printf("【Leader 已收到】写入 routeKey：%s\n", args.RouteKey)
	keySyncMap.Store(args.RouteKey, true)
	return nil
}

// 核销Key：上传时调用
func (k *KeyRPCService) DeleteKey(args *PutKeyArgs, reply *EmptyReply) bool {
	_, ok := keySyncMap.LoadAndDelete(args.RouteKey)
	return ok
}
