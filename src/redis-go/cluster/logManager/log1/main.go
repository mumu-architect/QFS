package logManager

import (
	"encoding/json"
	"fmt"
	"time"
)

func main() {
	logMgr, err := NewLogManager()
	if err != nil {
		panic(err)
	}
	defer logMgr.Close()

	// 测试写入
	entry := LogEntry{
		FlowID:    "123456",
		FileID:    "434444",
		CMD:       "Get",
		Key:       "name",
		Field:     "age",
		Value:     "testData",
		Version:   10,
		Timestamp: time.Now().UnixMilli(),
	}
	ok := logMgr.WriteLog(entry)
	fmt.Println("写入结果:", ok)

	// 更新游标
	_ = logMgr.UpdateLocalCursor("2", "123456")
	_ = logMgr.UpdateRemoteCursor("2", "remote_123456")

	// 获取游标
	cursor := logMgr.GetCursor()
	fmt.Printf("本地游标:%s 远端游标:%s\n", cursor.LocalMaxFlowID, cursor.RemoteMaxFlowID)

	// TODO：分批读取，每次800条，顺序读取日志
	logs, _ := logMgr.ReadAllLog()
	// 这样打印，就能看到完整 JSON 键名！
	bs, _ := json.MarshalIndent(logs, "", "  ")
	fmt.Println("读取日志数量:", string(bs))
	fmt.Printf("读取日志数量:%d\n", len(logs))

}
