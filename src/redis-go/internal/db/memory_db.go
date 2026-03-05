package db

import (
	"errors"
	"sync"
)

type MemoryDB struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMemoryDB() *MemoryDB {
	return &MemoryDB{
		data: make(map[string]string),
	}
}

// -------------------------- 原有基础功能（保持不变） --------------------------
func (db *MemoryDB) Set(key, value string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
}

func (db *MemoryDB) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	val, ok := db.data[key]
	return val, ok
}

func (db *MemoryDB) Del(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, key)
}

func (db *MemoryDB) KeyExists(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	_, ok := db.data[key]
	return ok
}

func (db *MemoryDB) KeyCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.data)
}

func (db *MemoryDB) ForEachKey(fn func(key string)) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	for key := range db.data {
		fn(key)
	}
}

func (db *MemoryDB) Dump(key string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	val, ok := db.data[key]
	if !ok {
		return "", errors.New("key not found")
	}
	return val, nil
}

func (db *MemoryDB) Restore(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
	return nil
}

// -------------------------- 新增功能实现 --------------------------

// SetNX 键不存在时设置（原子操作）
// 返回 true：设置成功（键不存在），false：设置失败（键已存在）
func (db *MemoryDB) SetNX(key, value string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.data[key]; !exists {
		db.data[key] = value
		return true
	}
	return false
}

// SetXX 键存在时更新（原子操作）
// 返回 true：更新成功（键存在），false：更新失败（键不存在）
func (db *MemoryDB) SetXX(key, value string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.data[key]; exists {
		db.data[key] = value
		return true
	}
	return false
}

// MSet 批量设置多键值对（原子操作）
// 所有键值对要么全部设置成功，要么全部不设置（无部分成功）
func (db *MemoryDB) MSet(kv map[string]string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for k, v := range kv {
		db.data[k] = v
	}
}

// MSetNX 批量设置（所有键不存在才生效，原子操作）
// 返回 true：所有键都不存在，设置成功；false：至少有一个键已存在，设置失败
func (db *MemoryDB) MSetNX(kv map[string]string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 先检查所有键是否都不存在
	for k := range kv {
		if _, exists := db.data[k]; exists {
			return false
		}
	}

	// 所有键都不存在，批量设置
	for k, v := range kv {
		db.data[k] = v
	}
	return true
}

// GetSet 设置新值并返回旧值（原子操作）
// 若键不存在，返回空字符串和 false；若存在，返回旧值和 true
func (db *MemoryDB) GetSet(key, value string) (string, bool) {
	db.mu.Lock()
	defer db.mu.Unlock()

	oldVal, exists := db.data[key]
	if exists {
		db.data[key] = value
	} else {
		// 键不存在时，是否设置新值？Redis 行为是：设置新值并返回 nil
		db.data[key] = value
	}
	return oldVal, exists
}

// MGet 批量获取多键值对（原子操作）
// 返回值顺序与传入键顺序一致，不存在的键对应的值为空字符串
func (db *MemoryDB) MGet(keys ...string) []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = db.data[k] // 不存在的键默认返回空字符串
	}
	return result
}

// GetRange 获取字符串子串（原子操作）
// start：起始索引（0 开始，负数表示倒数，如 -1 是最后一个字符）
// end：结束索引（包含，负数表示倒数）
// 若键不存在，返回空字符串；若索引超出范围，返回实际存在的子串
func (db *MemoryDB) GetRange(key string, start, end int) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	val, exists := db.data[key]
	if !exists {
		return ""
	}

	runes := []rune(val)
	length := len(runes)
	if length == 0 {
		return ""
	}

	// 处理负索引
	if start < 0 {
		start += length
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += length
		if end < 0 {
			return ""
		}
	}

	// 处理索引超出范围
	if start >= length {
		return ""
	}
	if end >= length {
		end = length - 1
	}
	if start > end {
		return ""
	}

	return string(runes[start : end+1])
}

// StrLen 获取字符串长度（原子操作）
// 若键不存在，返回 0；若存在，返回值的字符长度（支持中文等多字节字符）
func (db *MemoryDB) StrLen(key string) int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	val, exists := db.data[key]
	if !exists {
		return 0
	}
	return len([]rune(val)) // 用 rune 计算字符数，支持多字节字符
}
