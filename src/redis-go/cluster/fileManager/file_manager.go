package fileManager

import "fmt"

type FileInfo struct {
	FIleID     int64  `json:"fIleID"`   //文件唯一id,雪花id
	FileName   string `json:"fileName"` //文件原文件名称
	FilePath   string `json:"filePath"` //文件当前路径
	FileSize   int64  `json:"fileSize"` //文件总大小
	MineType   string `json:"mineType"` //文件类型ContentType
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
	IsDeleted  bool   `json:"IsDeleted"` //true|false
}

const FileCacheKey = "File"

type FileManager struct {
}

// GenerateFileCacheKey 获取缓存文件的key
func GenerateFileCacheKey(flakeId int64) string {
	fileCacheKey := fmt.Sprintf("%s:%d", FileCacheKey, flakeId)
	return fileCacheKey
}
