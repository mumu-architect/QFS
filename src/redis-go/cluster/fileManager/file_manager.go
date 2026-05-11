package fileManager

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ShardID int    `json:"shardID"`
	NodeID  int    `json:"id"`   // 节点唯一ID
	IP      string `json:"ip"`   // 地址（IP）
	Port    int    `json:"port"` // 地址（Port）
}

// FileManager 文件管理器
type NodeFile struct {
	ShardId       int           `json:"shardId"`
	NodeId        int           `json:"nodeId"`
	FileIp        string        `json:"fileIp"`
	FilePort      int           `json:"filePort"`
	LeaderId      int           `json:"leaderId"`
	ShardNodeInfo map[int]*Node `json:"shardNodeInfo"`
	LeaderAddr    string        `json:"leaderAddr"`
	LocalRoot     string        `json:"localRoot"`
	httpClient    *http.Client
	mu            sync.RWMutex
}

const FileDataDir = "./file_data"

// NewFileManager New 创建文件管理器
func NewNodeFile(shardId int, nodeId int, leaderId int, fileIp string, filePort int, filePeers string) *NodeFile {
	// 拼接目录 log_yyyyMMdd
	// 传入 leaderAddr 格式 ip:port，自动解析端口、自动生成 ./data-端口 本地目录
	//dir := fmt.Sprintf(FileDataDir+"/file_%d/log_%s", filePort, time.Now().Format("20060102"))

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 60 * time.Second,
			MaxConnsPerHost: 20,
		},
	}

	shardNodeInfo := stringToMapNodes(shardId, filePeers)
	localRoot := FileDataDir + fmt.Sprintf("/data_%d", filePort)
	fm := &NodeFile{
		ShardId:       shardId,
		NodeId:        nodeId,
		FileIp:        fileIp,
		FilePort:      filePort,
		LeaderId:      leaderId,
		LeaderAddr:    "",
		ShardNodeInfo: shardNodeInfo,
		LocalRoot:     localRoot,
		httpClient:    client,
	}
	// 初始化地址+目录
	fm.ChangeMasterAddr(leaderId)
	//TODO:启动文件同步Http服务
	//go fm.startFileHttpServer()
	//TODO:从机收到主机的rpc任务，实时拉去主机的文件
	go fm.StartFilePullServer()
	return fm
}

// TODO:根据leaderId获取主节点ip,port
func (nf *NodeFile) leaderIdToAddr(leaderId int) (string, int) {
	leaderIp := nf.ShardNodeInfo[leaderId].IP
	leaderPort := nf.ShardNodeInfo[leaderId].Port
	return leaderIp, leaderPort
}

// ChangeMasterAddr 主从热切换地址
// 自动解析端口、自动切换本地目录为 data-新端口
func (nf *NodeFile) ChangeMasterAddr(leaderId int) {
	nf.mu.Lock()
	defer nf.mu.Unlock()

	nf.LeaderId = leaderId
	leaderIp, leaderPort := nf.leaderIdToAddr(leaderId)
	nf.LeaderAddr = fmt.Sprintf("%s:%d", leaderIp, leaderPort)
	// 解析端口
	//if leaderPort > 0 {
	//	fm.localRoot = +strconv.Itoa(leaderPort)
	//}
}

// SyncFileByRelPath 传入相对路径 如 A/111.txt
// 自动下载到本地对应 data-端口 目录
func (nf *NodeFile) SyncFileByRelPath(relPath string) error {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return fmt.Errorf("文件相对路径不能为空")
	}

	nf.mu.RLock()
	leaderAddr := nf.LeaderAddr
	localRoot := nf.LocalRoot
	nf.mu.RUnlock()

	if leaderAddr == "" {
		return fmt.Errorf("未配置主节点地址")
	}
	if localRoot == "" {
		return fmt.Errorf("本地根目录解析失败，请检查主节点地址格式")
	}

	localFilePath := filepath.Join(localRoot, relPath)
	parentDir := filepath.Dir(localFilePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 固定 /files/ 路由前缀
	downloadURL := fmt.Sprintf("http://%s/files/%s", leaderAddr, relPath)

	resp, err := nf.httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("请求主节点文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("文件不存在或无权访问，状态码：%d", resp.StatusCode)
	}

	out, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("文件下载写入磁盘失败: %w", err)
	}
	return nil
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
