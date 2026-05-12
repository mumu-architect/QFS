package fileManager

import (
	"context"
)

type IncrementTaskManager struct {
	NodeFile    *NodeFile
	taskChan    chan *IncrementSyncTask
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewIncrementTaskManager(nodeFile *NodeFile, workerNum int, queueCap int) *IncrementTaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	itm := &IncrementTaskManager{
		NodeFile:    nodeFile,
		taskChan:    make(chan *IncrementSyncTask, queueCap),
		workerCount: workerNum,
		ctx:         ctx,
		cancel:      cancel,
	}

	for i := 0; i < workerNum; i++ {
		go itm.workerLoop()
	}
	return itm
}

// PushTask 临时关闭去重，保证测试100%能进任务
func (itm *IncrementTaskManager) PushTask(t *IncrementSyncTask) bool {
	select {
	case itm.taskChan <- t:
		return true
	default:
		return false
	}
}

// worker循环执行
func (itm *IncrementTaskManager) workerLoop() {
	for {
		select {
		case <-itm.ctx.Done():
			return
		case task := <-itm.taskChan:
			//_ = DoRemoteFileSync(task)
			itm.NodeFile.DoRemoteIncrementHttpFileSync(task)
		}
	}
}
