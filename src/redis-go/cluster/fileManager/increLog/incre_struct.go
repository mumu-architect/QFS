package increLog

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// 全局日志文件互斥锁 核心关键
var logFileLock sync.Mutex

const logFilePath = "./increment.log"
const batchSize = 500

const (
	StatusPending = "pending"
	StatusFinish  = "finish"
)

type IncrementLog struct {
	TaskID     string `json:"task_id"`
	FilePath   string `json:"file_path"`
	FileMD5    string `json:"file_md5"`
	TaskStatus string `json:"task_status"`
	CreateAt   int64  `json:"create_at"`
}

func BatchLoadPendingWithClean(pushFunc func([]IncrementLog) error) error {
	// 抢占锁
	logFileLock.Lock()
	defer logFileLock.Unlock()

	tmpPath := logFilePath + ".tmp"

	srcFile, err := os.Open(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	scanner := bufio.NewScanner(srcFile)
	writer := bufio.NewWriter(dstFile)

	var batchList []IncrementLog

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var logItem IncrementLog
		_ = json.Unmarshal([]byte(line), &logItem)

		// 直接丢弃finish
		if logItem.TaskStatus == StatusFinish {
			continue
		}

		if logItem.TaskStatus == StatusPending {
			batchList = append(batchList, logItem)
		}

		// 写回保留数据
		bs, _ := json.Marshal(logItem)
		writer.WriteString(string(bs) + "\n")

		// 满500条批量推送
		if len(batchList) >= batchSize {
			_ = pushFunc(batchList)
			batchList = batchList[:0]
		}
	}

	// 剩余不足批次推送
	if len(batchList) > 0 {
		_ = pushFunc(batchList)
	}

	writer.Flush()
	return os.Rename(tmpPath, logFilePath)
}

// UpdateLogToFinish 根据TaskID找到对应日志改为finish
func UpdateLogToFinish(taskID string) error {
	// 争抢同一把全局锁
	logFileLock.Lock()
	defer logFileLock.Unlock()

	tmpPath := logFilePath + ".tmp"

	srcFile, err := os.Open(logFilePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	scanner := bufio.NewScanner(srcFile)
	writer := bufio.NewWriter(dstFile)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var logItem IncrementLog
		if err := json.Unmarshal([]byte(line), &logItem); err != nil {
			writer.WriteString(line + "\n")
			continue
		}

		// 根据ID修改为finish
		if logItem.TaskID == taskID {
			logItem.TaskStatus = StatusFinish
		}

		bs, _ := json.Marshal(logItem)
		writer.WriteString(string(bs) + "\n")
	}

	writer.Flush()
	return os.Rename(tmpPath, logFilePath)
}
func main() {
	// 启动执行：清理finish + 分批投递任务
	err := BatchLoadPendingWithClean(func(tasks []IncrementLog) error {
		// 循环将每一批任务塞入你的WorkerPool
		for _, t := range tasks {
			WorkerPool.Push(t)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}
func WorkerTaskHandler(task IncrementLog) {
	// 业务处理逻辑...

	// 处理完成修改为finish
	_ = UpdateLogToFinish(task.TaskID)
}
