package logManager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster"
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

type SyncResponse struct {
	DataMap map[uint64][]byte `json:"dataMap"`
	HasMore bool              `json:"hasMore"`
}
type NodeLog struct {
	LogPort  int `json:"logPort"`
	LocalLog *wal.Log
}

// TODO:1 << 20,1MB , 1 << 10,1kb
//var LocalLog *wal.Log

// 分批读取阈值 控制IO压力
const batchSize = 3  //线上500
const BatchLimit = 3 //线上800
// InitWAL 初始化WAL 目录格式 log_日期
func NewNodeLog(logPort int) (*NodeLog, error) {
	// 拼接目录 log_yyyyMMdd
	dir := fmt.Sprintf("log_data/log_%s/log_%s", logPort, time.Now().Format("20060102"))
	opt := &wal.Options{
		SegmentSize: 1 << 10,
		LogFormat:   wal.Binary,
		NoSync:      false, //false内存数据立即落盘
	}
	log, err := wal.Open(dir, opt)
	if err != nil {
		return nil, err
	}
	nodeLog := &NodeLog{
		LogPort:  logPort,
		LocalLog: log,
	}
	return nodeLog, nil
}

// WriteLocalLog 写入本地WAL日志方法
// index:全局自增偏移 data:业务数据
func (nl *NodeLog) WriteLocalLog(index uint64, data []byte) error {
	if nl.LocalLog == nil {
		return errors.New("wal not init")
	}
	return nl.LocalLog.Write(index, data)
}

// 写入内存 业务方法
func writeToMemory(cl *cluster.Cluster, data []byte) (bool, error) {
	var entry LogEntry
	err := json.Unmarshal(data, &entry)
	if err != nil {
		return false, err
	}
	if entry.CMD == "Set" {
		cl.SetStringData(entry.Key, entry.Value)
	} else if entry.CMD == "HSet" || entry.CMD == "HMSet" {
		cl.SetHashData(entry.Key, entry.Field, entry.Value)
	}
	return true, nil
}

// 清空内存
func clearMemory() {}

// 模拟RPC 请求主节点增量日志
func (nl *NodeLog) reqMasterLog(cl *cluster.Cluster, addr string, startIdx uint64) (*SyncResponse, error) {
	localMaxLogIndex, _ := nl.LocalLog.LastIndex()
	url := fmt.Sprintf("http://127.0.0.1:8081/syncLog?lastIndex=%d", localMaxLogIndex)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res *SyncResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// PullLeaderLogOnce  从节点拉取主节点增量日志
func (nl *NodeLog) PullLeaderLogOnce(cl *cluster.Cluster, leaderAddr string) error {
	localMaxIdx, err := nl.LocalLog.LastIndex()
	if err != nil {
		return err
	}
	logMap, err := nl.reqMasterLog(cl, leaderAddr, localMaxIdx)
	if err != nil {
		return err
	}

	for idx, data := range logMap.DataMap {
		err := nl.LocalLog.Write(idx, data)
		if errors.Is(err, wal.ErrOutOfOrder) {
			continue
		}
		_, err = writeToMemory(cl, data)
		if err != nil {
			return err
		}
	}
	if logMap.HasMore {
		go func() {
			_ = nl.PullLeaderLogOnce(cl, leaderAddr)
		}()
	}
	return nil
}

// PullSyncLoop 循环拉去leader的最新log数据
func (nl *NodeLog) PullSyncLoop(cl *cluster.Cluster, leaderAddr string) {
	tt := time.NewTicker(500 * time.Millisecond)
	for range tt.C {
		if err := nl.PullLeaderLogOnce(cl, leaderAddr); err != nil {
			fmt.Printf("pull leader log error:%v \n", err)
		}
	}
}

// RestartBatchLoad 重启分批加载 低IO高性能
func (nl *NodeLog) RestartBatchLoad(cl *cluster.Cluster) error {
	clearMemory()

	firstIdx, err := nl.LocalLog.FirstIndex()
	if err != nil {
		return err
	}
	lastIdx, err := nl.LocalLog.LastIndex()
	if err != nil {
		return err
	}

	if firstIdx > lastIdx {
		return nil
	}

	start := firstIdx
	for start <= lastIdx {
		end := start + batchSize - 1
		if end > lastIdx {
			end = lastIdx
		}

		for idx := start; idx <= end; idx++ {
			data, err := nl.LocalLog.Read(idx)
			if err != nil {
				if err == wal.ErrNotFound {
					break
				}
				return fmt.Errorf("read idx:%d err:%w", idx, err)
			}
			writeToMemory(cl, data)
		}
		start = end + 1
	}
	return nil
}

// startHTTPAPI 启动HTTP服务
func StartHTTPAPI(cl *cluster.Cluster) {
	// 监听地址（与节点地址一致）
	//TODO: listenAddr := c.LocalNode.Addr
	//listenAddr := "1" + strings.Split(c.LocalNode.Addr, ":")[1]
	//listenAddr := cl.LocalNode.Addr

	listenAddr := ":8081"
	// 创建HTTP服务器实例
	cl.HttpServer = &http.Server{
		Addr:    listenAddr,
		Handler: cl.ServeMux,
	}
	log.Printf("HTTP服务启动，监听：%s", listenAddr)
	// 启动HTTP服务
	err := cl.HttpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

func (nl *NodeLog) registerDataHandlers(cl *cluster.Cluster) {
	cl.ServeMux.HandleFunc("/syncLog", func(w http.ResponseWriter, r *http.Request) {

		key := r.URL.Query().Get("lastIndex")
		if key == "" {
			http.Error(w, "key不能为空", http.StatusBadRequest)
			return
		}
		//获取本地节点的log的lastIndex

		lastIndex, _ := strconv.ParseUint(key, 10, 64)
		// 查出所有大于lastIdx的日志
		//firstIdx, _ := LocalLog.FirstIndex()
		maxIdx, _ := nl.LocalLog.LastIndex()
		// 无增量数据
		if lastIndex >= maxIdx {
			_ = json.NewEncoder(w).Encode(SyncResponse{HasMore: false})
			return
		}
		start := lastIndex + 1
		end := start + BatchLimit - 1
		// 边界截断
		if end > maxIdx {
			end = maxIdx
		}
		dataMap := make(map[uint64][]byte)
		// 批量读取区间数据
		for idx := start; idx <= end; idx++ {
			data, err := nl.LocalLog.Read(idx)
			if err != nil {
				continue
			}
			dataMap[idx] = data
		}
		// 判断是否还有下一批
		hasMore := end < maxIdx
		resp := SyncResponse{
			DataMap: dataMap,
			HasMore: hasMore,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}
