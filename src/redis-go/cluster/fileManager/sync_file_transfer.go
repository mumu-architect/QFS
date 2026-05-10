package fileManager

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DoRemoteFileSync  HTTP下载 + 重试机制 + MD5校验
func DoRemoteFileSync(task *SyncTask) error {
	var err error

	// 循环重试
	for retry := 0; retry <= MaxRetryCount; retry++ {
		err = taskDownload(task)
		if err == nil {
			// 下载成功，再做MD5校验
			md5Val, md5Err := GetFileMD5(task.TargetPath)
			if md5Err == nil && md5Val == task.FileMD5 {
				fmt.Printf("[同步成功] 文件:%s MD5校验通过\n", task.TargetPath)
				return nil
			}

			// MD5不匹配，判定失败，删除坏文件
			_ = os.Remove(task.TargetPath)
			err = fmt.Errorf("MD5校验不匹配，预期:%s 实际:%s", task.FileMD5, md5Val)
		}

		// 最后一次不再等待
		if retry >= MaxRetryCount {
			break
		}

		fmt.Printf("[同步失败] 第%d次重试, err:%v\n", retry+1, err)
		time.Sleep(RetryInterval)
	}

	return fmt.Errorf("同步最终失败，已用尽%d次重试", MaxRetryCount)
}
func taskDownload(task *SyncTask) error {
	fmt.Printf("[同步引擎] 开始同步: %s\n", task.TargetPath)

	// 1. 创建目录
	targetDir := filepath.Dir(task.TargetPath)
	_ = os.MkdirAll(targetDir, 0777)

	// 2. 解析源地址
	srcPath := strings.TrimPrefix(task.SourceURL, "file://")

	// 3. 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 4. 直接写入目标文件（Windows 兼容，不搞重命名）
	dstFile, err := os.Create(task.TargetPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 5. 拷贝
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		os.Remove(task.TargetPath)
		return err
	}

	fmt.Println("[同步引擎] ✅ 同步成功（Windows 兼容版）:", task.TargetPath)
	return nil
}

// 全局 HTTP 客户端（用于从机拉取主机文件）
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

// TODO: DoRemoteHttpFileSync从机 通过 HTTP 下载 主机文件
func DoRemoteHttpFileSync(task *SyncTask) error {
	fmt.Printf("[同步引擎] 远程HTTP下载: %s\n", task.SourceURL)

	// 1. 创建目标目录
	targetDir := filepath.Dir(task.TargetPath)
	err := os.MkdirAll(targetDir, 0777)
	if err != nil {
		fmt.Println("[同步引擎] 创建目录失败:", err)
		return err
	}

	// 2. 直接从主机 HTTP 下载文件
	resp, err := httpClient.Get(task.SourceURL)
	if err != nil {
		fmt.Println("[同步引擎] HTTP下载失败:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("[同步引擎] HTTP状态码错误:", resp.Status)
		return fmt.Errorf("下载失败：%s", resp.Status)
	}

	// 3. 创建目标文件
	dstFile, err := os.Create(task.TargetPath)
	if err != nil {
		fmt.Println("[同步引擎] 创建文件失败:", err)
		return err
	}
	defer dstFile.Close()

	// 4. 流式写入
	_, err = io.Copy(dstFile, resp.Body)
	if err != nil {
		os.Remove(task.TargetPath)
		return err
	}

	fmt.Println("[同步引擎] ✅ HTTP同步完成:", task.TargetPath)
	return nil
}

// GetFileMD5 计算一个文件的MD5值
func GetFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	// 转为16进制字符串
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
