package db

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// 哈希表中的键值对实体（保持不变）
type hashEntry struct {
	key   string
	value interface{}
	next  *hashEntry // 用于处理哈希冲突的链表
}

// 哈希表结构（新增并发安全锁，可选启用）
type HashTable struct {
	table      []*hashEntry // 哈希表数组
	size       int          // 当前元素总数量
	capacity   int          // 哈希表容量
	loadFactor float64      // 负载因子，用于触发扩容
	mu         sync.RWMutex // 并发安全锁（默认启用）
}

// 创建新的哈希表（支持并发安全控制）
func NewHashTable(initialCapacity int, loadFactor float64) *HashTable {
	if initialCapacity <= 0 {
		initialCapacity = 16 // 默认初始容量
	}
	if loadFactor <= 0 || loadFactor > 1 {
		loadFactor = 0.75 // 默认负载因子
	}

	return &HashTable{
		table:      make([]*hashEntry, initialCapacity),
		capacity:   initialCapacity,
		loadFactor: loadFactor,
	}
}

// 禁用并发安全（如果不需要多协程访问，可调用此方法提升性能）
func (ht *HashTable) DisableConcurrentSafe() {
	ht.mu = sync.RWMutex{} // 空锁（不影响原有逻辑，仅取消并发控制）
}

// 哈希函数：将字符串键映射到哈希表索引（优化负数处理）
func (ht *HashTable) hash(key string) int {
	hash := 0
	for _, c := range key {
		hash = 31*hash + int(c)
	}
	// 确保索引非负（处理哈希值为负数的情况）
	if hash < 0 {
		hash = -hash
	}
	return hash % ht.capacity
}

// 检查是否需要扩容（保持不变）
func (ht *HashTable) needResize() bool {
	return float64(ht.size)/float64(ht.capacity) >= ht.loadFactor
}

// 扩容哈希表（线程安全）
func (ht *HashTable) resize() {
	newCapacity := ht.capacity * 2
	newTable := make([]*hashEntry, newCapacity)

	// 重新哈希所有元素到新表
	for _, entry := range ht.table {
		current := entry
		for current != nil {
			next := current.next // 保存下一个节点引用
			// 重新计算新索引
			hash := 0
			for _, c := range current.key {
				hash = 31*hash + int(c)
			}
			if hash < 0 {
				hash = -hash
			}
			index := hash % newCapacity

			// 头插法插入新表
			current.next = newTable[index]
			newTable[index] = current

			current = next
		}
	}

	ht.table = newTable
	ht.capacity = newCapacity
}

// -------------------------- 完善的 HDel 系列命令 --------------------------

// HDel 单字段删除（原有功能，优化并发安全和返回值）
// 返回 true：删除成功（字段存在），false：删除失败（字段不存在）
func (ht *HashTable) HDel(key, field string) bool {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	
	fullKey := fmt.Sprintf("%s:%s", key, field)
	index := ht.hash(fullKey)

	entry := ht.table[index]
	var prev *hashEntry = nil

	for entry != nil {
		if entry.key == fullKey {
			// 从链表中移除节点
			if prev == nil {
				ht.table[index] = entry.next
			} else {
				prev.next = entry.next
			}
			ht.size--
			return true
		}
		prev = entry
		entry = entry.next
	}

	return false
}

// HDelMulti 批量删除多个字段（优化性能，返回删除成功数量）
// 支持传入多个字段，原子操作（要么全删，要么全不删）
func (ht *HashTable) HDelMulti(key string, fields ...string) int {
	if len(fields) == 0 {
		return 0
	}

	ht.mu.Lock()
	defer ht.mu.Unlock()

	deletedCount := 0
	// 先收集所有要删除的节点（避免边遍历边删除导致的索引错乱）
	toDelete := make(map[string]struct{})
	for _, field := range fields {
		toDelete[fmt.Sprintf("%s:%s", key, field)] = struct{}{}
	}

	// 遍历哈希表，删除匹配的节点
	for i, entry := range ht.table {
		var prev *hashEntry = nil
		current := entry
		for current != nil {
			next := current.next // 保存下一个节点
			if _, ok := toDelete[current.key]; ok {
				// 移除当前节点
				if prev == nil {
					ht.table[i] = next
				} else {
					prev.next = next
				}
				deletedCount++
				ht.size--
				delete(toDelete, current.key) // 避免重复删除
			} else {
				prev = current // 只有不删除时才更新前驱节点
			}
			current = next
		}
	}

	return deletedCount
}

// HClear 清空指定哈希表的所有字段（原子操作，返回清空数量）
func (ht *HashTable) HClear(key string) int {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	count := 0
	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)

	for i, entry := range ht.table {
		var prev *hashEntry = nil
		current := entry
		for current != nil {
			next := current.next // 保存下一个节点
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				// 移除当前节点
				if prev == nil {
					ht.table[i] = next
				} else {
					prev.next = next
				}
				count++
				ht.size--
			} else {
				prev = current // 只有不删除时才更新前驱节点
			}
			current = next
		}
	}

	return count
}

// HDelHash  删除整个哈希表（包括所有字段）
// 返回 true：删除成功（哈希表存在），false：删除失败（哈希表不存在）
func (ht *HashTable) HDelHash(key string) bool {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)
	exists := false

	for i, entry := range ht.table {
		var prev *hashEntry = nil
		current := entry
		for current != nil {
			next := current.next // 保存下一个节点
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				// 移除当前节点
				if prev == nil {
					ht.table[i] = next
				} else {
					prev.next = next
				}
				ht.size--
				exists = true
			} else {
				prev = current // 只有不删除时才更新前驱节点
			}
			current = next
		}
	}

	return exists
}

// -------------------------- 完善的 HSet 系列命令（保留并优化） --------------------------

// HSet 单字段设置（支持覆盖，线程安全）
func (ht *HashTable) HSet(key, field string, value interface{}) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	ht.HSetMultiUnsafe(key, map[string]interface{}{field: value})
}

// HSetNX 字段不存在时才设置（原子操作，线程安全）
func (ht *HashTable) HSetNX(key, field string, value interface{}) bool {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	fullKey := fmt.Sprintf("%s:%s", key, field)
	index := ht.hash(fullKey)

	// 检查字段是否存在
	entry := ht.table[index]
	for entry != nil {
		if entry.key == fullKey {
			return false // 字段已存在，设置失败
		}
		entry = entry.next
	}

	// 字段不存在，执行设置
	if ht.needResize() {
		ht.resize()
		index = ht.hash(fullKey) // 扩容后重新计算索引
	}

	newEntry := &hashEntry{
		key:   fullKey,
		value: value,
		next:  ht.table[index],
	}
	ht.table[index] = newEntry
	ht.size++
	return true
}

// HSetMulti 批量设置多个字段（原子操作，线程安全）
func (ht *HashTable) HSetMulti(key string, fieldValues map[string]interface{}) {
	if len(fieldValues) == 0 {
		return
	}

	ht.mu.Lock()
	defer ht.mu.Unlock()

	ht.HSetMultiUnsafe(key, fieldValues)
}

// HSetMultiUnsafe 批量设置（无锁，内部调用）
func (ht *HashTable) HSetMultiUnsafe(key string, fieldValues map[string]interface{}) {
	newCount := 0
	for field := range fieldValues {
		fullKey := fmt.Sprintf("%s:%s", key, field)
		if !ht.hExistsUnsafe(fullKey) {
			newCount++
		}
	}

	// 预扩容
	for float64(ht.size+newCount)/float64(ht.capacity) >= ht.loadFactor {
		ht.resize()
	}

	// 批量插入/更新
	for field, value := range fieldValues {
		fullKey := fmt.Sprintf("%s:%s", key, field)
		index := ht.hash(fullKey)
		entry := ht.table[index]

		updated := false
		for entry != nil {
			if entry.key == fullKey {
				entry.value = value
				updated = true
				break
			}
			entry = entry.next
		}

		if !updated {
			newEntry := &hashEntry{
				key:   fullKey,
				value: value,
				next:  ht.table[index],
			}
			ht.table[index] = newEntry
			ht.size++
		}
	}
}

// HIncrBy 字段值自增（支持 int/int64/字符串数字，线程安全）
func (ht *HashTable) HIncrBy(key, field string, delta int64) (int64, error) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	fullKey := fmt.Sprintf("%s:%s", key, field)
	index := ht.hash(fullKey)
	entry := ht.table[index]

	// 查找字段
	for entry != nil {
		if entry.key == fullKey {
			// 检查值类型
			switch val := entry.value.(type) {
			case int:
				entry.value = val + int(delta)
				return int64(entry.value.(int)), nil
			case int64:
				entry.value = val + delta
				return entry.value.(int64), nil
			case string:
				num, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return 0, errors.New("value is not a valid integer")
				}
				num += delta
				entry.value = num
				return num, nil
			default:
				return 0, errors.New("value is not an integer type")
			}
		}
		entry = entry.next
	}

	// 字段不存在，初始化并自增
	if ht.needResize() {
		ht.resize()
		index = ht.hash(fullKey)
	}

	newVal := delta
	newEntry := &hashEntry{
		key:   fullKey,
		value: newVal,
		next:  ht.table[index],
	}
	ht.table[index] = newEntry
	ht.size++
	return newVal, nil
}

// -------------------------- 完善的 HGet 系列命令（保留并优化） --------------------------

// HGet 单字段获取（线程安全）
func (ht *HashTable) HGet(key, field string) (interface{}, error) {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	fullKey := fmt.Sprintf("%s:%s", key, field)
	index := ht.hash(fullKey)

	entry := ht.table[index]
	for entry != nil {
		if entry.key == fullKey {
			return entry.value, nil
		}
		entry = entry.next
	}

	return nil, errors.New("field not found")
}

// HMGet 批量获取多个字段（线程安全）
func (ht *HashTable) HMGet(key string, fields ...string) []interface{} {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	result := make([]interface{}, len(fields))
	for i, field := range fields {
		fullKey := fmt.Sprintf("%s:%s", key, field)
		index := ht.hash(fullKey)
		entry := ht.table[index]
		for entry != nil {
			if entry.key == fullKey {
				result[i] = entry.value
				break
			}
			entry = entry.next
		}
	}
	return result
}

// HGetAll 获取所有字段和值（线程安全）
func (ht *HashTable) HGetAll(key string) map[string]interface{} {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	result := make(map[string]interface{})
	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				field := current.key[prefixLen:]
				result[field] = current.value
			}
			current = current.next
		}
	}

	return result
}

// HKeys 获取所有字段名（线程安全）
func (ht *HashTable) HKeys(key string) []string {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	var keys []string
	prefix := fmt.Sprintf("%s:%s", key, "")
	prefixLen := len(prefix)

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				field := current.key[prefixLen:]
				keys = append(keys, field)
			}
			current = current.next
		}
	}

	return keys
}

// HVals 获取所有字段值（线程安全）
func (ht *HashTable) HVals(key string) []interface{} {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	var vals []interface{}
	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				vals = append(vals, current.value)
			}
			current = current.next
		}
	}

	return vals
}

// HLen 获取字段数量（线程安全）
func (ht *HashTable) HLen(key string) int {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	count := 0
	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				count++
			}
			current = current.next
		}
	}

	return count
}

// HExists 检查字段是否存在（线程安全）
func (ht *HashTable) HExists(key, field string) bool {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	fullKey := fmt.Sprintf("%s:%s", key, field)

	return ht.hExistsUnsafe(fullKey)
}

// hExistsUnsafe 无锁检查字段是否存在（内部调用）
func (ht *HashTable) hExistsUnsafe(fullKey string) bool {
	index := ht.hash(fullKey)
	entry := ht.table[index]
	for entry != nil {
		if entry.key == fullKey {
			return true
		}
		entry = entry.next
	}
	return false
}

// -------------------------- 哈希表整体管理命令 --------------------------

// HashExists 检查哈希表是否存在（至少包含一个字段）
func (ht *HashTable) HashExists(key string) bool {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	prefix := fmt.Sprintf("%s:", key)
	prefixLen := len(prefix)

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			if len(current.key) > prefixLen && current.key[:prefixLen] == prefix {
				return true
			}
			current = current.next
		}
	}

	return false
}

// HashCount 获取哈希表总数（有多少个独立的 key）
func (ht *HashTable) HashCount() int {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	hashKeys := make(map[string]struct{})
	prefixSep := ":"

	for _, entry := range ht.table {
		current := entry
		for current != nil {
			// 提取哈希表 key（分隔符前的部分）
			sepIndex := strings.Index(current.key, prefixSep)
			if sepIndex > 0 {
				hashKey := current.key[:sepIndex]
				hashKeys[hashKey] = struct{}{}
			}
			current = current.next
		}
	}

	return len(hashKeys)
}

// Clear 清空整个哈希表（所有哈希表的所有字段）
func (ht *HashTable) Clear() {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	ht.table = make([]*hashEntry, ht.capacity)
	ht.size = 0
}

// Stats 获取哈希表统计信息
func (ht *HashTable) Stats() map[string]interface{} {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	return map[string]interface{}{
		"capacity":    ht.capacity,
		"size":        ht.size,
		"load_factor": float64(ht.size) / float64(ht.capacity),
		"hash_count":  ht.HashCount(),
	}
}

// HStrLen 获取指定字段值的字符串长度
// 若字段不存在返回错误，若值非字符串则返回其字符串表示的长度
func (ht *HashTable) HStrLen(key, field string) (int, error) {
	val, err := ht.HGet(key, field)
	if err != nil {
		return 0, err
	}
	return len(fmt.Sprintf("%v", val)), nil
}
