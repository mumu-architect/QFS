package fileManager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type IncrementTaskManager struct {
	NodeFile    *NodeFile
	taskChan    chan *IncrementSyncTask
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewIncrementTaskManager(nodeFile *NodeFile, workerNum int, queueCap int) (*IncrementTaskManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	itm := &IncrementTaskManager{
		NodeFile:    nodeFile,
		taskChan:    make(chan *IncrementSyncTask, queueCap),
		workerCount: workerNum,
		ctx:         ctx,
		cancel:      cancel,
	}
	//TODO:批量把增量日志写入worker
	go func() {
		tt := time.NewTicker(500 * time.Millisecond)
		defer tt.Stop()
		for {
			fmt.Println("===开始执行Dispatch ===") // 加这里

			err := itm.DispatchOriginToWorker()

			fmt.Println("===执行完成Dispatch===") // 加这里
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				fmt.Printf("dispatch origin to worker failed, err:%v\n", err)
				continue
			}
			return
		}
	}()

	for i := 0; i < workerNum; i++ {
		go itm.workerLoop()
	}
	return itm, nil
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

// DispatchOriginToWorker
// 作用：将所有原始日志任务批量分发给协程池异步执行
// 返回：分发错误
// 逻辑：按文件、按批次（500条）投递，执行后标记完成
func (itm *IncrementTaskManager) DispatchOriginToWorker() error {
	files, err := itm.NodeFile.IncrementFileManager.getAllOriginFiles()
	if err != nil {
		return err
	}
	fmt.Printf("44incrementSyncTask===:%v\n", 11111)
	// 遍历所有原始分片
	for _, filePath := range files {
		fmt.Printf("55incrementSyncTask===:%v\n", 55)
		var offset int64 = 0
		// 批量读取任务
		for {
			//batchLines, isEnd, err := batchReadLimit(filePath, itm.NodeFile.IncrementFileManager.BatchOriginNum)
			batchLines, newOffset, isEnd, err := batchReadLimitWithOffset(filePath, itm.NodeFile.IncrementFileManager.BatchOriginNum, offset)
			fmt.Printf("66incrementSyncTask===:%v=====%v====%v\n", batchLines, offset, err)
			if err != nil || (isEnd && len(batchLines) <= 0) {
				break
			}
			offset = newOffset
			fmt.Printf("1incrementSyncTask===:%v\n", 2222)
			// 提交任务到协程池
			linesCopy := batchLines
			for _, line := range linesCopy {
				// 执行业务逻辑
				var incrementSyncTask *IncrementSyncTask
				_ = json.Unmarshal([]byte(line), &incrementSyncTask)
				fmt.Printf("1incrementSyncTask===:%v\n", incrementSyncTask)
				fmt.Printf("1incrementSyncTask===:%v\n", 3333)
				//任务投递到worker
				itm.PushTask(incrementSyncTask)
				// 标记完成
				//taskID := getTaskIDFromLine(line)
				//_ = m.AppendFinish(taskID)
			}
			//
			//// 提交任务到协程池
			//linesCopy := batchLines
			//workerChan <- func() {
			//	for _, line := range linesCopy {
			//		// 执行业务逻辑
			//		m.doTaskBusiness(line)
			//		// 标记完成
			//		taskID := getTaskIDFromLine(line)
			//		_ = m.AppendFinish(taskID)
			//	}
			//}
		}
	}
	return nil
}
