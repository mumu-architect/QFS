package server

import (
	"fmt"
	"strconv"

	"mumu.com/redis-go/internal/cluster"
	"mumu.com/redis-go/internal/protocol"
)

// -------------------------- 新增哈希表命令实现 --------------------------

// handleHSetNX 处理 HSETNX 命令（字段不存在时才设置）
func (h *CommandHandler) handleHSetNX(args []protocol.Value) protocol.Value {
	// 校验参数：HSETNX hashKey field value（3个BulkString）
	if len(args) != 3 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString || args[2].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hsetnx' command"}
	}
	hashKey := args[0].Str
	field := args[1].Str
	value := args[2].Str

	// 集群模式：检查 hashKey 的槽位（Redis 哈希表命令基于 hashKey 计算槽位）
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 原子执行 HSetNX
	ok := h.server.GetHashTableStore().HSetNX(hashKey, field, value)
	if ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 成功返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 失败返回0
}

// handleHSetMulti 处理 HSETMULTI 命令（批量设置多个字段，原子操作）
// 命令格式：HSETMULTI hashKey field1 value1 field2 value2 ...
func (h *CommandHandler) handleHSetMulti(args []protocol.Value) protocol.Value {
	// 1. 参数校验：至少3个参数（hashKey + 至少1组 field-value），且总参数数为奇数
	if len(args) < 3 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hsetmulti' command: at least 3 arguments (hashKey, field1, value1)"}
	}
	if len(args)%2 != 1 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid argument count for 'hsetmulti' command: must be odd (hashKey + even number of field-value pairs)"}
	}
	// 校验 hashKey 类型为 BulkString
	if args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR hashKey must be a bulk string for 'hsetmulti' command"}
	}

	hashKey := args[0].Str
	// 核心修复：定义为 map[string]interface{} 以匹配 HashTableStore 接口要求
	fieldValues := make(map[string]interface{})

	// 2. 解析字段-值对，校验类型并转换
	for i := 1; i < len(args); i += 2 {
		// 确保不会越界（因前面已校验参数数为奇数，理论上不会触发，但做容错）
		if i+1 >= len(args) {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR incomplete field-value pair for 'hsetmulti' command"}
		}

		fieldVal := args[i]
		valueVal := args[i+1]

		// 校验字段和值均为 BulkString 类型（符合 Redis 协议规范）
		if fieldVal.Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR field at position %d must be a bulk string", i+1)}
		}
		if valueVal.Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR value for field '%s' must be a bulk string", fieldVal.Str)}
		}

		// 转换为 map[string]interface{}（string 可隐式转换为 interface{}）
		field := fieldVal.Str
		value := valueVal.Str
		fieldValues[field] = value
	}

	// 3. 集群模式：检查 hashKey 对应的槽位是否归当前节点所有
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR slot %d for hashKey '%s' has no owner", slot, hashKey)}
		}
		if ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hashKey '%s' belongs to slot %d (owner: %s), which is not handled by this node.go", hashKey, slot, ownerID)}
		}
	}

	// 4. 原子执行批量设置（HashTableStore 层保证原子性）
	h.server.GetHashTableStore().HSetMulti(hashKey, fieldValues)

	// 5. 返回成功响应（符合 Redis 批量操作响应规范）
	return protocol.Value{Type: protocol.TypeSimpleString, Str: "OK"}
}

// handleHIncrBy 处理 HINCRBY 命令（数值字段自增/自减）
func (h *CommandHandler) handleHIncrBy(args []protocol.Value) protocol.Value {
	// 校验参数：HINCRBY hashKey field delta（3个参数，delta为整数）
	if len(args) != 3 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString || args[2].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hincrby' command"}
	}
	hashKey := args[0].Str
	field := args[1].Str
	deltaStr := args[2].Str

	// 解析 delta 为 int64
	delta, err := strconv.ParseInt(deltaStr, 10, 64)
	if err != nil {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR value is not an integer or out of range"}
	}

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 执行自增操作
	newVal, err := h.server.GetHashTableStore().HIncrBy(hashKey, field, delta)
	if err != nil {
		return protocol.Value{Type: protocol.TypeError, Str: err.Error()}
	}

	return protocol.Value{Type: protocol.TypeInteger, Int: newVal}
}

// handleHMGet 处理 HMGET 命令（批量获取多个字段）
func (h *CommandHandler) handleHMGet(args []protocol.Value) protocol.Value {
	// 校验参数：HMGET hashKey field1 field2 ...（至少2个参数）
	if len(args) < 2 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hmget' command"}
	}
	hashKey := args[0].Str
	fields := make([]string, 0, len(args)-1)

	// 解析字段，校验类型
	for i := 1; i < len(args); i++ {
		if args[i].Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid field type for 'hmget' command"}
		}
		fields = append(fields, args[i].Str)
	}

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 批量获取字段值
	values := h.server.GetHashTableStore().HMGet(hashKey, fields...)

	// 构建 RESP 数组响应（不存在的字段返回 Null BulkString）
	respArray := make([]protocol.Value, len(values))
	for i, val := range values {
		if val == nil {
			respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Null: true}
		} else {
			respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Str: val.(string)}
		}
	}

	return protocol.Value{Type: protocol.TypeArray, Array: respArray}
}

// handleHKeys 处理 HKEYS 命令（获取所有字段名）
func (h *CommandHandler) handleHKeys(args []protocol.Value) protocol.Value {
	// 校验参数：HKEYS hashKey（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hkeys' command"}
	}
	hashKey := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 获取所有字段名
	keys := h.server.GetHashTableStore().HKeys(hashKey)

	// 构建 RESP 数组响应
	respArray := make([]protocol.Value, len(keys))
	for i, key := range keys {
		respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Str: key}
	}

	return protocol.Value{Type: protocol.TypeArray, Array: respArray}
}

// handleHVals 处理 HVALS 命令（获取所有字段值）
func (h *CommandHandler) handleHVals(args []protocol.Value) protocol.Value {
	// 校验参数：HVALS hashKey（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hvals' command"}
	}
	hashKey := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 获取所有字段值
	vals := h.server.GetHashTableStore().HVals(hashKey)

	// 构建 RESP 数组响应
	respArray := make([]protocol.Value, len(vals))
	for i, val := range vals {
		respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Str: val.(string)}
	}

	return protocol.Value{Type: protocol.TypeArray, Array: respArray}
}

// handleHDel 处理 HDEL 命令（单字段删除）
func (h *CommandHandler) handleHDel(args []protocol.Value) protocol.Value {
	// 校验参数：HDEL hashKey field（2个BulkString）
	if len(args) < 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hdel' command"}
	}
	hashKey := args[0].Str
	field := args[1].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	fmt.Println(hashKey)
	fmt.Println(field)
	// 执行删除
	ok := h.server.GetHashTableStore().HDel(hashKey, field)
	if ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 成功返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 失败返回0
}

// handleHDelMulti 处理 HDELMULTI 命令（批量删除多个字段）
func (h *CommandHandler) handleHDelMulti(args []protocol.Value) protocol.Value {
	// 校验参数：HDELMULTI hashKey field1 field2 ...（至少2个参数）
	if len(args) < 2 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hdelmulti' command"}
	}
	hashKey := args[0].Str
	fields := make([]string, 0, len(args)-1)

	// 解析字段，校验类型
	for i := 1; i < len(args); i++ {
		if args[i].Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid field type for 'hdelmulti' command"}
		}
		fields = append(fields, args[i].Str)
	}

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 批量删除，返回删除数量
	deletedCount := h.server.GetHashTableStore().HDelMulti(hashKey, fields...)
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(deletedCount)}
}

// handleHClear 处理 HCLEAR 命令（清空指定哈希表所有字段）
func (h *CommandHandler) handleHClear(args []protocol.Value) protocol.Value {
	// 校验参数：HCLEAR hashKey（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hclear' command"}
	}
	hashKey := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 清空哈希表，返回清空数量
	clearedCount := h.server.GetHashTableStore().HClear(hashKey)
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(clearedCount)}
}

// handleHDelHash 处理 HDELHASH 命令（删除整个哈希表）
func (h *CommandHandler) handleHDelHash(args []protocol.Value) protocol.Value {
	// 校验参数：HDELHASH hashKey（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hdelhash' command"}
	}
	hashKey := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 删除整个哈希表，返回是否存在
	exists := h.server.GetHashTableStore().HDelHash(hashKey)
	if exists {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 存在并删除返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 不存在返回0
}

// handleHashExists 处理 HASEXISTS 命令（检查哈希表是否存在）
func (h *CommandHandler) handleHashExists(args []protocol.Value) protocol.Value {
	// 校验参数：HASEXISTS hashKey（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hasexists' command"}
	}
	hashKey := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 检查哈希表是否存在
	exists := h.server.GetHashTableStore().HashExists(hashKey)
	if exists {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 存在返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 不存在返回0
}

// handleHashCount 处理 HASHCOUNT 命令（获取所有独立哈希表数量）
func (h *CommandHandler) handleHashCount(args []protocol.Value) protocol.Value {
	// 校验参数：HASHCOUNT 无参数
	if len(args) != 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hashcount' command"}
	}

	// 集群模式下：仅统计当前节点的哈希表
	count := h.server.GetHashTableStore().HashCount()
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(count)}
}

// handleHStrLen 处理 HSTRLEN 命令（获取字段值的字符串长度）
func (h *CommandHandler) handleHStrLen(args []protocol.Value) protocol.Value {
	// 校验参数：HSTRLEN hashKey field（2个BulkString）
	if len(args) != 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hstrlen' command"}
	}
	hashKey := args[0].Str
	field := args[1].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	// 获取字段值长度
	length, err := h.server.GetHashTableStore().HStrLen(hashKey, field)
	if err != nil {
		return protocol.Value{Type: protocol.TypeError, Str: err.Error()}
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(length)}
}

// -------------------------- 原有哈希表命令优化（修复bug） --------------------------
func (h *CommandHandler) handleHSet(args []protocol.Value) protocol.Value {
	if len(args) < 3 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString || args[2].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hset' command"}
	}
	hashKey := args[0].Str
	key := args[1].Str
	value := args[2].Str

	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	h.server.GetHashTableStore().HSet(hashKey, key, value)
	return protocol.Value{Type: protocol.TypeSimpleString, Str: "OK"}
}

// 优化 handleHGet：修复错误判断逻辑
func (h *CommandHandler) handleHGet(args []protocol.Value) protocol.Value {
	if len(args) != 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hget' command"}
	}
	hashKey := args[0].Str
	field := args[1].Str

	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	val, err := h.server.GetHashTableStore().HGet(hashKey, field)
	if err != nil || val == nil {
		return protocol.Value{Type: protocol.TypeBulkString, Null: true}
	}
	strVal, ok := val.(string)
	if !ok {
		return protocol.Value{Type: protocol.TypeBulkString, Str: fmt.Sprintf("%v", val)}
	}
	return protocol.Value{Type: protocol.TypeBulkString, Str: strVal}
}

// 优化 handleHGetAll：返回 RESP 数组格式（符合 Redis 标准）
func (h *CommandHandler) handleHGetAll(args []protocol.Value) protocol.Value {
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'hgetall' command"}
	}
	hashKey := args[0].Str

	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hash key '%s' belongs to slot %d, which is not handled by this node.go", hashKey, slot)}
		}
	}

	hashTable := h.server.GetHashTableStore().HGetAll(hashKey)

	// 符合 Redis 标准：返回 "field1", "value1", "field2", "value2" 格式的数组
	respArray := make([]protocol.Value, 0, len(hashTable)*2)
	for field, value := range hashTable {
		respArray = append(respArray, protocol.Value{Type: protocol.TypeBulkString, Str: field})
		respArray = append(respArray, protocol.Value{Type: protocol.TypeBulkString, Str: value.(string)})
	}

	return protocol.Value{Type: protocol.TypeArray, Array: respArray}
}

// handleHLen 处理 HLEN 命令（获取哈希表的字段数量）
// 命令格式：HLEN hashKey
// 功能：返回指定哈希表中存在的字段总数，哈希表不存在则返回 0
func (h *CommandHandler) handleHLen(args []protocol.Value) protocol.Value {
	// 1. 参数校验：仅需 1 个 BulkString 类型的 hashKey 参数
	if len(args) != 1 {
		return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR wrong number of arguments for 'hlen' command: expected 1, got %d", len(args))}
	}
	if args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR hashKey must be a bulk string for 'hlen' command"}
	}

	hashKey := args[0].Str

	// 2. 集群模式：检查 hashKey 对应的槽位是否归当前节点所有
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR slot %d for hashKey '%s' has no owner", slot, hashKey)}
		}
		if ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hashKey '%s' belongs to slot %d (owner: %s), which is not handled by this node.go", hashKey, slot, ownerID)}
		}
	}

	// 3. 调用存储层获取字段数量（原子操作，存储层保证并发安全）
	fieldCount := h.server.GetHashTableStore().HLen(hashKey)

	// 4. 返回结果：整数类型（符合 Redis HLEN 响应格式）
	// 哈希表不存在或无字段时返回 0，存在字段则返回实际数量
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(fieldCount)}
}

// handleHExists 处理 HEXISTS 命令（检查哈希表中指定字段是否存在）
// 命令格式：HEXISTS hashKey field
// 功能：存在返回 1，不存在（字段不存在或哈希表不存在）返回 0
func (h *CommandHandler) handleHExists(args []protocol.Value) protocol.Value {
	// 1. 参数校验：必须传入 2 个 BulkString 类型参数（hashKey + field）
	if len(args) != 2 {
		return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR wrong number of arguments for 'hexists' command: expected 2, got %d", len(args))}
	}
	if args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR hashKey must be a bulk string for 'hexists' command"}
	}
	if args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR field must be a bulk string for 'hexists' command"}
	}

	hashKey := args[0].Str
	field := args[1].Str

	// 2. 集群模式：检查 hashKey 对应的槽位是否归当前节点所有
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(hashKey)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR slot %d for hashKey '%s' has no owner", slot, hashKey)}
		}
		if ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR hashKey '%s' belongs to slot %d (owner: %s), which is not handled by this node.go", hashKey, slot, ownerID)}
		}
	}

	// 3. 调用存储层检查字段是否存在（原子操作，存储层保证并发安全）
	exists := h.server.GetHashTableStore().HExists(hashKey, field)

	// 4. 返回结果：1=存在，0=不存在（符合 Redis HEXISTS 响应格式）
	return protocol.Value{Type: protocol.TypeInteger, Int: map[bool]int64{true: 1, false: 0}[exists]}
}
