package logManager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// LogEntry 日志条目结构
type LogEntry struct {
	FlowID    string `json:"flowId"` // 全局唯一流水ID
	FileID    string `json:"fileId"` // 日志文件唯一ID
	CMD       string `json:"cmd"`    // Get|Set|HGet|HSet|HMGet|HMSet
	Key       string `json:"key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Version   uint64 `json:"version"`
	Timestamp int64  `json:"ts"`
}

// HotCursor 热点标记文件结构 存放双游标
type HotCursor struct {
	LocalBelongFileName  string `json:"localBelongFileName"` // 本地已处理最大流水ID
	LocalMaxFlowID       string `json:"localMaxFlowID"`      // 本地已处理最大流水ID
	RemoteBelongFileName string `json:"remoteBelongFileName"`
	RemoteMaxFlowID      string `json:"remoteMaxFlowID"` // 远端同步最大流水ID
}

// LogManager 日志管理器
type LogManager struct {
	mu          sync.Mutex
	LogDir      string    // 日志存储目录
	CursorPath  string    // 热点标记文件路径
	MaxFileSize int64     // 单文件上限5KB
	CurrFile    *os.File  // 当前文件句柄
	HotCursor   HotCursor // 双游标缓存
	FileSeq     uint64    // 文件自增序号
	SeqFilePath string    // 序号持久化文件路径
}

const rotateSize = 1 * 1024                     //每个日志文件的大小
const batchSize = 800                           // 分批读取，每批800条
const seqFileName = "cursor_meta/file_seq.dat"  //标记本地最新日志文件名
const cursorFile = "cursor_meta/cursor_hot.dat" //存储本地最新日志名和最新流水id，用作同步leader的数据，实现双写
const logDir = "./log_data"

// NewLogManager 初始化日志管理器
func NewLogManager() (*LogManager, error) {
	_ = os.MkdirAll(logDir+"/cursor_meta", 0755)
	seqPath := filepath.Join(logDir, seqFileName)
	cursorFilePath := filepath.Join(logDir, cursorFile)
	lm := &LogManager{
		LogDir:      logDir,
		CursorPath:  cursorFilePath,
		MaxFileSize: rotateSize,
		SeqFilePath: seqPath,
	}
	// 加载热点游标
	if err := lm.loadHotCursor(); err != nil {
		return nil, err
	}
	// 加载文件序号
	lm.loadFileSeq()
	// 初始化日志文件
	if err := lm.initNewLogFile(); err != nil {
		return nil, err
	}
	return lm, nil
}

// 加载文件序号
func (lm *LogManager) loadFileSeq() {
	data, err := os.ReadFile(lm.SeqFilePath)
	if err != nil {
		lm.FileSeq = 1
		return
	}
	num, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || num == 0 {
		lm.FileSeq = 1
		return
	}
	lm.FileSeq = num
}

// 保存文件序号
func (lm *LogManager) saveFileSeq() {
	_ = os.WriteFile(lm.SeqFilePath, []byte(strconv.FormatUint(lm.FileSeq, 10)), 0644)
}

// 新建有序递增日志文件
func (lm *LogManager) initNewLogFile() error {
	fileName := fmt.Sprintf("%d.log", lm.FileSeq)
	path := filepath.Join(lm.LogDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	lm.CurrFile = file
	return nil
}

// 检查文件超过5KB自动切割
func (lm *LogManager) checkFileRotate() error {
	info, err := lm.CurrFile.Stat()
	if err != nil {
		return err
	}
	if info.Size() < lm.MaxFileSize {
		return nil
	}
	_ = lm.CurrFile.Close()
	lm.FileSeq++
	lm.saveFileSeq()
	return lm.initNewLogFile()
}

// 加载游标文件
func (lm *LogManager) loadHotCursor() error {
	_, err := os.Stat(lm.CursorPath)
	if os.IsNotExist(err) {
		lm.HotCursor = HotCursor{}
		return nil
	}
	data, err := os.ReadFile(lm.CursorPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &lm.HotCursor)
}

// 保存游标
func (lm *LogManager) saveHotCursor() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	data, err := json.Marshal(&lm.HotCursor)
	if err != nil {
		return err
	}
	return os.WriteFile(lm.CursorPath, data, 0644)
}

// UpdateLocalCursor 更新本地游标
func (lm *LogManager) UpdateLocalCursor(BelongFileName string, flowID string) error {
	lm.mu.Lock()
	lm.HotCursor.LocalBelongFileName = BelongFileName
	lm.HotCursor.LocalMaxFlowID = flowID
	lm.mu.Unlock()
	return lm.saveHotCursor()
}

// UpdateRemoteCursor 更新远端游标
func (lm *LogManager) UpdateRemoteCursor(RemoteBelongFileName string, flowID string) error {
	lm.mu.Lock()
	lm.HotCursor.RemoteBelongFileName = RemoteBelongFileName
	lm.HotCursor.RemoteMaxFlowID = flowID
	lm.mu.Unlock()
	return lm.saveHotCursor()
}

// GetCursor 获取双游标
func (lm *LogManager) GetCursor() HotCursor {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return lm.HotCursor
}

// WriteLog 日志写入
func (lm *LogManager) WriteLog(entry LogEntry) bool {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	_ = lm.checkFileRotate()

	entry.Timestamp = time.Now().UnixMilli()
	data, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	data = append(data, '\n')
	_, err = lm.CurrFile.Write(data)
	if err != nil {
		return false
	}

	return true
}

// ReadAllLog TODO:分批读取数据
// ReadAllLog 按序号顺序读取所有日志（逐行解析，完整保留所有KEY）
func (lm *LogManager) ReadAllLog() ([]LogEntry, error) {
	files, err := filepath.Glob(filepath.Join(lm.LogDir, "*.log"))
	if err != nil {
		return nil, err
	}

	// 文件排序
	sort.Slice(files, func(i, j int) bool {
		name1 := filepath.Base(files[i])
		name2 := filepath.Base(files[j])
		num1, _ := strconv.Atoi(name1[:len(name1)-4])
		num2, _ := strconv.Atoi(name2[:len(name2)-4])
		return num1 < num2
	})

	var result []LogEntry
	var batch []LogEntry

	// 一次只打开一个文件，读完关闭
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}

			// 关键：这里能完整解析所有 KEY + VALUE
			var entry LogEntry
			err := json.Unmarshal(line, &entry)
			if err != nil {
				fmt.Println("解析错误:", err)
				continue
			}

			batch = append(batch, entry)

			// 每批800条
			if len(batch) >= batchSize {
				result = append(result, batch...)
				batch = batch[:0]
			}
		}

		// 剩余数据
		if len(batch) > 0 {
			result = append(result, batch...)
			batch = batch[:0]
		}

		file.Close()
	}

	return result, nil
}

// ReadAllLogWriteCache TODO:分批读取数据
func (lm *LogManager) ReadAllLogWriteCache() ([]LogEntry, error) {
	files, err := filepath.Glob(filepath.Join(lm.LogDir, "*.log"))
	if err != nil {
		return nil, err
	}

	// 按序号排序
	sort.Slice(files, func(i, j int) bool {
		name1 := filepath.Base(files[i])
		name2 := filepath.Base(files[j])
		num1, _ := strconv.Atoi(name1[:len(name1)-4])
		num2, _ := strconv.Atoi(name2[:len(name2)-4])
		return num1 < num2
	})

	var logs []LogEntry
	var batch []LogEntry

	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var entry LogEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}
			batch = append(batch, entry)

			// 攒够批次合并，清空批次
			if len(batch) >= batchSize {
				logs = append(logs, batch...)
				batch = batch[:0]

				//TODO:循环写入缓存

			}
		}
		// 收尾剩余数据
		if len(batch) > 0 {
			logs = append(logs, batch...)
			batch = batch[:0]
			//TODO:循环写入缓存
		}
		file.Close()
	}
	return logs, nil
}

// Close 关闭资源
func (lm *LogManager) Close() {
	if lm.CurrFile != nil {
		_ = lm.CurrFile.Close()
	}
	_ = lm.saveHotCursor()
}
