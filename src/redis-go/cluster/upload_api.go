package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"

	"mumu.com/redis-go/cluster/fileManager"
)

// PreUploadRequest 预上传请求
type PreUploadRequest struct{}

// PreUploadResponse 返回给SDK
type PreUploadResponse struct {
	RouteKey   string `json:"route_key"`
	LeaderAddr string `json:"leader_addr"`
}

// 注册文件上传助手函数
func (cl *Cluster) registerRpcHandlers() {
	// 2. String - Get
	cl.ServeMux.HandleFunc("/PreUpload", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key不能为空", http.StatusBadRequest)
			return
		}
		//TODO:生成预请求routeKey,leaderAddr
		preUploadResponse := cl.HandlePreUpload()
		json.NewEncoder(w).Encode(preUploadResponse)
	})
	cl.ServeMux.HandleFunc("/Upload", func(w http.ResponseWriter, r *http.Request) {
		// 1. 从url取参数
		rootKey := r.URL.Query().Get("root_key")
		fileName := r.URL.Query().Get("filename")

		// 基础校验
		if rootKey == "" || fileName == "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 400,
				"msg":  "参数缺失",
			})
			return
		}
		//TODO:上传文件
		// 2. 核销一次性rootKey，失效直接拒绝
		_, err, filePath, fileSize := cl.HandleUpload(rootKey, fileName, r)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 400,
				"msg":  err,
			})
			return
		}

		// 6. 返回成功信息
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":     200,
			"msg":      "上传成功",
			"filePath": filePath,
			"size":     fileSize,
		})
	})
}

// HandleUpload 接收文件上传 + 核销route_key
func (cl *Cluster) HandleUpload(routeKey string, fileName string, r *http.Request) (bool, error, string, int64) {
	leaderId, _, _ := cl.GetLeaderHTTPAddr(routeKey)
	// 只有Leader处理核销
	if leaderId != cl.LocalNode.NodeID {
		return false, errors.New("no leader"), "", 0
	}
	// 一次性核销：取到即删除
	_, ok := keySyncMap.LoadAndDelete(routeKey)
	if !ok {
		return false, errors.New("the routeKey does not exist"), "", 0
	}
	// 3. 定义文件存储路径
	dir := fileManager.FileDataDir + fmt.Sprintf("/data_%d/file_%s/", cl.LocalNode.RpcPort, time.Now().Format("20060102"))
	savePath := dir + fileName
	// 确保目录存在
	_ = os.MkdirAll("./upload", 0755)
	// 4. 创建目标文件
	dstFile, err := os.Create(savePath)
	if err != nil {
		return false, errors.New("failed to create file"), "", 0
	}
	defer dstFile.Close() // 必须延迟关闭

	// 5. 核心：io.Copy 流式拷贝  r.Body -> 磁盘文件
	written, err := io.Copy(dstFile, r.Body)
	if err != nil {
		return false, errors.New("file write failed"), "", 0
	}

	return true, nil, savePath, written
}

// HandlePreUpload HTTP预上传处理函数
func (cl *Cluster) HandlePreUpload() PreUploadResponse {
	// 1. 任意节点本地生成雪花key
	routeKey := cl.GenSnowflakeKey()
	leaderId, leaderIp, _ := cl.GetLeaderHTTPAddr(routeKey)
	leaderRpcAddr := fmt.Sprintf("%s:%d", leaderIp, cl.LocalNode.RpcPort)
	// 2. 判断自身是不是Leader
	if leaderId == cl.LocalNode.NodeID {
		// 自己是主：直接本地存入map
		keySyncMap.Store(routeKey, true)
	} else {
		// 从节点：RPC远程调用Leader存key
		rpcClient, err := rpc.Dial("tcp", leaderRpcAddr)
		if err == nil {
			defer rpcClient.Close()
			args := &PutKeyArgs{RouteKey: routeKey}
			var reply EmptyReply
			_ = rpcClient.Call("KeyRPCService.PutKey", args, &reply)
		}
	}

	// 3. 直接返回 key + 真实Leader地址
	return PreUploadResponse{
		RouteKey:   routeKey,
		LeaderAddr: leaderRpcAddr,
	}
}

// 雪花ID生成 工具方法
func (cl *Cluster) GenSnowflakeKey() string {
	flakeKey := cl.SnowflakeGenerate.GetFlowID()
	// 替换成你自己的雪花算法
	return strconv.FormatInt(flakeKey, 10)
}

// 获取集群真正的Leader RPC地址 & HTTP地址
func (cl *Cluster) GetLeaderHTTPAddr(key string) (int, string, int) {
	leaderId := cl.keyToleaderID(key)
	leaderIp, leaderPort, _ := cl.LeaderIdToLeaderAddr(leaderId)
	return leaderId, leaderIp, leaderPort
}

func (cl *Cluster) GetLeaderRPCAddr() string {

	return "127.0.0.1:9000"
}

// TODO 根据key获取槽，再根据槽获取leaderID
func (cl *Cluster) keyToleaderID(key string) int {
	slot := cl.calcSlot(key)
	leaderId := cl.GetSlotToLeaderID(slot)
	return leaderId
}

// LeaderIdToLeaderAddr 字符串转map的node节点
func (cl *Cluster) LeaderIdToLeaderAddr(nodeId int) (string, int, error) {
	peerList := strings.Split(cl.AllNodeInfos, ",")
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
		//addr := fmt.Sprintf("%s:%d", peerNodeIP, peerNodePort)
		if nodeId == peerNodeID {
			return peerNodeIP, peerNodePort, nil
		}
	}
	return "", 0, errors.New("no leader found")
}
