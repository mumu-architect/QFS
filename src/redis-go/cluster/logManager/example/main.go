package main

import (
	"fmt"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster"
	"mumu.com/redis-go/cluster/logManager"
)

func main() {
	nl, _ := logManager.NewNodeLog(128, 1, 1, "127.0.0.1", 8081, "1=127.0.0.1:8081,2=127.0.0.1:8082,3=127.0.0.1:8083")
	defer func(LocalLog *wal.Log) {
		err := LocalLog.Close()
		if err != nil {

		}
	}(nl.LocalLog)
	////TODO:初始化雪花id
	//snowflake.InitIDgenerator()
	//flakeId := snowflake.GetFlowID()
	//fmt.Printf("snowflake.GetFlowID():%v\n", flakeId)
	//// 示例：写入本地日志调用
	//for i := 0; i < 30; i++ {
	//	// 测试写入
	//	//file信息
	//	flakeId := snowflake.GetFlowID()
	//	fileInfo := fileManager.FileInfo{
	//		FIleID:     flakeId,
	//		FileName:   "aaaaaa",
	//		FilePath:   "data/a/aa.jpg",
	//		FileSize:   123232,
	//		MineType:   "image/jpeg ",
	//		CreateTime: time.Now().UnixMilli(),
	//		UpdateTime: time.Now().UnixMilli(),
	//		IsDeleted:  false,
	//	}
	//	fileInfoString, _ := json.Marshal(fileInfo)
	//	key := fileManager.GenerateFileCacheKey(flakeId)
	//	//文件key信息
	//	entry := logManager.LogEntry{
	//		FlowID:    flakeId,
	//		CMD:       "Set",
	//		Key:       key,
	//		Field:     "age",
	//		Value:     string(fileInfoString),
	//		Version:   10,
	//		Timestamp: time.Now().UnixMilli(),
	//	}
	//	entry.Key = entry.Key + strconv.Itoa(i)
	//	entry.Value = entry.Value + strconv.Itoa(i)
	//	data, err := json.Marshal(entry)
	//	if err != nil {
	//		fmt.Printf("json.Marshal：%v\n", err)
	//		break
	//	}
	//	_ = nl.WriteLocalLog(uint64(i), data)
	//	err = nl.WriteLocalLog(uint64(i), data)
	//	if errors.Is(err, wal.ErrOutOfOrder) {
	//		fmt.Printf("重复索引：%d\n", i)
	//		continue
	//	}
	//}
	//
	firstIndex, _ := nl.LocalLog.FirstIndex()
	fmt.Printf("读取日志firstIndex:%d\n", firstIndex)
	lastIndex, _ := nl.LocalLog.LastIndex()
	fmt.Printf("读取日志lastIndex:%d\n", lastIndex)
	// 启动分批加载
	//_ = logManager.RestartBatchLoad()
	cl := cluster.NewCluster(128, 1, 1, "127.0.0.1", 9001, "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003", "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006", nil, nil, nil, nil, "127.0.0.1:9001")

	//TODO:重启分批加载本地log数据到内存
	//err := nl.RestartBatchLoadLog(cl)
	//if err != nil {
	//	fmt.Printf("RestartBatchLoad:%v\n", err)
	//}
	val, _ := cl.GetStringData("File:13953530982930433716")
	fmt.Printf("name1:%v\n", val)
	val2, _ := cl.GetStringData("name49")
	fmt.Printf("name49:%v\n", val2)
	//TODO:本地数据同步完成同步leader的增量数据
	//indexID, err := nl.LogIndexGenerator()
	//if err != nil {
	//	fmt.Printf("nl.LogIndexGenerator:%v\n", err)
	//}
	//fmt.Printf("indexID:%v\n", indexID)

	//TODO:实时获取新的leader
	//TODO:动态修改cluster中所有的leaderID
	//go func() {
	//	tt := time.NewTicker(3 * time.Second)
	//	for range tt.C {
	//		//TODO:动态修改cluster中所有的leaderID
	//		shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, node.ShardID)
	//		if err != nil {
	//			fmt.Printf("222====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
	//		}
	//		fmt.Printf("222====nodeMeta.GetShardNodeLeaderID data:%v \n", shardNodeLeaderID)
	//		if nl.ShardID == node.ShardID {
	//			nl.LeaderID = shardNodeLeaderID.LeaderID
	//		}
	//
	//	}
	//}()

	//循环拉去leader的增量数据到本地节点
	//go logManager.PullSyncLoop(cl, "127.0.0.1:8081")
	//TODO:阻塞主进程
	//select {}
}
