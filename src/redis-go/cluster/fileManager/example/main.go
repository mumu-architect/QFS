package main

import (
	"fmt"
	"os"
	"time"

	"mumu.com/redis-go/cluster/fileManager"
)

func main() {
	fmt.Println("===== QFS 主从文件同步（RPC已开启）=====\n")

	// 1. 初始化同步模块 + 注册RPC服务
	fileManager.InitSyncModule()

	// 2. 启动从机 RPC 服务
	fileManager.StartRPCServer(":8081")
	fmt.Println("✅ RPC 服务已启动 :8081")

	time.Sleep(1 * time.Second)

	// 3. 测试
	testSingleSync()
	testMultiSync()

	fmt.Println("\n🎉 全部完成！")
}

func testSingleSync() {
	fmt.Println("\n>>> 测试单个文件同步")

	_ = os.MkdirAll("./test_upload", 0755)
	src := "./test_upload/test.jpg"
	content := []byte("主节点文件→从机同步")
	_ = os.WriteFile(src, content, 0644)
	//
	//task := &fileManager.SyncTask{
	//	TaskID:     "t1",
	//	SourceURL:  "file://" + src,
	//	TargetPath: "./slave_data/test.jpg",
	//	FileSize:   int64(len(content)),
	//}
	// 2.计算MD5
	fileMD5, _ := fileManager.GetFileMD5(src)

	// 3.组装任务，带上MD5
	task := &fileManager.SyncTask{
		TaskID:     "t1",
		SourceURL:  "file://" + src,
		TargetPath: "./slave_data/test.jpg",
		FileSize:   int64(len(content)),
		FileMD5:    fileMD5, // 传给从机
	}

	// 推送到从机
	fileManager.LeaderPushSyncToSlaves([]string{"127.0.0.1:8081"}, task)

	// 等待足够时间让 Worker 执行完成
	time.Sleep(3 * time.Second)

	// 检查结果
	if _, err := os.Stat(task.TargetPath); err == nil {
		fmt.Println("✅ 同步成功")
	} else {
		fmt.Println("❌ 同步失败，原因：", err)
	}

	//os.RemoveAll("./test_upload")
	//os.RemoveAll("./slave_data")
}

func testMultiSync() {
	fmt.Println("\n>>> 测试 10 个并发文件同步")
	_ = os.MkdirAll("./test_upload", 0755)

	// 提交10个任务
	for i := 1; i <= 10; i++ {
		src := fmt.Sprintf("./test_upload/f%d.txt", i)
		content := []byte(fmt.Sprintf("文件 %d", i))
		_ = os.WriteFile(src, content, 0644)

		//task := &fileManager.SyncTask{
		//	TaskID:     fmt.Sprintf("t%d", i),
		//	SourceURL:  "file://" + src,
		//	TargetPath: fmt.Sprintf("./slave_data/f%d.txt", i),
		//	FileSize:   int64(len(content)),
		//}
		fileMD5, _ := fileManager.GetFileMD5(src)
		task := &fileManager.SyncTask{
			TaskID:     fmt.Sprintf("t%d", i),
			SourceURL:  "file://" + src,
			TargetPath: fmt.Sprintf("./slave_data/f%d.txt", i),
			FileSize:   int64(len(content)),
			FileMD5:    fileMD5, // 传给从机
		}

		fileManager.LeaderPushSyncToSlaves([]string{"127.0.0.1:8081"}, task)
	}

	// 等待全部同步完成
	time.Sleep(4 * time.Second)

	success := 0
	for i := 1; i <= 10; i++ {
		path := fmt.Sprintf("./slave_data/f%d.txt", i)
		if _, err := os.Stat(path); err == nil {
			success++
		}
	}

	fmt.Printf("✅ 成功：%d/10\n", success)
	//os.RemoveAll("./test_upload")
	//os.RemoveAll("./slave_data")
}
