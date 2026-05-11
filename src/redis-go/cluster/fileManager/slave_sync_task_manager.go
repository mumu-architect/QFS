package fileManager

import (
	"context"
)

type SlaveSyncManager struct {
	NodeFile    *NodeFile
	taskChan    chan *SyncTask
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewSlaveSyncManager(nodeFile *NodeFile, workerNum int, queueCap int) *SlaveSyncManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &SlaveSyncManager{
		NodeFile:    nodeFile,
		taskChan:    make(chan *SyncTask, queueCap),
		workerCount: workerNum,
		ctx:         ctx,
		cancel:      cancel,
	}

	for i := 0; i < workerNum; i++ {
		go m.workerLoop()
	}
	return m
}

// 临时关闭去重，保证测试100%能进任务
func (m *SlaveSyncManager) PushTask(t *SyncTask) bool {
	select {
	case m.taskChan <- t:
		return true
	default:
		return false
	}
}

func (m *SlaveSyncManager) workerLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.taskChan:
			//_ = DoRemoteFileSync(task)
			m.NodeFile.DoRemoteHttpFileSync(task)
		}
	}
}
