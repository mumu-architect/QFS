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

// StartFilePullServer 启动RPC服务 所有节点都要开启
func (nf *NodeFile) StartFilePullServer() {

	//log.Fatal(http.ListenAndServe(":9988", nil))

	listenAddr := fmt.Sprintf(":%d", nf.FilePort)
	//cl.NodeFile.FileIp
	// 自动按端口生成目录 data-端口
	bizDir := FileDataDir + "/data_" + strconv.Itoa(nf.FilePort)
	// 不存在自动创建
	err := os.MkdirAll(bizDir, 0755)
	if err != nil {
		_ = fmt.Sprintf("create file dir error:%v", err)
		return
	}
	// 静态文件服务 + 规范 /files/ 前缀
	//fs := http.FileServer(http.Dir(bizDir))
	//http.Handle("/files/", http.StripPrefix("/files/", fs))
	http.HandleFunc("/SlavePullFile", SlavePullStreamHandler)
	log.Printf("master stream listen :%d", nf.FilePort)
	// 创建HTTP服务器实例
	httpServer := &http.Server{
		Addr:         listenAddr,
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  15 * time.Second, //空闲间隔
	}
	log.Printf("HTTP服务启动，监听：%s", listenAddr)
	// 启动HTTP服务
	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

// SlavePullStreamHandler 增量文件流式拉取接口
func SlavePullStreamHandler(w http.ResponseWriter, r *http.Request) {
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

//func main() {
//	http.HandleFunc("/SlavePullFile", SlavePullStreamHandler)
//	log.Println("master stream listen :9988")
//	log.Fatal(http.ListenAndServe(":9988", nil))
//}
