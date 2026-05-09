package logManager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster/dragonboatRaft"
	"mumu.com/redis-go/cluster/fileManager"
)

// LogEntry 日志条目结构
type LogEntry struct {
	FlowID    int64  `json:"flowId"` // 全局唯一流水ID
	CMD       string `json:"cmd"`    // Get|Set|HGet|HSet|HMGet|HMSet
	Key       string `json:"key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Version   uint64 `json:"version"`
	Timestamp int64  `json:"ts"`
}

type SyncResponse struct {
	DataMap    map[uint64][]byte `json:"dataMap"`
	HasMore    bool              `json:"hasMore"`
	DataNumber uint64            `json:"dataNumber"`
}
type NodeLog struct {
	ShardId       int           `json:"shardId"`
	NodeId        int           `json:"nodeId"`
	LeaderId      int           `json:"leaderId"`
	LogIp         string        `json:"logIp"`
	LogPort       int           `json:"logPort"`
	ShardNodeInfo map[int]*Node `json:"shardNodeInfo"`
	LocalLog      *wal.Log
	ServeMux      *http.ServeMux // 每个节点自己的HTTP路由
	HttpServer    *http.Server   // HTTP服务器实例
}

// Node 集群节点信息
type Node struct {
	ShardID int    `json:"shardID"`
	NodeID  int    `json:"id"`   // 节点唯一ID
	IP      string `json:"ip"`   // 地址（IP）
	Port    int    `json:"port"` // 地址（Port）
}

// TODO:1 << 20,1MB , 1 << 10,1kb
// var LocalLog *wal.Log
// 分批读取阈值 控制IO压力
const BatchLimit = 3  //线上800
const rollbackCnt = 6 //128M设置回滚为16

// NewNodeLog 初始化WAL 目录格式 log_日期
func NewNodeLog(shardId int, leaderId int, nodeId int, logIp string, logPort int, logPeers string) (*NodeLog, error) {
	// 拼接目录 log_yyyyMMdd
	dir := fmt.Sprintf("log_data/log_%d/log_%s", logPort, time.Now().Format("20060102"))
	opt := &wal.Options{
		SegmentSize: 1 << 10,
		LogFormat:   wal.Binary,
		NoSync:      false, //false内存数据立即落盘
	}
	wallow, err := wal.Open(dir, opt)
	if err != nil {
		return nil, err
	}
	shardNodeInfo := stringToMapNodes(shardId, logPeers)
	nodeLog := &NodeLog{
		ShardId:       shardId,
		NodeId:        nodeId,
		LogIp:         logIp,
		LogPort:       logPort,
		LeaderId:      leaderId,
		ShardNodeInfo: shardNodeInfo,
		LocalLog:      wallow,
		ServeMux:      http.NewServeMux(),
	}
	//TODO:开启8081
	go nodeLog.RegisterLogHandlers()
	go nodeLog.startLogHTTPServer()
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

// 模拟RPC 请求主节点增量日志
func (nl *NodeLog) ReqMasterLog() (*SyncResponse, error) {
	localMaxLogIndex, _ := nl.LocalLog.LastIndex()
	//TODO:同步主节点的增量数据到从节点
	leaderNode := nl.ShardNodeInfo[nl.LeaderId]
	if leaderNode == nil {
		fmt.Printf("同步日志失败：未找到leader节点，leaderId=%d，节点map长度=%d\n", nl.LeaderId, len(nl.ShardNodeInfo))
		return nil, errors.New("同步日志失败")
	}
	url := fmt.Sprintf("http://%s:%d/syncLog?lastIndex=%d", leaderNode.IP, leaderNode.Port, localMaxLogIndex)
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

// TODO:重启读取本地日志到内存前，先执行此方法，
func (nl *NodeLog) RollbackLogLastIndex() error {
	oldLast, err := nl.LocalLog.LastIndex()
	if err != nil {
		return nil

	}
	if oldLast == 0 {
		return nil
	}
	//truncIdx := int64(oldLast) - int64(rollbackCnt)
	fmt.Printf("2223=====%v===%v\n", oldLast, err)
	firstIdx, err := nl.LocalLog.FirstIndex()
	if err != nil {

		firstIdx = 0
		return err
	}
	// 🔥 核心安全计算：防止 uint64 溢出
	var truncIdx uint64
	if oldLast < rollbackCnt {
		truncIdx = firstIdx // 不够回滚，直接回滚到第一条
	} else {
		truncIdx = oldLast - uint64(rollbackCnt) // 安全减法
	}
	fmt.Printf("回滚信息：oldLast=%d, truncIdx=%d\n", oldLast, truncIdx)
	fmt.Printf("333=====%v\n", truncIdx)
	//TODO:读取truncIdx到lastIndex之间的所有数据
	for i := truncIdx; i < oldLast; i++ {
		data, err := nl.LocalLog.Read(i)
		if err != nil {
			continue
		}
		fmt.Printf("3334=====%v\n", truncIdx)
		var entry LogEntry
		err = json.Unmarshal(data, &entry)
		if err != nil {
			continue
		}
		fmt.Printf("3335=====%v\n", entry.Key)
		var fileInfo fileManager.FileInfo
		err = json.Unmarshal([]byte(entry.Value), &fileInfo)
		if err != nil {
			fmt.Printf("非文件日志，跳过：%v\n", err)
			continue // 👈 关键！跳过这条，继续下一条，不退出
		}
		fmt.Printf("3336=====%v\n", truncIdx)
		_, err = os.Stat(fileInfo.FilePath)
		if err != nil {
			//如果不是文件进入循化下一次
			continue
		}
		fmt.Printf("3337=====%v\n", truncIdx)
		//存在直接删除
		_ = os.Remove(fileInfo.FilePath)
		fmt.Printf("3338=====%v\n", truncIdx)
	}
	fmt.Printf("3339=====%v\n", truncIdx)
	_ = nl.LocalLog.TruncateBack(truncIdx)
	return nil
}

// LogIndexGenerator 生成自增id
func (nl *NodeLog) LogIndexGenerator() (uint64, error) {
	lastIndex, err := nl.LocalLog.LastIndex()
	fmt.Printf("数据落log1==：%v\n", lastIndex)
	fmt.Printf("数据落log1==：%v\n", err)
	if err != nil {
		return 0, err
	}
	lastIndex += 1
	fmt.Printf("数据落log2==：%v\n", lastIndex)
	return lastIndex, nil
}

// StartHTTPAPI  启动HTTP服务
func (nl *NodeLog) startLogHTTPServer() {
	// 监听地址（与节点地址一致）
	//TODO: listenAddr := c.LocalNode.Addr
	//listenAddr := "1" + strings.Split(c.LocalNode.Addr, ":")[1]
	//listenAddr := cl.LocalNode.Addr

	listenAddr := fmt.Sprintf(":%d", nl.LogPort)
	// 创建HTTP服务器实例
	nl.HttpServer = &http.Server{
		Addr:    listenAddr,
		Handler: nl.ServeMux,
	}
	log.Printf("HTTP服务启动，监听：%s", listenAddr)
	// 启动HTTP服务
	err := nl.HttpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

func (nl *NodeLog) RegisterLogHandlers() {
	nl.ServeMux.HandleFunc("/syncLog", func(w http.ResponseWriter, r *http.Request) {
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
			DataMap:    dataMap,
			HasMore:    hasMore,
			DataNumber: uint64(len(dataMap)),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// 字符串转map的node节点
func stringToMapNodes(shardID int, peers string) map[int]*Node {
	var shardNodeInfo = make(map[int]*Node)
	peerList := strings.Split(peers, ",")
	for _, peer := range peerList {
		parts := strings.Split(peer, "=")
		if len(parts) != 2 {
			continue
		}
		peerNodeID, _ := strconv.Atoi(parts[0])
		nodeAddr := parts[1]
		parts2 := strings.Split(nodeAddr, ":")
		if len(parts) != 2 {
			continue
		}
		peerNodeIP := parts2[0]
		peerNodePort, _ := strconv.Atoi(parts2[1])
		peerNodeInfo := &Node{
			ShardID: shardID,
			NodeID:  peerNodeID,
			IP:      peerNodeIP,
			Port:    peerNodePort,
		}
		//fmt.Printf("111====%d======%v \n", peerNodeID, peerNodeInfo)
		shardNodeInfo[peerNodeID] = peerNodeInfo
	}
	return shardNodeInfo
}
func (nl *NodeLog) DynamicallyModifyLogNodeLeaderID(shardNodeLeaderID *dragonboatRaft.NodeShardLeaderMeta) {

	if nl.ShardId == shardNodeLeaderID.ShardID {
		nl.LeaderId = shardNodeLeaderID.LeaderID
	}

}
