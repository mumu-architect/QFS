package common

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 全局配置（可根据需求调整）
const (
	ChunkSize         = 5 * 1024 * 1024         // 分片大小：5MB
	UploadDir         = "./data/uploaded_files" // 最终文件存储目录
	TempChunkDir      = "./data/temp_chunks"    // 临时分片存储目录
	FileMD5Header     = "X-File-MD5"            // 请求头：文件唯一标识（MD5）
	ChunkIdxHeader    = "X-Chunk-Index"         // 请求头：分片索引（从0开始）
	TotalChunksHeader = "X-Total-Chunks"        // 请求头：总分片数
)

// 初始化存储目录
func init() {
	_ = os.MkdirAll(UploadDir, 0755)
	_ = os.MkdirAll(TempChunkDir, 0755)
}

// CalcFileMD5 计算文件MD5（作为文件唯一标识）
func CalcFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败：%v", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算MD5失败：%v", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SplitFile 拆分文件为分片（客户端用）
func SplitFile(filePath string) ([]string, string, error) {
	// 计算文件MD5
	fileMD5, err := CalcFileMD5(filePath)
	if err != nil {
		return nil, "", err
	}

	// 创建客户端临时分片目录（按MD5命名，避免冲突）
	clientChunkDir := filepath.Join(os.TempDir(), "file_chunks", fileMD5)
	_ = os.MkdirAll(clientChunkDir, 0755)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	var chunkPaths []string
	buf := make([]byte, ChunkSize)
	chunkIdx := 0

	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", fmt.Errorf("读取文件失败：%v", err)
		}

		// 写入分片文件
		chunkPath := filepath.Join(clientChunkDir, fmt.Sprintf("chunk_%d", chunkIdx))
		chunkFile, err := os.Create(chunkPath)
		if err != nil {
			return nil, "", fmt.Errorf("创建分片文件失败：%v", err)
		}
		if _, err := chunkFile.Write(buf[:n]); err != nil {
			chunkFile.Close()
			return nil, "", fmt.Errorf("写入分片文件失败：%v", err)
		}
		chunkFile.Close()

		chunkPaths = append(chunkPaths, chunkPath)
		chunkIdx++
	}

	return chunkPaths, fileMD5, nil
}

// MergeChunks 合并分片为完整文件（服务端用）
func MergeChunks(fileMD5 string, totalChunks int) (string, error) {
	// 服务端分片存储目录
	serverChunkDir := filepath.Join(TempChunkDir, fileMD5)
	if _, err := os.Stat(serverChunkDir); os.IsNotExist(err) {
		return "", errors.New("分片目录不存在，可能未上传任何分片")
	}

	// 最终文件路径（用MD5+原文件后缀命名，避免冲突）
	srcFileName := filepath.Base(fileMD5) // 实际场景可从客户端传入原始文件名
	destPath := filepath.Join(UploadDir, fmt.Sprintf("%s%s", fileMD5, filepath.Ext(srcFileName)))
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败：%v", err)
	}
	defer destFile.Close()

	// 按索引顺序合并分片
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(serverChunkDir, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return "", fmt.Errorf("分片 %d 不存在：%v", i, err)
		}

		if _, err := io.Copy(destFile, chunkFile); err != nil {
			chunkFile.Close()
			return "", fmt.Errorf("合并分片 %d 失败：%v", i, err)
		}
		chunkFile.Close()

		// 合并后删除分片（节省空间）
		_ = os.Remove(chunkPath)
	}

	// 删除空分片目录
	_ = os.Remove(serverChunkDir)

	// 校验合并后文件的MD5（确保完整性）
	mergedMD5, err := CalcFileMD5(destPath)
	if err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("校验合并文件MD5失败：%v", err)
	}
	if mergedMD5 != fileMD5 {
		_ = os.Remove(destPath)
		return "", errors.New("文件合并后MD5不匹配，上传失败")
	}

	return destPath, nil
}
