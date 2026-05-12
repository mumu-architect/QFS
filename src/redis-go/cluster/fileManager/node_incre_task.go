package fileManager

// SyncTask 文件同步任务体
type IncrementSyncTask struct {
	TaskID     string `json:"fileId"` // 唯一任务ID
	FileName   string `json:"fileName"`
	FilePath   string `json:"filePath"`
	MineType   string `json:"mineType"`
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
	IsDeleted  bool   `json:"isDeleted"`
	Status     string `json:"status"` //Pending,Running,Finished
}

const (
	DefaultIncrementWorkerNum = 6
	DefaultIncrementQueueCap  = 300
)
