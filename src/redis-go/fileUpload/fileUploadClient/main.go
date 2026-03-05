package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mumu.com/redis-go/fileUpload/common"
)

// 客户端配置
const (
	ServerAddr     = "http://localhost:8080" // 服务端地址
	MaxConcurrency = 5                       // 最大并发上传协程数
)

func main() {
	// 待上传文件路径（替换为你的大文件路径）
	//filePath := "./large_file.test"
	// 关键：打印当前工作目录（CWD）
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("获取当前目录失败：", err)
		return
	}
	fmt.Println("当前工作目录：", cwd) // 重点看这行输出！

	filePath := "./fileUpload/fileUploadClient/22.pdf"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("错误：文件 %s 不存在\n", filePath)
		return
	}

	// 1. 拆分文件为分片
	fmt.Println("开始拆分文件...")
	chunkPaths, fileMD5, err := common.SplitFile(filePath)
	if err != nil {
		fmt.Printf("文件拆分失败：%v\n", err)
		return
	}
	totalChunks := len(chunkPaths)
	fmt.Printf("文件拆分完成：共 %d 个分片，文件MD5：%s\n", totalChunks, fileMD5)

	// 2. 查询已上传分片（断点续传核心）
	fmt.Println("查询已上传分片状态...")
	uploadedChunks, err := queryUploadedChunks(fileMD5)
	if err != nil {
		fmt.Printf("查询上传状态失败：%v\n", err)
		return
	}
	fmt.Printf("已上传分片索引：%v\n", uploadedChunks)

	// 3. 筛选未上传的分片
	var pendingChunks []int
	for i := 0; i < totalChunks; i++ {
		exists := false
		for _, idx := range uploadedChunks {
			if idx == i {
				exists = true
				break
			}
		}
		if !exists {
			pendingChunks = append(pendingChunks, i)
		}
	}
	fmt.Printf("待上传分片索引：%v\n", pendingChunks)
	if len(pendingChunks) == 0 {
		fmt.Println("所有分片已上传，直接通知服务端合并...")
		if err := notifyComplete(fileMD5, totalChunks, filepath.Base(filePath)); err != nil {
			fmt.Printf("通知合并失败：%v\n", err)
		}
		return
	}

	// 4. 并发上传未完成的分片
	fmt.Println("开始并发上传分片...")
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, MaxConcurrency) // 控制并发数
	errCh := make(chan error, len(pendingChunks))    // 接收上传错误

	for _, idx := range pendingChunks {
		wg.Add(1)
		semaphore <- struct{}{} // 占用并发名额

		go func(chunkIdx int) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放并发名额

			chunkPath := chunkPaths[chunkIdx]
			if err := uploadSingleChunk(chunkPath, fileMD5, chunkIdx, totalChunks); err != nil {
				fmt.Printf("分片 %d 上传失败：%v\n", chunkIdx, err)
				errCh <- err
				return
			}
			fmt.Printf("分片 %d 上传成功\n", chunkIdx)
		}(idx)
	}

	// 等待所有上传协程完成
	wg.Wait()
	close(errCh)
	close(semaphore)

	// 检查是否有上传失败
	if len(errCh) > 0 {
		fmt.Printf("上传失败：共 %d 个分片上传失败\n", len(errCh))
		return
	}

	// 5. 通知服务端合并文件
	fmt.Println("所有分片上传完成，通知服务端合并...")
	if err := notifyComplete(fileMD5, totalChunks, filepath.Base(filePath)); err != nil {
		fmt.Printf("文件合并失败：%v\n", err)
		return
	}

	fmt.Println("文件上传完成！")
}

// queryUploadedChunks 向服务端查询已上传的分片
func queryUploadedChunks(fileMD5 string) ([]int, error) {
	url := fmt.Sprintf("%s/api/upload/status?file_md5=%s", ServerAddr, fileMD5)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求服务端失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务端返回错误状态码：%d", resp.StatusCode)
	}

	type respData struct {
		Code           int   `json:"code"`
		UploadedChunks []int `json:"uploaded_chunks"`
	}
	var data respData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析响应失败：%v", err)
	}

	return data.UploadedChunks, nil
}

// uploadSingleChunk 上传单个分片到服务端
func uploadSingleChunk(chunkPath, fileMD5 string, chunkIdx, totalChunks int) error {
	// 读取分片数据
	chunkData, err := os.ReadFile(chunkPath)
	if err != nil {
		return fmt.Errorf("读取分片失败：%v", err)
	}

	// 构建请求
	url := fmt.Sprintf("%s/api/upload/chunk", ServerAddr)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(chunkData))
	if err != nil {
		return fmt.Errorf("构建请求失败：%v", err)
	}

	// 设置请求头
	req.Header.Set(common.FileMD5Header, fileMD5)
	req.Header.Set(common.ChunkIdxHeader, fmt.Sprintf("%d", chunkIdx))
	req.Header.Set(common.TotalChunksHeader, fmt.Sprintf("%d", totalChunks))
	req.Header.Set("Content-Type", "application/octet-stream")

	// 发送请求（设置超时时间）
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("服务端返回错误：%d，详情：%s", resp.StatusCode, string(body))
	}

	return nil
}

// notifyComplete 通知服务端分片上传完成，触发合并
func notifyComplete(fileMD5 string, totalChunks int, fileName string) error {
	url := fmt.Sprintf("%s/api/upload/complete", ServerAddr)
	reqBody, err := json.Marshal(map[string]interface{}{
		"file_md5":     fileMD5,
		"total_chunks": totalChunks,
		"file_name":    fileName,
	})
	if err != nil {
		return fmt.Errorf("构建请求体失败：%v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("构建请求失败：%v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second} // 合并大文件可能耗时较长
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("服务端返回错误：%d，详情：%s", resp.StatusCode, string(body))
	}

	// 解析合并结果
	type respData struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		SavePath string `json:"save_path"`
	}
	var data respData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("解析合并结果失败：%v", err)
	}

	fmt.Printf("文件合并成功，服务端存储路径：%s\n", data.SavePath)
	return nil
}
