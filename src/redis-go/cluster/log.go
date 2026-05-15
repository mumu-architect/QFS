package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tidwall/wal"
	"mumu.com/redis-go/cluster/fileManager"
	"mumu.com/redis-go/cluster/logManager"
)

var IncrementFileMaxIdx uint64

// PullSyncLoop 循环拉去leader的最新log数据
func (cl *Cluster) PullSyncLoop(logIndex uint64) {
	tt := time.NewTicker(500 * time.Millisecond)
	for range tt.C {
		if err := cl.PullLeaderLogOnce(logIndex); err != nil {
			fmt.Printf("pull leader log error:%v \n", err)
		}
	}
}

// PullLeaderLogOnce  从节点拉取主节点增量日志
func (cl *Cluster) PullLeaderLogOnce(logIndex uint64) error {
	//var onceFlag = false
	//var logMap *logManager.SyncResponse
	//var logMapOnce *logManager.SyncResponse
	//if !onceFlag {
	//	logMapOne, err := cl.NodeLog.ReqMasterLog(0)
	//	if err != nil {
	//		return err
	//	}
	//	onceFlag = true
	//	logMap = logMapOne
	//	logMapOnce = logMapOne
	//} else {
	//	logMapMany, err := cl.NodeLog.ReqMasterLog()
	//	if err != nil {
	//		return err
	//	}
	//	logMap = logMapMany
	//}
	logMap, err := cl.NodeLog.ReqMasterLog(logIndex)
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
		////TODO:leader增量文件同步到本地,先写入本地增量日志在通过worker执行
		//if idx <= logMapOnce.IncrementFileMaxIdx {
		//	var fileInfo *fileManager.FileInfo
		//	var entry *logManager.LogEntry
		//	_ = json.Unmarshal(data, &entry)
		//	_ = json.Unmarshal([]byte(entry.Value), &fileInfo)
		//	fmt.Printf("fileInfo:%v \n", fileInfo)
		//	fileEntry := &fileManager.FileEntry{
		//		FileId:     strconv.FormatInt(fileInfo.FileID, 10),
		//		FileName:   fileInfo.FileName,
		//		FilePath:   fileInfo.FilePath,
		//		MineType:   fileInfo.MineType,
		//		FileSize:   fileInfo.FileSize,
		//		CreateTime: fileInfo.CreateTime,
		//		UpdateTime: fileInfo.UpdateTime,
		//		IsDeleted:  fileInfo.IsDeleted,
		//		Status:     "Pending", //Pending,Running,Finished
		//	}
		//	cl.NodeFile.IncrementFileManager.WriteLog(fileEntry)
		//	//比较耗时
		//}

	}
	if logMap.HasMore {
		go func() {
			_ = cl.PullLeaderLogOnce(logIndex)
		}()
	}
	return nil
}

// PullLeaderIncrementLogOnce   从节点拉取主节点增量日志,文件
func (cl *Cluster) PullLeaderIncrementLogOnce() (*logManager.SyncResponse, error) {
	var logMapOnce *logManager.SyncResponse

	logMapOnce, err := cl.NodeLog.ReqMasterLog(0)
	if err != nil {
		return nil, err
	}
	return logMapOnce, nil
}

// PullLeaderIncrementLog 增量文件一次没拉去完多拉去几次
func (cl *Cluster) PullLeaderIncrementLog(logIndex uint64, maxIndex uint64) error {
	var logMapOnce *logManager.SyncResponse
	logMapOnce, err := cl.NodeLog.ReqMasterIncrementLog(logIndex, maxIndex)
	if err != nil {
		return err
	}
	_ = cl.doPullLeaderIncrementLog(logMapOnce, logIndex, maxIndex)
	return nil
}

// DoPullLeaderIncrementLog 执行增量日志文件worker
func (cl *Cluster) doPullLeaderIncrementLog(logMap *logManager.SyncResponse, logIndex uint64, maxIndex uint64) error {

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
		//TODO:leader增量文件同步到本地,先写入本地增量日志在通过worker执行
		if idx <= logMap.IncrementFileMaxIdx {
			var fileInfo *fileManager.FileInfo
			var entry *logManager.LogEntry
			_ = json.Unmarshal(data, &entry)
			_ = json.Unmarshal([]byte(entry.Value), &fileInfo)
			fmt.Printf("doPullLeaderIncrementLog-fileInfo:%v \n", fileInfo)
			fileEntry := &fileManager.FileEntry{
				FileId:     strconv.FormatInt(fileInfo.FileID, 10),
				FileName:   fileInfo.FileName,
				FilePath:   fileInfo.FilePath,
				MineType:   fileInfo.MineType,
				FileSize:   fileInfo.FileSize,
				CreateTime: fileInfo.CreateTime,
				UpdateTime: fileInfo.UpdateTime,
				IsDeleted:  fileInfo.IsDeleted,
				Status:     "Pending", //Pending,Running,Finished
			}
			cl.NodeFile.IncrementFileManager.WriteLog(fileEntry)
			//比较耗时
		}

	}
	if logMap.HasMore {
		go func() {
			_ = cl.PullLeaderIncrementLog(logIndex, maxIndex)
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
