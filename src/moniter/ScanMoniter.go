package main

import (
	"fmt"
	"time"
)

func ScanMoniter() {
	// 创建一个每隔1秒触发一次的定时器
	ticker := time.NewTicker(10 * time.Second)

	// 启动一个 goroutine 来执行定时扫描的任务
	go func() {
		for {
			<-ticker.C // 等待定时器触发的事件

			// 在这里执行你的扫描监控类数据的逻辑
			// 例如，可以从文件、数据库或网络中读取数据，并进行处理
			//time.Sleep(60 * time.Second)
			fmt.Println("扫描监控数据")
			nodeHeartData := GetInstance()
			nodeDataMap := nodeHeartData.nodeMap

			nodeDataMap.Range(func(k string, v NodeData) bool {
				addrString := k
				nodeData := v
				nowTime := uint64(time.Now().UnixMicro())
				offlineTime := nowTime - nodeData.NewHeartTime
				//权重为0
				if nodeData.Weight <= 0 {
					nodeData.SetWeight(0)
					nodeData.SetAlive(false)
				} else {
					//30秒未在线
					if offlineTime > 30*1e6 {
						nodeData.DecrementWeight()
					}
				}
				//修改后重新放入
				nodeDataMap.Set(addrString, nodeData)
				return true
			})
		}
	}()

	// 主程序休眠一段时间，以保持定时器运行
	time.Sleep(10 * time.Second)
}
