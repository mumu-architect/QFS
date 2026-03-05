package server

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"mumu.com/redis-go/internal/cluster"
	"mumu.com/redis-go/internal/protocol"
)

// -------------------------- 原有命令实现（保持不变） --------------------------
func (h *CommandHandler) handlePing(args []protocol.Value) protocol.Value {
	if len(args) == 0 {
		return protocol.Value{Type: protocol.TypeSimpleString, Str: "PONG"}
	}
	if len(args) == 1 && args[0].Type == protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeBulkString, Str: args[0].Str}
	}
	return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'ping' command"}
}

func (h *CommandHandler) handleSet(args []protocol.Value) protocol.Value {
	if len(args) < 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'set' command"}
	}
	key := args[0].Str
	value := args[1].Str

	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	h.server.GetDB().Set(key, value)
	return protocol.Value{Type: protocol.TypeSimpleString, Str: "OK"}
}

func (h *CommandHandler) handleGet(args []protocol.Value) protocol.Value {
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'get' command"}
	}
	key := args[0].Str

	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	val, ok := h.server.GetDB().Get(key)
	if !ok {
		return protocol.Value{Type: protocol.TypeBulkString, Null: true}
	}
	return protocol.Value{Type: protocol.TypeBulkString, Str: val}
}

func (h *CommandHandler) handleDel(args []protocol.Value) protocol.Value {
	if len(args) == 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'del' command"}
	}
	deleted := 0
	for _, arg := range args {
		if arg.Type != protocol.TypeBulkString {
			continue
		}
		key := arg.Str
		if h.server.GetDB().KeyExists(key) {
			h.server.GetDB().Del(key)
			deleted++
		}
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(deleted)}
}

func (h *CommandHandler) handleExists(args []protocol.Value) protocol.Value {
	if len(args) == 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'exists' command"}
	}
	count := 0
	for _, arg := range args {
		if arg.Type != protocol.TypeBulkString {
			continue
		}
		if h.server.GetDB().KeyExists(arg.Str) {
			count++
		}
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(count)}
}

func (h *CommandHandler) handleCluster(args []protocol.Value) protocol.Value {
	if !h.server.IsClusterMode() {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR cluster mode is disabled"}
	}
	if len(args) == 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'cluster' command"}
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "MEET":
		if len(args) < 3 {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'cluster meet' command"}
		}
		nodeID := args[1].Str
		nodeAddr := args[2].Str
		h.server.GetClusterState().MeetNode(nodeID, nodeAddr)
		return protocol.Value{Type: protocol.TypeSimpleString, Str: "OK"}
	case "NODES":
		return protocol.Value{Type: protocol.TypeSimpleString, Str: "This is a simplified cluster nodes response"}
	case "INFO":
		return protocol.Value{Type: protocol.TypeSimpleString, Str: "This is a simplified cluster info response"}
	default:
		return protocol.Value{Type: protocol.TypeError, Str: "ERR unknown cluster subcommand '" + subCmd + "'"}
	}
}

// -------------------------- 新增命令实现 --------------------------

// handleSetNX 处理 SETNX 命令（键不存在时设置）
func (h *CommandHandler) handleSetNX(args []protocol.Value) protocol.Value {
	// 校验参数：必须2个BulkString（key + value）
	if len(args) != 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'setnx' command"}
	}
	key := args[0].Str
	value := args[1].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	// 原子执行 SETNX
	ok := h.server.GetDB().SetNX(key, value)
	if ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 成功返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 失败返回0
}

// handleSetXX 处理 SETXX 命令（键存在时更新）
func (h *CommandHandler) handleSetXX(args []protocol.Value) protocol.Value {
	// 校验参数：必须2个BulkString（key + value）
	if len(args) != 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'setxx' command"}
	}
	key := args[0].Str
	value := args[1].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	// 原子执行 SETXX
	ok := h.server.GetDB().SetXX(key, value)
	if ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 成功返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 失败返回0
}

// handleMSet 处理 MSET 命令（批量设置，原子操作）
func (h *CommandHandler) handleMSet(args []protocol.Value) protocol.Value {
	// 校验参数：必须是偶数个BulkString（key1,value1,key2,value2...）
	if len(args) == 0 || len(args)%2 != 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'mset' command"}
	}
	kv := make(map[string]string)
	allKeys := make([]string, 0, len(args)/2)

	// 解析键值对并校验类型
	for i := 0; i < len(args); i += 2 {
		keyVal := args[i]
		valueVal := args[i+1]
		if keyVal.Type != protocol.TypeBulkString || valueVal.Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid argument type for 'mset' command"}
		}
		key := keyVal.Str
		value := valueVal.Str
		kv[key] = value
		allKeys = append(allKeys, key)
	}

	// 集群模式检查：所有键必须属于当前节点（Redis MSET 不支持跨节点批量操作）
	if h.server.IsClusterMode() {
		for _, key := range allKeys {
			slot := cluster.SlotForKey(key)
			ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
			if !ok || ownerID != h.server.GetClusterState().Self.ID {
				return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
			}
		}
	}

	// 原子执行批量设置
	h.server.GetDB().MSet(kv)
	return protocol.Value{Type: protocol.TypeSimpleString, Str: "OK"}
}

// handleMSetNX 处理 MSETNX 命令（所有键不存在才批量设置）
func (h *CommandHandler) handleMSetNX(args []protocol.Value) protocol.Value {
	// 校验参数：必须是偶数个BulkString
	if len(args) == 0 || len(args)%2 != 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'msetnx' command"}
	}
	kv := make(map[string]string)
	allKeys := make([]string, 0, len(args)/2)

	// 解析键值对并校验类型
	for i := 0; i < len(args); i += 2 {
		keyVal := args[i]
		valueVal := args[i+1]
		if keyVal.Type != protocol.TypeBulkString || valueVal.Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid argument type for 'msetnx' command"}
		}
		key := keyVal.Str
		value := valueVal.Str
		kv[key] = value
		allKeys = append(allKeys, key)
	}

	// 集群模式检查：所有键必须属于当前节点
	if h.server.IsClusterMode() {
		for _, key := range allKeys {
			slot := cluster.SlotForKey(key)
			ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
			if !ok || ownerID != h.server.GetClusterState().Self.ID {
				return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
			}
		}
	}

	// 原子执行 MSetNX
	ok := h.server.GetDB().MSetNX(kv)
	if ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 1} // 成功返回1
	}
	return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 失败返回0
}

// -------------------------- 新增缺失的四个命令实现 --------------------------

// handleGetSet 处理 GETSET 命令（原子替换，设置新值并返回旧值）
func (h *CommandHandler) handleGetSet(args []protocol.Value) protocol.Value {
	// 校验参数：GETSET key value（2个BulkString）
	if len(args) != 2 || args[0].Type != protocol.TypeBulkString || args[1].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'getset' command"}
	}
	key := args[0].Str
	newValue := args[1].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	// 原子执行 GetSet（存储层保证原子性）
	oldVal, exists := h.server.GetDB().GetSet(key, newValue)
	if !exists {
		// 键不存在时，返回 Null BulkString（符合 Redis 行为）
		return protocol.Value{Type: protocol.TypeBulkString, Null: true}
	}
	return protocol.Value{Type: protocol.TypeBulkString, Str: oldVal}
}

// handleMGet 处理 MGET 命令（批量获取多键值对）
func (h *CommandHandler) handleMGet(args []protocol.Value) protocol.Value {
	// 校验参数：MGET key1 key2 ...（至少1个BulkString）
	if len(args) == 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'mget' command"}
	}
	keys := make([]string, 0, len(args))

	// 解析所有键并校验类型
	for _, arg := range args {
		if arg.Type != protocol.TypeBulkString {
			return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid key type for 'mget' command"}
		}
		keys = append(keys, arg.Str)
	}

	// 集群模式检查：所有键必须属于当前节点（Redis MGET 不支持跨节点）
	if h.server.IsClusterMode() {
		for _, key := range keys {
			slot := cluster.SlotForKey(key)
			ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
			if !ok || ownerID != h.server.GetClusterState().Self.ID {
				return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
			}
		}
	}

	// 批量获取值（存储层保证原子性）
	values := h.server.GetDB().MGet(keys...)

	// 构建 RESP 数组响应：不存在的键返回 Null BulkString
	respArray := make([]protocol.Value, len(values))
	for i, val := range values {
		if val == "" {
			respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Null: true}
		} else {
			respArray[i] = protocol.Value{Type: protocol.TypeBulkString, Str: val}
		}
	}

	return protocol.Value{Type: protocol.TypeArray, Array: respArray}
}

// handleGetRange 处理 GETRANGE 命令（截取字符串子串）
func (h *CommandHandler) handleGetRange(args []protocol.Value) protocol.Value {
	// 校验参数：GETRANGE key start end（3个参数，start/end 为整数）
	if len(args) != 3 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'getrange' command"}
	}
	key := args[0].Str
	startStr := args[1].Str
	endStr := args[2].Str

	// 解析 start 和 end 为整数
	start, err := strconv.Atoi(startStr)
	if err != nil {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR start index is not an integer"}
	}
	end, err := strconv.Atoi(endStr)
	if err != nil {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR end index is not an integer"}
	}

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	// 获取原始值并截取子串
	val, ok := h.server.GetDB().Get(key)
	if !ok {
		return protocol.Value{Type: protocol.TypeBulkString, Str: ""} // 键不存在返回空字符串
	}

	// 支持 UTF-8 多字节字符（如中文），基于 rune 处理
	runes := []rune(val)
	length := len(runes)
	if length == 0 {
		return protocol.Value{Type: protocol.TypeBulkString, Str: ""}
	}

	// 处理负索引（负数表示倒数）
	if start < 0 {
		start += length
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += length
		if end < 0 {
			return protocol.Value{Type: protocol.TypeBulkString, Str: ""}
		}
	}

	// 处理索引超出范围
	if start >= length {
		return protocol.Value{Type: protocol.TypeBulkString, Str: ""}
	}
	if end >= length {
		end = length - 1
	}
	if start > end {
		return protocol.Value{Type: protocol.TypeBulkString, Str: ""}
	}

	// 截取并返回子串
	subStr := string(runes[start : end+1])
	return protocol.Value{Type: protocol.TypeBulkString, Str: subStr}
}

// handleStrLen 处理 STRLEN 命令（获取字符串字符长度）
func (h *CommandHandler) handleStrLen(args []protocol.Value) protocol.Value {
	// 校验参数：STRLEN key（1个BulkString）
	if len(args) != 1 || args[0].Type != protocol.TypeBulkString {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR wrong number of arguments for 'strlen' command"}
	}
	key := args[0].Str

	// 集群模式槽位检查
	if h.server.IsClusterMode() {
		slot := cluster.SlotForKey(key)
		ownerID, ok := h.server.GetClusterState().GetSlotOwner(slot)
		if !ok || ownerID != h.server.GetClusterState().Self.ID {
			return protocol.Value{Type: protocol.TypeError, Str: fmt.Sprintf("ERR key '%s' belongs to slot %d, which is not handled by this node.go", key, slot)}
		}
	}

	// 获取值并计算字符长度（支持 UTF-8 多字节字符）
	val, ok := h.server.GetDB().Get(key)
	if !ok {
		return protocol.Value{Type: protocol.TypeInteger, Int: 0} // 键不存在返回 0
	}

	// 用 utf8.RuneCountInString 计算字符数（而非字节数）
	length := utf8.RuneCountInString(val)
	return protocol.Value{Type: protocol.TypeInteger, Int: int64(length)}
}
