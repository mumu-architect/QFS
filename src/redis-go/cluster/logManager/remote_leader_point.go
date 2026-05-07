package logManager

// 全局点位元数据 存入Dragonboat 999分片
type NodeCursorLogMeta struct {
	NodeID                string `json:"nodeID"`                // 节点唯一标识
	ShardID               uint64 `json:"shardID"`               // 固定999
	RemoteCurrMaxFileName string `json:"remoteCurrMaxFileName"` // 当前日志文件名
	RemoteCurrMaxFlowID   string `json:"remoteCurrMaxFlowID"`   // 全局最新流水ID
}
