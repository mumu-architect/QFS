package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster"
	"mumu.com/redis-go/cluster/logManager"
)

func main() {
	nl, _ := logManager.NewNodeLog(8081)
	defer func(LocalLog *wal.Log) {
		err := LocalLog.Close()
		if err != nil {

		}
	}(nl.LocalLog)

	// 示例：写入本地日志调用
	for i := 0; i < 30; i++ {
		// 测试写入
		entry := logManager.LogEntry{
			FlowID:    "123456",
			FileID:    "434444",
			CMD:       "Set",
			Key:       "name",
			Field:     "age",
			Value:     "testData",
			Version:   10,
			Timestamp: time.Now().UnixMilli(),
		}
		entry.Key = entry.Key + strconv.Itoa(i)
		entry.Value = entry.Value + strconv.Itoa(i)
		data, err := json.Marshal(entry)
		if err != nil {
			fmt.Printf("json.Marshal：%v\n", err)
			break
		}
		_ = nl.WriteLocalLog(uint64(i), data)
		err = nl.WriteLocalLog(uint64(i), data)
		if errors.Is(err, wal.ErrOutOfOrder) {
			fmt.Printf("重复索引：%d\n", i)
			continue
		}
	}
	//
	firstIndex, _ := nl.LocalLog.FirstIndex()
	fmt.Printf("读取日志firstIndex:%d\n", firstIndex)
	lastIndex, _ := nl.LocalLog.LastIndex()
	fmt.Printf("读取日志lastIndex:%d\n", lastIndex)
	// 启动分批加载
	//_ = logManager.RestartBatchLoad()
	cl := cluster.NewCluster(128, 1, 1, "127.0.0.1", 9001, "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003", "1=127.0.0.1:9001,2=127.0.0.1:9002,3=127.0.0.1:9003,4=127.0.0.1:9004,5=127.0.0.1:9005,6=127.0.0.1:9006", nil, nil, nil, "127.0.0.1:9001")

	//TODO:重启分批加载本地log数据到内存
	err := nl.RestartBatchLoad(cl)
	if err != nil {
		fmt.Printf("RestartBatchLoad:%v\n", err)
	}
	val, _ := cl.GetStringData("name1")
	fmt.Printf("name1:%v\n", val)
	val2, _ := cl.GetStringData("name49")
	fmt.Printf("name49:%v\n", val2)
	//TODO:开启8081
	nl.RegisterLogHandlers()
	nl.StartHTTPAPI()

	//循环拉去leader的增量数据到本地节点
	//go logManager.PullSyncLoop(cl, "127.0.0.1:8081")
}
