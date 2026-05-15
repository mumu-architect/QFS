package fileManager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"sync"
)

type FileEntry struct {
	FileId     string `json:"fileId"` // 全局唯一文件ID
	FileName   string `json:"fileName"`
	FilePath   string `json:"filePath"`
	MineType   string `json:"mineType"`
	FileSize   int64  `json:"fileSize"`
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
	IsDeleted  bool   `json:"isDeleted"`
	Status     string `json:"status"` //Pending,Running,Finished
}

// const rotateSize = 1 * 1024 //每个日志文件的大小
// const batchSize = 800                            // 分批读取，每批800条
const seqFileName = "cursor_meta/file_seq.dat"   //标记本地最新日志文件名
const incrementFileDir = "./file_increment_data" //增量文件目录
type IncrementFileManager struct {
	mu                sync.Mutex
	IncrementFilePort int
	IncrementFileDir  string // 日志存储目录
	//MaxFileSize      int64  // 单文件上限5KB
	OriginMaxLines int
	BatchFinishNum int
	OriginRegex    *regexp.Regexp
	BatchOriginNum int      // 原始日志批量读取条数
	FileSeq        int      // 文件自增序号
	SeqFilePath    string   // 序号持久化文件路径
	CurrFile       *os.File // 当前文件句柄
}

// NewIncrementFileManager 初始化日志管理器
func NewIncrementFileManager(incrementFilePort int) (*IncrementFileManager, error) {
	incrementFileDataDir := incrementFileDir + fmt.Sprintf("/data_%d", incrementFilePort)
	_ = os.MkdirAll(incrementFileDataDir+"/cursor_meta", 0755)
	seqPath := filepath.Join(incrementFileDataDir, seqFileName)
	ifm := &IncrementFileManager{
		IncrementFilePort: incrementFilePort,
		IncrementFileDir:  incrementFileDataDir,
		//MaxFileSize:      rotateSize,
		OriginMaxLines: 20000,
		BatchFinishNum: 2000, // 完成日志批量加载条数
		OriginRegex:    regexp.MustCompile(`^origin_(\d+)\.log$`),
		BatchOriginNum: 300,
		SeqFilePath:    seqPath,
	}

	//TODO:清理已经完成的的增量日志,这个不是用go程
	//if ifm.CurrFile != nil {
	//	ifm.CurrFile.Close()
	//	ifm.CurrFile = nil // 必须置空！！！
	//}
	err := ifm.RebuildOriginWithCompare()
	if err != nil {
		fmt.Printf("RebuildOriginWithCompare err: %v\n", err)
		return nil, err
	}
	// 加载文件序号
	ifm.loadFileSeq()
	// 初始化日志文件
	err = ifm.initNewIncrementFile()
	if err != nil {
		return nil, err
	}
	return ifm, nil
}

// 新建有序递增日志文件
func (ifm *IncrementFileManager) initNewIncrementFile() error {
	fileName := fmt.Sprintf("origin_%d.log", ifm.FileSeq)
	path := filepath.Join(ifm.IncrementFileDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	ifm.CurrFile = file
	return nil
}

// WriteLog 日志写入
func (ifm *IncrementFileManager) WriteLog(entry *FileEntry) bool {
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

//// ReadAllLog TODO:分批读取数据
//// ReadAllLog 按序号顺序读取所有日志（逐行解析，完整保留所有KEY）
//func (ifm *IncrementFileManager) ReadAllLog() ([]FileEntry, error) {
//	files, err := filepath.Glob(filepath.Join(ifm.IncrementFileDir, "*.log"))
//	if err != nil {
//		return nil, err
//	}
//
//	// 文件排序
//	sort.Slice(files, func(i, j int) bool {
//		name1 := filepath.Base(files[i])
//		name2 := filepath.Base(files[j])
//		num1, _ := strconv.Atoi(name1[:len(name1)-4])
//		num2, _ := strconv.Atoi(name2[:len(name2)-4])
//		return num1 < num2
//	})
//
//	var result []FileEntry
//	var batch []FileEntry
//
//	// 一次只打开一个文件，读完关闭
//	for _, filePath := range files {
//		file, err := os.Open(filePath)
//		if err != nil {
//			continue
//		}
//
//		scanner := bufio.NewScanner(file)
//		buf := make([]byte, 1024*1024)
//		scanner.Buffer(buf, 1024*1024)
//
//		for scanner.Scan() {
//			line := bytes.TrimSpace(scanner.Bytes())
//			if len(line) == 0 {
//				continue
//			}
//
//			// 关键：这里能完整解析所有 KEY + VALUE
//			var entry FileEntry
//			err := json.Unmarshal(line, &entry)
//			if err != nil {
//				fmt.Println("解析错误:", err)
//				continue
//			}
//
//			batch = append(batch, entry)
//
//			// 每批800条
//			if len(batch) >= batchSize {
//				result = append(result, batch...)
//				batch = batch[:0]
//			}
//		}
//
//		// 剩余数据
//		if len(batch) > 0 {
//			result = append(result, batch...)
//			batch = batch[:0]
//		}
//
//		file.Close()
//	}
//
//	return result, nil
//}

// 加载文件序号
func (ifm *IncrementFileManager) loadFileSeq() {
	data, err := os.ReadFile(ifm.SeqFilePath)
	if err != nil {
		ifm.FileSeq = 1
		return
	}
	num, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil || num == 0 {
		ifm.FileSeq = 1
		return
	}
	ifm.FileSeq = int(num)
}

// // 检查文件超过5KB自动切割
//
//	func (ifm *IncrementFileManager) checkFileRotate() error {
//		info, err := ifm.CurrFile.Stat()
//		if err != nil {
//			return err
//		}
//		if info.Size() < ifm.MaxFileSize {
//			return nil
//		}
//		_ = ifm.CurrFile.Close()
//		ifm.FileSeq++
//		ifm.saveFileSeq()
//		return ifm.initNewIncrementFile()
//	}
//

// RebuildOriginWithCompare
// 作用：重启时重建原始日志，清理已完成任务，删除空文件，清空finish日志
// 返回：重建错误
// 场景：服务重启后，只保留未完成任务，重新分片
func (ifm *IncrementFileManager) RebuildOriginWithCompare() error {
	ifm.mu.Lock()
	defer ifm.mu.Unlock()

	// 加载已完成任务
	finishMap, err := ifm.loadFinishMap()
	fmt.Printf("finishMap:%v\n", finishMap)
	if err != nil {
		return err
	}

	// 获取所有原始日志文件
	oldFiles, err := ifm.getAllOriginFiles()
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
			fmt.Printf("taskID getTaskIDFromLine:%v\n", taskID)
			if !finishMap[taskID] {

				remain = append(remain, line)
			}
		}

		// 删除旧分片
		allLines = nil
		runtime.GC()
		err = os.Remove(file)
		if err != nil {
			fmt.Printf("删除文件失败:%v\n", file)
			return err
		}
		if len(remain) == 0 {
			continue
		}

		// 重新分片写入
		pendingBuffer = append(pendingBuffer, remain...)
		for len(pendingBuffer) >= ifm.OriginMaxLines {
			chunk := pendingBuffer[:ifm.OriginMaxLines]
			pendingBuffer = pendingBuffer[ifm.OriginMaxLines:]
			newPath := filepath.Join(ifm.IncrementFileDir, fmt.Sprintf("origin_%d.log", newSeq))
			_ = writeLinesToFile(newPath, chunk)
			newSeq++
		}
	}

	// 写入剩余不足一个分片的数据
	if len(pendingBuffer) > 0 {
		newPath := filepath.Join(ifm.IncrementFileDir, fmt.Sprintf("origin_%d.log", newSeq))
		_ = writeLinesToFile(newPath, pendingBuffer)
		newSeq++
	}

	ifm.FileSeq = newSeq - 1

	// 清空完成日志
	finishPath := filepath.Join(ifm.IncrementFileDir, "finish.log")
	_ = os.Remove(finishPath)

	return nil
}

// AppendFinish
// 作用：记录已完成的任务ID，用于去重/恢复
// 参数：taskID - 任务唯一标识
// 返回：写入错误
func (ifm *IncrementFileManager) AppendFinish(taskID string) error {
	ifm.mu.Lock()
	defer ifm.mu.Unlock()

	finishPath := filepath.Join(ifm.IncrementFileDir, "finish.log")
	f, err := os.OpenFile(finishPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(taskID + "\n")
	return err
}

// batchReadLimit
// 作用：从文件开头批量读取指定行数数据
// 参数：path - 文件路径；limit - 最大读取行数
// 返回：行内容列表、错误
//func batchReadLimit(path string, limit int) ([]string, error) {
//	f, err := os.Open(path)
//	if err != nil {
//		return nil, err
//	}
//	defer f.Close()
//
//	var res []string
//	scanner := bufio.NewScanner(f)
//	for i := 0; i < limit && scanner.Scan(); i++ {
//		res = append(res, scanner.Text())
//	}
//	return res, nil
//}

// batchReadLimit
// 作用：从文件开头批量读取指定行数数据，文件读完就停止
// 参数：path - 文件路径；limit - 最大读取行数
// 返回：行内容列表、是否已经读到文件末尾、错误
//
//	func batchReadLimit(path string, limit int) ([]string, bool, error) {
//		f, err := os.Open(path)
//		if err != nil {
//			return nil, false, err
//		}
//		defer f.Close()
//
//		var lines []string
//		scanner := bufio.NewScanner(f)
//
//		// 核心：最多读 limit 行，文件读完自动退出
//		for i := 0; i < limit; i++ {
//			// 读不到内容 = 文件结束
//			if !scanner.Scan() {
//				break
//			}
//			lines = append(lines, scanner.Text())
//		}
//
//		// 返回是否读到文件末尾
//		isEOF := !scanner.Scan() && scanner.Err() == nil
//
//		// 返回读取到的行 + 是否读完 + 错误
//		return lines, isEOF, scanner.Err()
//	}
//
// offset: 当前读取字节偏移
// 返回：读到的行、新偏移、是否EOF、错误
func batchReadLimitWithOffset(path string, limit int, offset int64) ([]string, int64, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, false, err
	}
	defer f.Close()

	// 关键：跳到上一次读取的偏移位置
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, offset, false, err
	}

	scanner := bufio.NewScanner(f)
	var lines []string

	for i := 0; i < limit && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, offset, false, err
	}

	// 获取当前读完后的最新偏移
	newOffset, _ := f.Seek(0, io.SeekCurrent)
	eof := !scanner.Scan()

	return lines, newOffset, eof, nil
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

// getTaskIDFromLine
// 作用：从一行原始日志中提取任务唯一ID
// 参数：line - 原始日志行
// 返回：任务ID字符串
// 说明：需根据实际日志格式实现
func getTaskIDFromLine(line string) string {
	var incrementSyncTask IncrementSyncTask
	err := json.Unmarshal([]byte(line), &incrementSyncTask)
	if err != nil {
		fmt.Printf("getTastIDFromLine err:%s\n", err)
		return ""
	}
	return incrementSyncTask.TaskID
}

// batchReadAllLines
// 作用：读取文件全部内容到字符串切片
// 参数：path - 文件路径
// 返回：全部行、错误
//
//	func batchReadAllLines(path string) ([]string, error) {
//		f, err := os.Open(path)
//		if err != nil {
//			return nil, err
//		}
//		defer f.Close()
//
//		var res []string
//		scanner := bufio.NewScanner(f)
//		for scanner.Scan() {
//			res = append(res, scanner.Text())
//		}
//		return res, nil
//	}
//
// batchReadAllLines
// 作用：读取文件全部内容到字符串切片
// 参数：path - 文件路径
// 返回：全部行、错误
//func batchReadAllLines(path string) ([]string, error) {
//	f, err := os.Open(path)
//	if err != nil {
//		return nil, err
//	}
//
//	var res []string
//	scanner := bufio.NewScanner(f)
//	for scanner.Scan() {
//		res = append(res, scanner.Text())
//	}
//	// 必须校验扫描错误，否则句柄异常残留
//	if err := scanner.Err(); err != nil {
//		_ = f.Close()
//		return nil, err
//	}
//
//	// 手动立刻关闭，不依赖defer延迟关闭
//	_ = f.Close()
//	// 强制回收文件句柄，消灭Windows延迟占用
//	runtime.GC()
//
//	return res, nil
//}

func batchReadAllLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// 立即关闭，不等待 defer
	defer func() {
		f.Close()
	}()

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// 必须检查错误，否则句柄会卡住
	if err := scanner.Err(); err != nil {
		return lines, err
	}

	// 关键：把文件对象主动设为 nil，强制释放句柄
	f = nil

	// 强制GC回收，Windows 必须加
	runtime.GC()

	return lines, nil
}

// getAllOriginFiles
// 作用：获取所有原始日志文件，并按序号从小到大排序
// 返回：排序后的文件路径列表、错误
func (ifm *IncrementFileManager) getAllOriginFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(ifm.IncrementFileDir, "origin_*.log"))
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
		match := ifm.OriginRegex.FindStringSubmatch(base)
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
func (ifm *IncrementFileManager) loadFinishMap() (map[string]bool, error) {
	finishPath := filepath.Join(ifm.IncrementFileDir, "finish.log")
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
		for len(batch) < ifm.BatchFinishNum && scanner.Scan() {
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

// checkRotate
// 作用：检查当前原始日志是否需要分片（超过最大行数）
// 返回：错误信息（无错误）
// 逻辑：行数超限则序号+1，自动切换新文件
func (ifm *IncrementFileManager) checkFileRotate() error {
	path := ifm.getCurrentOriginPath()
	cnt, _ := countFileLines(path)
	if cnt >= ifm.OriginMaxLines {
		ifm.FileSeq++
		ifm.saveFileSeq()
	}
	return nil
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

// getCurrentOriginPath
// 作用：获取当前正在写入的原始日志文件完整路径
// 返回：文件路径字符串
func (ifm *IncrementFileManager) getCurrentOriginPath() string {
	return filepath.Join(ifm.IncrementFileDir, fmt.Sprintf("origin_%d.log", ifm.FileSeq))
}

// 保存文件序号
func (ifm *IncrementFileManager) saveFileSeq() {
	_ = os.WriteFile(ifm.SeqFilePath, []byte(strconv.FormatInt(int64(ifm.FileSeq), 10)), 0644)
}
