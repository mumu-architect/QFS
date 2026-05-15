package increLog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
)

// 常量定义：日志分片、批量处理、协程池等配置
const (
	originMaxLines = 20000        // 原始日志单个文件最大行数，超过自动分片
	batchOriginNum = 500          // 原始日志批量读取条数
	batchFinishNum = 2000         // 完成日志批量加载条数
	logDir         = "./task_log" // 日志存放根目录
	workerPoolSize = 8            // 任务处理协程池大小
)

// 全局变量：锁、日志文件名正则、协程池通道
var (
	globalLock  sync.Mutex                                  // 全局互斥锁，保证文件操作并发安全
	originRegex = regexp.MustCompile(`^origin_(\d+)\.log$`) // 原始日志文件名匹配正则
	workerChan  chan func()                                 // 协程池任务通道
)

// InitWorkerPool
// 作用：初始化任务处理协程池，启动固定数量的工作协程
// 说明：程序启动时调用一次，用于后台异步处理任务
func InitWorkerPool() {
	workerChan = make(chan func(), workerPoolSize)
	// 启动指定数量的工作协程，持续从通道获取任务执行
	for i := 0; i < workerPoolSize; i++ {
		go func() {
			for task := range workerChan {
				task()
			}
		}()
	}
}

// TaskLogManager 任务日志管理器
// 负责：原始日志分片写入、完成日志记录、日志重建、任务分发
type TaskLogManager struct {
	currentSeq int // 当前正在写入的原始日志分片序号
}

// NewTaskLogManager
// 作用：创建并初始化日志管理器实例
// 返回：初始化好的管理器指针
// 逻辑：自动创建日志目录，加载历史最大分片序号
func NewTaskLogManager() *TaskLogManager {
	_ = os.MkdirAll(logDir, 0755)
	mgr := &TaskLogManager{}
	mgr.currentSeq = mgr.getMaxOriginSeq()
	return mgr
}

// getMaxOriginSeq
// 作用：获取历史原始日志文件的最大序号，用于续写
// 返回：最大分片序号，无文件时返回 1
func (m *TaskLogManager) getMaxOriginSeq() int {
	files, _ := filepath.Glob(filepath.Join(logDir, "origin_*.log"))
	maxSeq := 0
	for _, f := range files {
		base := filepath.Base(f)
		match := originRegex.FindStringSubmatch(base)
		if len(match) < 2 {
			continue
		}
		seq, _ := strconv.Atoi(match[1])
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	if maxSeq == 0 {
		return 1
	}
	return maxSeq
}

// getCurrentOriginPath
// 作用：获取当前正在写入的原始日志文件完整路径
// 返回：文件路径字符串
func (m *TaskLogManager) getCurrentOriginPath() string {
	return filepath.Join(logDir, fmt.Sprintf("origin_%d.log", m.currentSeq))
}

// checkRotate
// 作用：检查当前原始日志是否需要分片（超过最大行数）
// 返回：错误信息（无错误）
// 逻辑：行数超限则序号+1，自动切换新文件
func (m *TaskLogManager) checkRotate() error {
	path := m.getCurrentOriginPath()
	cnt, _ := countFileLines(path)
	if cnt >= originMaxLines {
		m.currentSeq++
	}
	return nil
}

// AppendOrigin
// 作用：追加一行原始任务日志，自动分片
// 参数：line - 原始日志字符串
// 返回：写入错误
func (m *TaskLogManager) AppendOrigin(line string) error {
	globalLock.Lock()
	defer globalLock.Unlock()

	_ = m.checkRotate()
	path := m.getCurrentOriginPath()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}

// AppendFinish
// 作用：记录已完成的任务ID，用于去重/恢复
// 参数：taskID - 任务唯一标识
// 返回：写入错误
func (m *TaskLogManager) AppendFinish(taskID string) error {
	globalLock.Lock()
	defer globalLock.Unlock()

	finishPath := filepath.Join(logDir, "finish.log")
	f, err := os.OpenFile(finishPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(taskID + "\n")
	return err
}

// countFileLines
// 作用：统计指定文件的总行数
// 参数：path - 文件路径
// 返回：行数、错误信息
// 说明：文件不存在返回 0，无错误
func countFileLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	lines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
	}
	return lines, nil
}

// batchReadLimit
// 作用：从文件开头批量读取指定行数数据
// 参数：path - 文件路径；limit - 最大读取行数
// 返回：行内容列表、错误
func batchReadLimit(path string, limit int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var res []string
	scanner := bufio.NewScanner(f)
	for i := 0; i < limit && scanner.Scan(); i++ {
		res = append(res, scanner.Text())
	}
	return res, nil
}

// batchReadAllLines
// 作用：读取文件全部内容到字符串切片
// 参数：path - 文件路径
// 返回：全部行、错误
func batchReadAllLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var res []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		res = append(res, scanner.Text())
	}
	return res, nil
}

// getAllOriginFiles
// 作用：获取所有原始日志文件，并按序号从小到大排序
// 返回：排序后的文件路径列表、错误
func getAllOriginFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(logDir, "origin_*.log"))
	if err != nil {
		return nil, err
	}

	type seqFile struct {
		seq  int
		path string
	}
	var list []seqFile

	for _, f := range files {
		base := filepath.Base(f)
		match := originRegex.FindStringSubmatch(base)
		if len(match) < 2 {
			continue
		}
		seq, _ := strconv.Atoi(match[1])
		list = append(list, seqFile{seq: seq, path: f})
	}

	// 按分片序号升序排列
	sort.Slice(list, func(i, j int) bool {
		return list[i].seq < list[j].seq
	})

	var res []string
	for _, v := range list {
		res = append(res, v.path)
	}
	return res, nil
}

// loadFinishMap
// 作用：加载已完成任务ID到内存map，用于快速去重判断
// 返回：任务ID去重map、错误
func loadFinishMap() (map[string]bool, error) {
	finishPath := filepath.Join(logDir, "finish.log")
	if _, err := os.Stat(finishPath); os.IsNotExist(err) {
		return make(map[string]bool), nil
	}

	f, err := os.Open(finishPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := make(map[string]bool)
	scanner := bufio.NewScanner(f)

	// 批量加载提升性能
	for {
		var batch []string
		for len(batch) < batchFinishNum && scanner.Scan() {
			batch = append(batch, scanner.Text())
		}
		if len(batch) == 0 {
			break
		}
		for _, tid := range batch {
			m[tid] = true
		}
	}
	return m, nil
}

// writeLinesToFile
// 作用：将行列表覆盖写入文件（用于重建日志）
// 参数：path - 目标路径；lines - 待写入行
// 返回：错误
func writeLinesToFile(path string, lines []string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, line := range lines {
		_, _ = w.WriteString(line + "\n")
	}
	return w.Flush()
}

// RebuildOriginWithCompare
// 作用：重启时重建原始日志，清理已完成任务，删除空文件，清空finish日志
// 返回：重建错误
// 场景：服务重启后，只保留未完成任务，重新分片
func (m *TaskLogManager) RebuildOriginWithCompare() error {
	globalLock.Lock()
	defer globalLock.Unlock()

	// 加载已完成任务
	finishMap, err := loadFinishMap()
	if err != nil {
		return err
	}

	// 获取所有原始日志文件
	oldFiles, err := getAllOriginFiles()
	if err != nil {
		return err
	}

	var pendingBuffer []string
	newSeq := 1

	// 遍历旧文件，过滤已完成任务
	for _, file := range oldFiles {
		allLines, _ := batchReadAllLines(file)
		var remain []string

		for _, line := range allLines {
			taskID := getTaskIDFromLine(line)
			if !finishMap[taskID] {
				remain = append(remain, line)
			}
		}

		// 删除旧分片
		_ = os.Remove(file)
		if len(remain) == 0 {
			continue
		}

		// 重新分片写入
		pendingBuffer = append(pendingBuffer, remain...)
		for len(pendingBuffer) >= originMaxLines {
			chunk := pendingBuffer[:originMaxLines]
			pendingBuffer = pendingBuffer[originMaxLines:]
			newPath := filepath.Join(logDir, fmt.Sprintf("origin_%d.log", newSeq))
			_ = writeLinesToFile(newPath, chunk)
			newSeq++
		}
	}

	// 写入剩余不足一个分片的数据
	if len(pendingBuffer) > 0 {
		newPath := filepath.Join(logDir, fmt.Sprintf("origin_%d.log", newSeq))
		_ = writeLinesToFile(newPath, pendingBuffer)
		newSeq++
	}

	m.currentSeq = newSeq - 1

	// 清空完成日志
	finishPath := filepath.Join(logDir, "finish.log")
	_ = os.Remove(finishPath)

	return nil
}

// DispatchOriginToWorker
// 作用：将所有原始日志任务批量分发给协程池异步执行
// 返回：分发错误
// 逻辑：按文件、按批次（500条）投递，执行后标记完成
func (m *TaskLogManager) DispatchOriginToWorker() error {
	files, err := getAllOriginFiles()
	if err != nil {
		return err
	}

	// 遍历所有原始分片
	for _, filePath := range files {
		// 批量读取任务
		for {
			batchLines, err := batchReadLimit(filePath, batchOriginNum)
			if err != nil || len(batchLines) == 0 {
				break
			}

			// 提交任务到协程池
			linesCopy := batchLines
			workerChan <- func() {
				for _, line := range linesCopy {
					// 执行业务逻辑
					m.doTaskBusiness(line)
					// 标记完成
					taskID := getTaskIDFromLine(line)
					_ = m.AppendFinish(taskID)
				}
			}
		}
	}
	return nil
}

// getTaskIDFromLine
// 作用：从一行原始日志中提取任务唯一ID
// 参数：line - 原始日志行
// 返回：任务ID字符串
// 说明：需根据实际日志格式实现
func getTaskIDFromLine(line string) string {
	return line
}

// doTaskBusiness
// 作用：任务实际业务处理逻辑
// 参数：line - 原始任务日志
// 说明：用户自行实现：文件同步、传输、校验等核心逻辑
func (m *TaskLogManager) doTaskBusiness(line string) {
	// 此处填写你的业务代码
}
