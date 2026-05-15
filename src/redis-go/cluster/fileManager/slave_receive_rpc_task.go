package fileManager

import "fmt"

// SyncRPCService 从机RPC处理服务
type SyncRPCService struct {
	SyncMgr  *SlaveSyncManager
	NodeFile *NodeFile
}
type EmptyReply struct{}

// ReceiveSyncTask RPC 接收任务
// 这里必须 真正把任务丢给同步池！
func (srs *SyncRPCService) ReceiveSyncTask(task *SyncTask, empty *struct{}) error {
	fmt.Println("ReceiveSyncTask收到同步任务:", task.LocalPath)

	// 关键：必须推入队列
	ok := srs.SyncMgr.PushTask(task)
	if ok {
		fmt.Println("【从机】任务已进入同步队列 -> 即将开始同步")
	} else {
		fmt.Println("【从机】任务入队失败")
	}

	//// 同步执行一次（强制保证测试能看到文件）
	////go DoRemoteFileSync(task)
	//go srs.NodeFile.DoRemoteHttpFileSync(task)

	return nil
}
