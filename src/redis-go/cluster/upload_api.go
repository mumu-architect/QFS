package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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
	RouteKey      string `json:"route_key"`
	LeaderAddr    string `json:"leader_addr"`
	LeaderRpcAddr string `json:"leaderRpcAddr"`
}

// 注册文件上传助手函数
func (cl *Cluster) registerRpcHandlers() {
	// 2. String - Get
	cl.ServeMux.HandleFunc("/PreUpload", func(w http.ResponseWriter, r *http.Request) {
		//key := r.URL.Query().Get("key")
		//if key == "" {
		//	http.Error(w, "key不能为空", http.StatusBadRequest)
		//	return
		//}
		//TODO:生成预请求routeKey,leaderAddr
		preUploadResponse := cl.HandlePreUpload()
		json.NewEncoder(w).Encode(preUploadResponse)
	})
	cl.ServeMux.HandleFunc("/Upload", func(w http.ResponseWriter, r *http.Request) {
		// 1. 从url取参数
		rootKey := r.URL.Query().Get("rootKey")
		fileName := r.URL.Query().Get("fileName")
		// 基础校验
		if rootKey == "" || fileName == "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 400,
				"msg":  "参数缺失",
			})
			return
		}
		// 2. 解析上传的文件（必须先执行这一步，才能拿到 handler）
		file, _, err := r.FormFile("file") // 前端上传的文件字段名：file
		if err != nil {
			http.Error(w, "获取文件失败", http.StatusBadRequest)
			return
		}
		defer file.Close()
		//// 3.  获取 Content-Type（两种方式，你任选）
		//// 方式1：从请求头获取（前端传的）
		//contentType := handler.Header.Get("Content-Type")

		// 方式2：获取【真实文件类型】，最准（推荐！）
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Seek(0, io.SeekStart) // 重置文件指针
		realContentType := http.DetectContentType(buf[:n])

		//TODO:上传文件
		// 2. 核销一次性rootKey，失效直接拒绝
		_, err, filePath, fileSize := cl.HandleUpload(rootKey, fileName, realContentType, file)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 400,
				"msg":  err.Error(),
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

// TODO:发送rpc文件下载任务到从机
func (cl *Cluster) sendTaskToSlave(sourceURL string, fileName string, routeKey string) {
	for _, nodeInfo := range cl.ShardRpcNodeInfo {
		if nodeInfo.NodeID == cl.LeaderID {
			continue
		}
		slaveRpcAddr := fmt.Sprintf("%s:%d", nodeInfo.IP, nodeInfo.Port)

		slavePort := cl.NodeFile.ShardNodeInfo[nodeInfo.NodeID].Port
		slaveDir := fileManager.FileDataDir + fmt.Sprintf("/data_%d/file_%s/", slavePort, time.Now().Format("20060102"))
		slavePath := slaveDir + fileName
		leaderNodeFileAddr := fmt.Sprintf("http://%s:%d", cl.NodeFile.ShardNodeInfo[cl.LeaderID].IP, cl.NodeFile.ShardNodeInfo[cl.LeaderID].Port)
		rpcClient, err := rpc.Dial("tcp", slaveRpcAddr)
		fmt.Printf("==slaveRpcAddr:%s\n", slaveRpcAddr)
		if err == nil {
			defer rpcClient.Close()
			task := &fileManager.SyncTask{
				TaskID:    fmt.Sprintf("task_%s", routeKey),
				LeaderURL: leaderNodeFileAddr,
				SourceURL: sourceURL,
				LocalPath: slavePath,
			}
			var reply fileManager.EmptyReply
			callErr := rpcClient.Call("SyncRPCService.ReceiveSyncTask", task, &reply)
			if callErr != nil {
				fmt.Printf("RPC调用失败：%v\n", callErr)
			} else {
				fmt.Printf("RPC调用成功，写入routeKey：%s\n", routeKey)
			}
		}
	}
}

// HandleUpload 接收文件上传 + 核销route_key
func (cl *Cluster) HandleUpload(routeKey string, fileName string, realContentType string, file multipart.File) (bool, error, string, int64) {
	leaderId, _, _, _ := cl.GetLeaderHTTPAddr(routeKey)
	// 只有Leader处理核销
	fmt.Printf("leaderId:%d ===cl.LocalNode.NodeID:%d \n", leaderId, cl.LocalNode.NodeID)
	if leaderId != cl.LocalNode.NodeID {
		return false, errors.New("no leader"), "", 0
	}
	// 一次性核销：取到即删除
	fmt.Printf("=====接收routeKey：%s\n", routeKey)
	_, ok := keySyncMap.LoadAndDelete(routeKey) // 成功才删除
	fmt.Println("LoadAndDelete:", ok)

	if !ok {
		return false, errors.New("the routeKey does not exist"), "", 0
	}

	// 3. 定义文件存储路径
	dir := fileManager.FileDataDir + fmt.Sprintf("/data_%d/file_%s/", cl.NodeFile.FilePort, time.Now().Format("20060102"))
	savePath := dir + fileName
	// 确保目录存在
	_ = os.MkdirAll(dir, 0755)
	// 4. 创建目标文件
	dstFile, err := os.Create(savePath)
	if err != nil {
		return false, errors.New("failed to create file"), "", 0
	}
	defer dstFile.Close() // 必须延迟关闭
	//TODO:发送rpc通知到当前shard的所有从机,rpc地址
	go cl.sendTaskToSlave(savePath, fileName, routeKey)

	// 5. 核心：io.Copy 流式拷贝  r.Body -> 磁盘文件
	fileSize, err := io.Copy(dstFile, file)
	if err != nil {
		return false, errors.New("file write failed"), "", 0
	}
	//TODO:上传文件信息写入内存，并持久化到本地
	//fileSize := r.ContentLength
	routeKeyInt, err := strconv.ParseInt(routeKey, 10, 64)
	fileInfo := &fileManager.FileInfo{
		FIleID:     routeKeyInt,
		FileName:   fileName,
		FilePath:   savePath,
		FileSize:   fileSize,
		MineType:   realContentType,
		CreateTime: time.Now().UnixMilli(),
		UpdateTime: time.Now().UnixMilli(),
		IsDeleted:  false,
	}
	fileInfoStr, _ := json.Marshal(fileInfo)
	key := fileManager.GenerateFileCacheKey(routeKeyInt)
	fmt.Printf("fileManager.FileInfo : %s\n", string(fileInfoStr))
	go cl.SetToLog(key, string(fileInfoStr))
	return true, nil, savePath, fileSize
}

// HandlePreUpload HTTP预上传处理函数
func (cl *Cluster) HandlePreUpload() PreUploadResponse {
	// 1. 任意节点本地生成雪花key
	routeKey := cl.GenSnowflakeKey()
	leaderId, leaderIp, leaderPort, rpcLeaderPort := cl.GetLeaderHTTPAddr(routeKey)
	leaderRpcAddr := fmt.Sprintf("%s:%d", leaderIp, rpcLeaderPort)
	leaderAddr := fmt.Sprintf("%s:%d", leaderIp, leaderPort)
	// 2. 判断自身是不是Leader
	if leaderId == cl.LocalNode.NodeID {
		// 自己是主：直接本地存入map
		keySyncMap.Store(routeKey, true)
	} else {
		fmt.Printf("leaderRpcAddr:%s", leaderRpcAddr)
		// 从节点：RPC远程调用Leader存key
		rpcClient, err := rpc.Dial("tcp", leaderRpcAddr)
		if err == nil {
			defer rpcClient.Close()
			args := &PutKeyArgs{RouteKey: routeKey}
			var reply EmptyReply
			callErr := rpcClient.Call("KeyRPCService.PutKey", args, &reply)
			if callErr != nil {
				fmt.Printf("RPC调用失败：%v\n", callErr)
			} else {
				fmt.Printf("RPC调用成功，写入routeKey：%s\n", routeKey)
			}
		}
	}

	// 3. 直接返回 key + 真实Leader地址
	return PreUploadResponse{
		RouteKey:      routeKey,
		LeaderAddr:    leaderAddr,
		LeaderRpcAddr: leaderRpcAddr,
	}
}

// 雪花ID生成 工具方法
func (cl *Cluster) GenSnowflakeKey() string {
	flakeKey := cl.SnowflakeGenerate.GetFlowID()
	// 替换成你自己的雪花算法
	return strconv.FormatInt(flakeKey, 10)
}

// 获取集群真正的Leader RPC地址 & HTTP地址
func (cl *Cluster) GetLeaderHTTPAddr(key string) (int, string, int, int) {
	leaderId := cl.keyToleaderID(key)
	leaderIp, leaderPort, _ := cl.LeaderIdToLeaderAddr(leaderId)
	_, rpcLeaderPort, _ := cl.LeaderIdToRpcLeaderAddr(leaderId)
	return leaderId, leaderIp, leaderPort, rpcLeaderPort
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

// LeaderIdToRpcLeaderAddr 获取rpcleaderAddr
func (cl *Cluster) LeaderIdToRpcLeaderAddr(nodeId int) (string, int, error) {
	peerList := strings.Split(cl.RpcNodeInfo, ",")
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
