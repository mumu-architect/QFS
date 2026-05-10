package fileManager

import "time"

// SyncTask 文件同步任务体
type SyncTask struct {
	TaskID     string // 唯一任务ID
	SourceURL  string // 主机HTTP下载地址
	TargetPath string // 从机落地路径
	FileSize   int64  // 文件大小校验
	FileMD5    string // 新增：文件MD5哈希值
}

const (
	DefaultWorkerNum = 6
	DefaultQueueCap  = 300
	ExpireDuplicate  = 5 * time.Minute

	// 重试配置
	MaxRetryCount = 3 // 最大重试3次
	RetryInterval = 1 * time.Second
)
