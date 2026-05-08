package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster/logManager"
)

// PullSyncLoop 循环拉去leader的最新log数据
func (cl *Cluster) PullSyncLoop() {
	tt := time.NewTicker(500 * time.Millisecond)
	for range tt.C {
		if err := cl.PullLeaderLogOnce(); err != nil {
			fmt.Printf("pull leader log error:%v \n", err)
		}
	}
}

// PullLeaderLogOnce  从节点拉取主节点增量日志
func (cl *Cluster) PullLeaderLogOnce() error {
	logMap, err := cl.NodeLog.ReqMasterLog()
	if err != nil {
		return err
	}
	for idx, data := range logMap.DataMap {
		err := cl.NodeLog.WriteLocalLog(idx, data)
		if errors.Is(err, wal.ErrOutOfOrder) {
			continue
		}
		//增量数据写入内存
		_, err = cl.writeToMemory(data)
		if err != nil {
			return err
		}
	}
	if logMap.HasMore {
		go func() {
			_ = cl.PullLeaderLogOnce()
		}()
	}
	return nil
}

// 清空内存
func clearMemory() {}

// RestartBatchLoadLog  重启分批加载 低IO高性能
func (cl *Cluster) RestartBatchLoadLog() error {
	//clearMemory()
	//回滚6条数据
	err := cl.NodeLog.RollbackLogLastIndex()
	fmt.Printf("1112=========%v==%v \n", 0, 0)
	if err != nil {
		fmt.Printf("1112=========%v==%v \n", 0, 0)
		return err
	}
	firstIdx, err := cl.NodeLog.LocalLog.FirstIndex()
	if err != nil {
		fmt.Printf("1113=========%v==0 \n", firstIdx)
		firstIdx = 0 // 空日志赋值 0
		return err
	}

	lastIdx, err := cl.NodeLog.LocalLog.LastIndex()
	if err != nil {
		fmt.Printf("1113=========%v==0 \n", err)
		firstIdx = 0 // 空日志赋值 0
		return err
	}
	fmt.Printf("1114=========%v==%v \n", firstIdx, lastIdx)
	if firstIdx > lastIdx {
		return nil
	}

	start := firstIdx
	for start <= lastIdx {
		end := start + batchSize - 1
		if end > lastIdx {
			end = lastIdx
		}
		fmt.Printf("1115=========%v==%v \n", firstIdx, lastIdx)
		for idx := start; idx <= end; idx++ {
			data, err := cl.NodeLog.LocalLog.Read(idx)
			if err != nil {
				if errors.Is(err, wal.ErrNotFound) {
					continue
				}
				return fmt.Errorf("read idx:%d err:%w", idx, err)
			}
			fmt.Printf("1116=========%v \n", data)
			_, err = cl.writeToMemory(data)
			if err != nil {
				return err
			}
		}
		start = end + 1
	}
	return nil
}

// 写入内存 业务方法

func (cl *Cluster) writeToMemory(data []byte) (bool, error) {
	var entry logManager.LogEntry
	err := json.Unmarshal(data, &entry)
	if err != nil {
		return false, err
	}
	if entry.CMD == "Set" {
		cl.SetStringData(entry.Key, entry.Value)
	} else if entry.CMD == "HSet" || entry.CMD == "HMSet" {
		cl.SetHashData(entry.Key, entry.Field, entry.Value)
	}
	return true, nil
}
