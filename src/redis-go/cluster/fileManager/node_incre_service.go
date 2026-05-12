package fileManager

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// StartMasterHTTPServer
// 自动规则：监听端口 8086 → 程序运行目录下 data-8086
// 监听端口 8081 → 自动 data-8081
// 路由固定前缀 /files/
func (nf *NodeFile) startIncrementFileHttpServer() {
	// 拆分出端口 例如 0.0.0.0:8086 → 8086
	listenAddr := fmt.Sprintf(":%d", nf.IncrementFilePort)
	//cl.NodeFile.FileIp
	// 自动按端口生成目录 data-端口
	bizDir := FileDataDir + "/data_" + strconv.Itoa(nf.FilePort)
	// 不存在自动创建
	err := os.MkdirAll(bizDir, 0755)
	if err != nil {
		_ = fmt.Sprintf("create file dir error:%v", err)
		return
	}
	http.HandleFunc("/IncrementPullFile", IncrementPullStreamHandler)
	log.Printf("master stream listen :%d", nf.FilePort)
	srv := &http.Server{
		Addr:         listenAddr,
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  15 * time.Second, //空闲间隔
	}
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

// IncrementPullStreamHandler 增量文件流式拉取接口
func IncrementPullStreamHandler(w http.ResponseWriter, r *http.Request) {
	// 获取请求参数
	filePath := r.URL.Query().Get("file")
	offsetStr := r.URL.Query().Get("offset")
	if filePath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	offset := int64(0)
	if offsetStr != "" {
		val, err := strconv.ParseInt(offsetStr, 10, 64)
		if err == nil {
			offset = val
		}
	}

	// 流式必须响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache,no-store,must-revalidate")
	w.Header().Set("Pragma", "no-cache")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream not support", http.StatusInternalServerError)
		return
	}

	// 打开目标文件
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	// 定位到从机需要的偏移位置
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 4096)

	// 无限循环 等待文件追加新内容
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			flusher.Flush()
		}

		// 读到末尾就休眠等待新写入，不断开连接
		if err == io.EOF {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if err != nil {
			log.Printf("read file err: %v", err)
			break
		}
	}
}
