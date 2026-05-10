package main

import (
	"fmt"
	"time"

	"mumu.com/redis-go/cluster/fileManager"
)

// 测试步骤：
// 1. 先运行 启动主节点服务
// 2. 再运行 从节点下载测试
// 3. 测试主从切换 换端口自动换目录

func main() {

	// 等待服务启动
	time.Sleep(1 * time.Second)

	// ============ 1. 初始化从节点管理器 ============
	nf := fileManager.NewNodeFile(128, 3, 2, "127.0.0.1", 7073, "1=127.0.0.1:7071,2=127.0.0.1:7072,3=127.0.0.1:7073")
	// ============ 2. 启动主节点文件服务（8086端口） ============

	//TODO:动态获取leaderId并修改本地的leaderId
	//go func() {
	//	tt := time.NewTicker(3 * time.Second)
	//	for range tt.C {
	//		//TODO:动态修改cluster中所有的leaderID
	//		shardNodeLeaderID, err := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, node.ShardID)
	//		if err != nil {
	//			fmt.Printf("222====nodeMeta.GetShardNodeLeaderID error:%v \n", err)
	//		}
	//		fmt.Printf("222====nodeMeta.GetShardNodeLeaderID data:%v \n", shardNodeLeaderID)
	//		if shardNodeLeaderID == nil {
	//			continue
	//		}
	//		if nl.ShardId == node.ShardID {
	//			nl.LeaderId = shardNodeLeaderID.LeaderID
	//		}
	//	}
	//}()
	// ============ 3. 测试下载文件 A/111.txt ============
	// 提前手动在程序运行目录下创建 data-8086/A/111.txt
	relPath := "A/2.txt"
	err := nf.SyncFileByRelPath(relPath)
	if err != nil {
		fmt.Printf("文件同步失败：%v\n", err)
	}

	// ============ 4. 测试主从切换：切换到新主 8081 ============
	// 模拟Raft选主完成，更换主节点IP和端口
	//m.ChangeMasterAddr(2)

	// 后续下载自动使用 data-8081 目录
}
