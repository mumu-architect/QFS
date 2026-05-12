package fileManager

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
)

type FileEntry struct {
	FileId     string `json:"fileId"` // 全局唯一文件ID
	FileName   string `json:"fileName"`
	FilePath   string `json:"filePath"`
	MineType   string `json:"mineType"`
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
	IsDeleted  bool   `json:"isDeleted"`
	Status     string `json:"status"` //Pending,Running,Finished
}

const rotateSize = 1 * 1024                      //每个日志文件的大小
const batchSize = 800                            // 分批读取，每批800条
const seqFileName = "cursor_meta/file_seq.dat"   //标记本地最新日志文件名
const incrementFileDir = "./file_increment_data" //增量文件目录
type IncrementFileManager struct {
	mu               sync.Mutex
	IncrementFileDir string   // 日志存储目录
	MaxFileSize      int64    // 单文件上限5KB
	FileSeq          uint64   // 文件自增序号
	SeqFilePath      string   // 序号持久化文件路径
	CurrFile         *os.File // 当前文件句柄
}

// NewIncrementFileManager 初始化日志管理器
func NewIncrementFileManager() (*IncrementFileManager, error) {
	_ = os.MkdirAll(incrementFileDir+"/cursor_meta", 0755)
	seqPath := filepath.Join(incrementFileDir, seqFileName)
	ifm := &IncrementFileManager{
		IncrementFileDir: incrementFileDir,
		MaxFileSize:      rotateSize,
		SeqFilePath:      seqPath,
	}
	// 加载文件序号
	ifm.loadFileSeq()
	// 初始化日志文件
	err := ifm.initNewIncrementFile()
	if err != nil {
		return nil, err
	}
	return ifm, nil
}

// 新建有序递增日志文件
func (ifm *IncrementFileManager) initNewIncrementFile() error {
	fileName := fmt.Sprintf("%d.log", ifm.FileSeq)
	path := filepath.Join(ifm.IncrementFileDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	ifm.CurrFile = file
	return nil
}

// WriteLog 日志写入
func (ifm *IncrementFileManager) WriteLog(entry FileEntry) bool {
	ifm.mu.Lock()
	defer ifm.mu.Unlock()
	_ = ifm.checkFileRotate()
	data, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	data = append(data, '\n')
	_, err = ifm.CurrFile.Write(data)
	if err != nil {
		return false
	}

	return true
}

// ReadAllLog TODO:分批读取数据
// ReadAllLog 按序号顺序读取所有日志（逐行解析，完整保留所有KEY）
func (ifm *IncrementFileManager) ReadAllLog() ([]FileEntry, error) {
	files, err := filepath.Glob(filepath.Join(ifm.IncrementFileDir, "*.log"))
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

	var result []FileEntry
	var batch []FileEntry

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
			var entry FileEntry
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

// 加载文件序号
func (ifm *IncrementFileManager) loadFileSeq() {
	data, err := os.ReadFile(ifm.SeqFilePath)
	if err != nil {
		ifm.FileSeq = 1
		return
	}
	num, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || num == 0 {
		ifm.FileSeq = 1
		return
	}
	ifm.FileSeq = num
}

// 检查文件超过5KB自动切割
func (ifm *IncrementFileManager) checkFileRotate() error {
	info, err := ifm.CurrFile.Stat()
	if err != nil {
		return err
	}
	if info.Size() < ifm.MaxFileSize {
		return nil
	}
	_ = ifm.CurrFile.Close()
	ifm.FileSeq++
	ifm.saveFileSeq()
	return ifm.initNewIncrementFile()
}

// 保存文件序号
func (ifm *IncrementFileManager) saveFileSeq() {
	_ = os.WriteFile(ifm.SeqFilePath, []byte(strconv.FormatUint(ifm.FileSeq, 10)), 0644)
}
