package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"mumu.com/redis-go/fileUpload/common"
)

// 上传状态存储（内存版，生产环境建议替换为Redis/数据库）
var uploadStatus = struct {
	sync.RWMutex
	data map[string][]int // key: 文件MD5, value: 已上传的分片索引列表
}{
	data: make(map[string][]int),
}

func main() {
	// 注册路由
	http.HandleFunc("/api/upload/status", getUploadStatus)  // 查询上传状态
	http.HandleFunc("/api/upload/chunk", uploadChunk)       // 上传分片
	http.HandleFunc("/api/upload/complete", completeUpload) // 触发合并

	// 启动服务
	fmt.Println("服务端启动：http://localhost:8080")
	// 定义处理函数
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!") // 将 "Hello, World!" 作为响应返回
	}
	// 注册处理函数，并监听端口
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("服务启动失败：%v\n", err)
	}
}

// getUploadStatus 查询文件已上传的分片
func getUploadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("仅支持GET请求"))
		return
	}

	fileMD5 := r.URL.Query().Get("file_md5")
	if fileMD5 == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("参数 file_md5 不能为空"))
		return
	}

	// 读取已上传分片（读锁，不阻塞其他查询）
	uploadStatus.RLock()
	uploadedChunks := uploadStatus.data[fileMD5]
	uploadStatus.RUnlock()

	// 响应结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":            0,
		"message":         "success",
		"file_md5":        fileMD5,
		"uploaded_chunks": uploadedChunks,
	})
}

// uploadChunk 接收客户端上传的分片
func uploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("仅支持POST请求"))
		return
	}

	// 读取请求头参数
	fileMD5 := r.Header.Get(common.FileMD5Header)
	chunkIdxStr := r.Header.Get(common.ChunkIdxHeader)
	totalChunksStr := r.Header.Get(common.TotalChunksHeader)
	if fileMD5 == "" || chunkIdxStr == "" || totalChunksStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("缺少必要请求头（X-File-MD5/X-Chunk-Index/X-Total-Chunks）"))
		return
	}

	// 解析参数
	chunkIdx, err := strconv.Atoi(chunkIdxStr)
	totalChunks, err2 := strconv.Atoi(totalChunksStr)
	if err != nil || err2 != nil || chunkIdx < 0 || chunkIdx >= totalChunks {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("分片索引或总片数不合法"))
		return
	}

	// 创建服务端分片存储目录
	serverChunkDir := filepath.Join(common.TempChunkDir, fileMD5)
	_ = os.MkdirAll(serverChunkDir, 0755)

	// 写入分片文件
	chunkPath := filepath.Join(serverChunkDir, fmt.Sprintf("chunk_%d", chunkIdx))
	// 若分片已存在，直接返回成功（幂等性）
	if _, err := os.Stat(chunkPath); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("分片已存在，无需重复上传"))
		return
	}

	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("创建分片文件失败：%v", err)))
		return
	}
	defer chunkFile.Close()

	// 读取客户端分片数据并写入
	if _, err := io.Copy(chunkFile, r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("写入分片文件失败：%v", err)))
		return
	}

	// 更新上传状态（写锁，保证线程安全）
	uploadStatus.Lock()
	defer uploadStatus.Unlock()
	uploadedChunks := uploadStatus.data[fileMD5]
	// 避免重复添加分片索引
	for _, idx := range uploadedChunks {
		if idx == chunkIdx {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("分片上传成功"))
			return
		}
	}
	uploadStatus.data[fileMD5] = append(uploadedChunks, chunkIdx)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("分片 %d 上传成功", chunkIdx)))
}

// completeUpload 客户端通知分片上传完成，触发合并
func completeUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("仅支持POST请求"))
		return
	}

	// 解析请求体
	type reqBody struct {
		FileMD5     string `json:"file_md5"`
		TotalChunks int    `json:"total_chunks"`
		FileName    string `json:"file_name"` // 客户端传入的原始文件名
	}
	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("请求体格式错误"))
		return
	}

	if req.FileMD5 == "" || req.TotalChunks <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("file_md5 和 total_chunks 为必填参数"))
		return
	}

	// 校验所有分片是否已上传
	uploadStatus.RLock()
	uploadedChunks := uploadStatus.data[req.FileMD5]
	uploadStatus.RUnlock()

	if len(uploadedChunks) != req.TotalChunks {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("分片未全部上传：已上传 %d 个，需上传 %d 个", len(uploadedChunks), req.TotalChunks)))
		return
	}

	// 合并分片
	destPath, err := common.MergeChunks(req.FileMD5, req.TotalChunks)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("文件合并失败：%v", err)))
		return
	}

	// 清除上传状态（可选）
	uploadStatus.Lock()
	delete(uploadStatus.data, req.FileMD5)
	uploadStatus.Unlock()

	// 响应结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":      0,
		"message":   "文件上传完成",
		"file_name": req.FileName,
		"save_path": destPath,
	})
}
