package increLog

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// 配置参数 完全按你要求
const (
	OriginPath    = "./origin.log"
	FinishPath    = "./finish.log"
	TmpOriginPath = "./origin.tmp"

	FinishBatchSize = 2000 // finish每批加载2000个ID
	OriginBatchSize = 500  // origin每次读取500条处理
)

const (
	StatusPending = "pending"
)

// IncrementLog 原始日志结构体
type IncrementLog struct {
	TaskID     string `json:"task_id"`
	FilePath   string `json:"file_path"`
	FileMD5    string `json:"file_md5"`
	TaskStatus string `json:"task_status"`
	CreateAt   int64  `json:"create_at"`
}

// 全局文件锁 防止并发追加冲突
var fileLock sync.Mutex

// AppendOriginLog 追加一条原始pending日志
func AppendOriginLog(log IncrementLog) error {
	fileLock.Lock()
	defer fileLock.Unlock()

	f, err := os.OpenFile(OriginPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := json.Marshal(log)
	if err != nil {
		return err
	}
	_, _ = f.WriteString(string(bs) + "\n")
	return nil
}

// AppendFinishID 完成后只追加TaskID到finish.log
func AppendFinishID(taskID string) error {
	fileLock.Lock()
	defer fileLock.Unlock()

	f, err := os.OpenFile(FinishPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, _ = f.WriteString(taskID + "\n")
	return nil
}

// ClearFinishLog 清空finish文件
func ClearFinishLog() error {
	fileLock.Lock()
	defer fileLock.Unlock()
	f, err := os.OpenFile(FinishPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

// RebuildOriginFile 原子替换新origin文件
func RebuildOriginFile() error {
	fileLock.Lock()
	defer fileLock.Unlock()
	return os.Rename(TmpOriginPath, OriginPath)
}

// PushTasksToWorker 你自己的worker入队方法，这里留接口
func PushTasksToWorker(tasks []IncrementLog) {
	// 在这里实现：批量加入你的workerPool
}

// StartupCleanAndLoad 【核心重启清算方法】
// 1. 分2000批次读取finish所有ID
// 2. origin每次读500条做比对
// 3. 过滤已完成，残留写入新临时origin
// 4. 未完成批量推入worker
func StartupCleanAndLoad() error {
	// 打开临时新origin文件，用来存放剩余未完成日志
	tmpOriginFile, err := os.Create(TmpOriginPath)
	if err != nil {
		return err
	}
	defer tmpOriginFile.Close()
	tmpWriter := bufio.NewWriter(tmpOriginFile)

	// 第一步：先把所有finishID，分2000一批缓存，准备多轮比对
	finishScanner, err := os.Open(FinishPath)
	if err != nil {
		if os.IsNotExist(err) {
			// finish不存在，直接全量加载origin即可
			return loadOriginAllNoFilter(tmpWriter)
		}
		return err
	}
	defer finishScanner.Close()

	scannerFinish := bufio.NewScanner(finishScanner)

	// 用来记录所有最终需要保留的taskID（多轮finish比对后汇总）
	var remainTaskMap = make(map[string]struct{})

	// 第一轮先把origin全部读一遍，先把所有taskID暂存进来
	originAllMap, err := readOriginAllTaskID()
	if err != nil {
		return err
	}

	// 不断分批读取finish：每批2000个
	for {
		finishMap := make(map[string]struct{}, FinishBatchSize)
		count := 0

		// 读取一批2000个finishID
		for ; count < FinishBatchSize && scannerFinish.Scan(); count++ {
			id := scannerFinish.Text()
			if id == "" {
				continue
			}
			finishMap[id] = struct{}{}
		}

		// 这批没有数据了，退出循环
		if len(finishMap) == 0 {
			break
		}

		// 从总origin集合中剔除已完成的ID
		for fid := range finishMap {
			delete(originAllMap, fid)
		}
	}

	// 此时 originAllMap 剩下的就是真正未完成的ID集合

	// 再重新遍历origin.log，每次读取500条
	err = readOriginBatchBy500(originAllMap, tmpWriter)
	if err != nil {
		return err
	}

	_ = tmpWriter.Flush()

	// 原子替换新origin
	if err := RebuildOriginFile(); err != nil {
		return err
	}

	// 清空finish.log
	_ = ClearFinishLog()

	return nil
}

// readOriginAllTaskID 读取origin所有TaskID，用于前置过滤
func readOriginAllTaskID() (map[string]struct{}, error) {
	res := make(map[string]struct{})
	f, err := os.Open(OriginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var log IncrementLog
		if json.Unmarshal([]byte(line), &log) != nil {
			continue
		}
		res[log.TaskID] = struct{}{}
	}
	return res, nil
}

// readOriginBatchBy500 逐条遍历origin，每攒够500条就处理+入队
func readOriginBatchBy500(validMap map[string]struct{}, tmpWriter *bufio.Writer) error {
	f, err := os.Open(OriginPath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var batchList []IncrementLog

	for scanner.Scan() {
		line := scanner.Text()
		var log IncrementLog
		if json.Unmarshal([]byte(line), &log) != nil {
			continue
		}

		// 只保留还存在有效集合里的
		if _, ok := validMap[log.TaskID]; ok {
			batchList = append(batchList, log)

			// 写入临时新origin文件
			bs, _ := json.Marshal(log)
			_, _ = tmpWriter.WriteString(string(bs) + "\n")

			// 满500条，推入worker并清空批次
			if len(batchList) >= OriginBatchSize {
				PushTasksToWorker(batchList)
				batchList = batchList[:0]
			}
		}
	}

	// 剩余不足500条也推入
	if len(batchList) > 0 {
		PushTasksToWorker(batchList)
	}

	return nil
}

// loadOriginAllNoFilter 当finish不存在时，直接500条批量加载全部origin
func loadOriginAllNoFilter(tmpWriter *bufio.Writer) error {
	f, err := os.Open(OriginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var batchList []IncrementLog

	for scanner.Scan() {
		line := scanner.Text()
		var log IncrementLog
		if json.Unmarshal([]byte(line), &log) != nil {
			continue
		}

		batchList = append(batchList, log)
		bs, _ := json.Marshal(log)
		_, _ = tmpWriter.WriteString(string(bs) + "\n")

		if len(batchList) >= OriginBatchSize {
			PushTasksToWorker(batchList)
			batchList = batchList[:0]
		}
	}

	if len(batchList) > 0 {
		PushTasksToWorker(batchList)
	}

	_ = tmpWriter.Flush()
	return RebuildOriginFile()
}

func main() {
	// 重启清算 + 加载任务进worker
	err := StartupCleanAndLoad()
	if err != nil {
		panic(err)
	}
}
