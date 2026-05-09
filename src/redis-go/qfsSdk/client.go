package qfsSdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"time"
)

// SDK配置
type QFSConfig struct {
	ClusterEntry []string // QFS集群任意节点http地址
	Timeout      time.Duration
}

var globalCfg *QFSConfig
var httpCli = &http.Client{Timeout: 15 * time.Second}

// 初始化SDK
func InitSDK(cfg *QFSConfig) {
	globalCfg = cfg
}

// 内部找一个可用的集群入口节点
func getAliveEntry() string {
	// 遍历所有集群入口地址
	for _, addr := range globalCfg.ClusterEntry {
		// 建立 TCP 连接测试（100ms 超时）
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			// 连不上 → 跳过，试下一个
			continue
		}
		// 能连上 → 关闭连接，返回这个地址
		_ = conn.Close()
		return addr
	}
	return ""
}

// 第一步：预请求
func preUpload() (*PreUploadResp, error) {
	entry := getAliveEntry()
	url := fmt.Sprintf("http://%s/PreUpload", entry)

	resp, err := httpCli.Post(url, "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res PreUploadResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// 第二步：直传Leader上传文件
func uploadFile(leaderAddr, routeKey string, file io.Reader, fileName string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/Upload", leaderAddr)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("route_key", routeKey)
	fw, _ := writer.CreateFormFile("file", fileName)
	_, _ = io.Copy(fw, file)
	_ = writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// Upload 对外暴露：一站式上传方法
func Upload(file io.Reader, fileName string) ([]byte, error) {
	// 1. 预请求拿 key + leader地址
	preInfo, err := preUpload()
	if err != nil {
		return nil, err
	}
	// 2. 直传Leader
	return uploadFile(preInfo.LeaderAddr, preInfo.RouteKey, file, fileName)
}
