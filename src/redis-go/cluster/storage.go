package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
func (c *Cluster) initRedisData() {
	c.dataStore = RedisData{
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

func getDataFilePath(addr string) string {
	filename := strings.Replace(addr, ":", "_", -1) + ".data"
	return filepath.Join(DataDir, filename)
}

// 数据落盘：序列化RedisData整体落盘
func (c *Cluster) persistData() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 序列化整个RedisData结构
	data, err := json.Marshal(c.dataStore)
	if err != nil {
		log.Printf("数据序列化失败：%v", err)
		return err
	}

	tmpFile := c.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("临时文件写入失败：%v", err)
		return err
	}

	if err := os.Rename(tmpFile, c.dataFile); err != nil {
		log.Printf("文件替换失败：%v", err)
		return err
	}

	log.Printf("主节点 %s 数据落盘成功（String:%d 条，Hash:%d 个）",
		c.LocalNode.Addr, len(c.dataStore.(RedisData).String), len(c.dataStore.(RedisData).Hash))
	return nil
}

// 加载持久化数据：反序列化为RedisData
func (c *Cluster) loadPersistedData() error {
	if _, err := os.Stat(c.dataFile); os.IsNotExist(err) {
		c.initRedisData() // 无文件时初始化空结构
		return nil
	}

	data, err := os.ReadFile(c.dataFile)
	if err != nil {
		return fmt.Errorf("文件读取失败：%v", err)
	}

	var redisData RedisData
	if err := json.Unmarshal(data, &redisData); err != nil {
		return fmt.Errorf("数据反序列化失败：%v", err)
	}

	c.dataStore = redisData
	log.Printf("加载持久化数据成功（String:%d 条，Hash:%d 个）",
		len(redisData.String), len(redisData.Hash))
	return nil
}

// 从节点数据落盘（同主节点格式）
func (c *Cluster) persistSlaveData() error {
	if c.LocalNode.Type != Slave {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.Marshal(c.dataStore)
	if err != nil {
		log.Printf("从节点 %s 数据序列化失败：%v", c.LocalNode.Addr, err)
		return err
	}

	tmpFile := c.dataFile + ".slave.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("从节点 %s 临时文件写入失败：%v", c.LocalNode.Addr, err)
		return err
	}

	if err := os.Rename(tmpFile, c.dataFile); err != nil {
		log.Printf("从节点 %s 文件替换失败：%v", c.LocalNode.Addr, err)
		return err
	}

	log.Printf("从节点 %s 数据落盘成功（String:%d 条，Hash:%d 个）",
		c.LocalNode.Addr, len(c.dataStore.(RedisData).String), len(c.dataStore.(RedisData).Hash))
	return nil
}

// 辅助：获取String类型数据（供外部调用）
func (c *Cluster) getStringData(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, exists := c.dataStore.(RedisData).String[key]
	return val, exists
}

// 辅助：设置String类型数据
func (c *Cluster) setStringData(key, val string) {
	c.dataStore.(RedisData).String[key] = val
	c.recentChanges[key] = time.Now() // 记录变更，用于增量同步
}

// 辅助：获取Hash类型数据
func (c *Cluster) getHashData(key, field string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	hashMap, exists := c.dataStore.(RedisData).Hash[key]
	if !exists {
		return "", false
	}
	val, exists := hashMap[field]
	return val, exists
}

// 辅助：设置Hash类型数据
func (c *Cluster) setHashData(key, field, val string) {
	if _, exists := c.dataStore.(RedisData).Hash[key]; !exists {
		c.dataStore.(RedisData).Hash[key] = make(map[string]string)
	}
	c.dataStore.(RedisData).Hash[key][field] = val
	c.recentChanges[fmt.Sprintf("hash:%s:%s", key, field)] = time.Now() // 哈希变更标记
}
