package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DataDir = "./redis-cluster-data" // 数据落盘目录
)

// 扩展：支持Redis核心数据结构
type RedisData struct {
	String map[string]string            `json:"string"` // String类型：key->val
	Hash   map[string]map[string]string `json:"hash"`   // Hash类型：key->field->val
}

// 初始化数据存储（替换原dataStore）
func (cl *Cluster) initRedisData() {
	cl.dataStore = RedisData{
		String: make(map[string]string),
		Hash:   make(map[string]map[string]string),
	}
}

// 原有方法修改：适配新数据结构
func InitDataDir() {
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败：%v", err)
	}
}

func getDataFilePath(nodeId int, addr string) string {
	if err := os.MkdirAll(DataDir+"/data_"+strconv.Itoa(nodeId), 0755); err != nil {
		log.Fatalf("创建nodeID数据目录失败：%v", err)
	}
	filename := strconv.Itoa(nodeId) + strings.Replace(addr, ":", "_", -1) + ".data"
	return filepath.Join(DataDir+"/data_"+strconv.Itoa(nodeId), filename)
}

// 数据落盘：序列化RedisData整体落盘
func (cl *Cluster) persistData() error {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	// 序列化整个RedisData结构
	data, err := json.Marshal(cl.dataStore)
	if err != nil {
		log.Printf("数据序列化失败：%v", err)
		return err
	}

	tmpFile := cl.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("临时文件写入失败：%v", err)
		return err
	}

	if err := os.Rename(tmpFile, cl.dataFile); err != nil {
		log.Printf("文件替换失败：%v", err)
		return err
	}

	log.Printf("主节点 %s 数据落盘成功（String:%d 条，Hash:%d 个）",
		cl.LocalNode.Addr, len(cl.dataStore.(RedisData).String), len(cl.dataStore.(RedisData).Hash))
	return nil
}

// 加载持久化数据：反序列化为RedisData
func (cl *Cluster) loadPersistedData() error {
	if _, err := os.Stat(cl.dataFile); os.IsNotExist(err) {
		cl.initRedisData() // 无文件时初始化空结构
		return nil
	}

	data, err := os.ReadFile(cl.dataFile)
	if err != nil {
		return fmt.Errorf("文件读取失败：%v", err)
	}

	var redisData RedisData
	if err := json.Unmarshal(data, &redisData); err != nil {
		return fmt.Errorf("数据反序列化失败：%v", err)
	}

	cl.dataStore = redisData
	log.Printf("加载持久化数据成功（String:%d 条，Hash:%d 个）",
		len(redisData.String), len(redisData.Hash))
	return nil
}

// 从节点数据落盘（同主节点格式）
func (cl *Cluster) persistSlaveData() error {
	if cl.LocalNode.LeaderID == cl.LocalNode.NodeID {
		return nil
	}

	cl.mu.RLock()
	defer cl.mu.RUnlock()

	data, err := json.Marshal(cl.dataStore)
	if err != nil {
		log.Printf("从节点 %s 数据序列化失败：%v", cl.LocalNode.Addr, err)
		return err
	}

	tmpFile := cl.dataFile + ".slave.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("从节点 %s 临时文件写入失败：%v", cl.LocalNode.Addr, err)
		return err
	}

	if err := os.Rename(tmpFile, cl.dataFile); err != nil {
		log.Printf("从节点 %s 文件替换失败：%v", cl.LocalNode.Addr, err)
		return err
	}

	log.Printf("从节点 %s 数据落盘成功（String:%d 条，Hash:%d 个）",
		cl.LocalNode.Addr, len(cl.dataStore.(RedisData).String), len(cl.dataStore.(RedisData).Hash))
	return nil
}

// 辅助：获取String类型数据（供外部调用）
func (cl *Cluster) getStringData(key string) (string, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	val, exists := cl.dataStore.(RedisData).String[key]
	return val, exists
}

// 辅助：设置String类型数据
func (cl *Cluster) setStringData(key, val string) {
	cl.dataStore.(RedisData).String[key] = val
	cl.recentChanges[key] = time.Now() // 记录变更，用于增量同步
}

// 辅助：获取Hash类型数据
func (cl *Cluster) getHashData(key, field string) (string, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	hashMap, exists := cl.dataStore.(RedisData).Hash[key]
	if !exists {
		return "", false
	}
	val, exists := hashMap[field]
	return val, exists
}

// 辅助：设置Hash类型数据
func (cl *Cluster) setHashData(key, field, val string) {
	if _, exists := cl.dataStore.(RedisData).Hash[key]; !exists {
		cl.dataStore.(RedisData).Hash[key] = make(map[string]string)
	}
	cl.dataStore.(RedisData).Hash[key][field] = val
	cl.recentChanges[fmt.Sprintf("hash:%s:%s", key, field)] = time.Now() // 哈希变更标记

	//

}

// // TODO:测试使用,最后删除
// // 辅助：获取Hash类型数据
func (cl *Cluster) GetHashData(key, field string) (string, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	hashMap, exists := cl.dataStore.(RedisData).Hash[key]
	if !exists {
		return "", false
	}
	val, exists := hashMap[field]
	return val, exists
}

// TODO:测试使用,最后删除
// 辅助：获取String类型数据（供外部调用）
func (cl *Cluster) GetStringData(key string) (string, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	val, exists := cl.dataStore.(RedisData).String[key]
	return val, exists
}

// 辅助：设置Hash类型数据
func (cl *Cluster) SetHashData(key, field, val string) {
	if _, exists := cl.dataStore.(RedisData).Hash[key]; !exists {
		cl.dataStore.(RedisData).Hash[key] = make(map[string]string)
	}
	cl.dataStore.(RedisData).Hash[key][field] = val
	cl.recentChanges[fmt.Sprintf("hash:%s:%s", key, field)] = time.Now() // 哈希变更标记
}

// 辅助：设置String类型数据
func (cl *Cluster) SetStringData(key, val string) {
	cl.dataStore.(RedisData).String[key] = val
	cl.recentChanges[key] = time.Now() // 记录变更，用于增量同步
}
