package server

import (
	"fmt"
	"strings"

	"mumu.com/redis-go/internal/protocol"
)

// CommandHandler 负责解析和执行具体的Redis命令
type CommandHandler struct {
	server *Server
}

// NewCommandHandler 创建一个新的命令处理器
func NewCommandHandler(srv *Server) *CommandHandler {
	return &CommandHandler{
		server: srv,
	}
}

// CommandHandler 保持原有结构，新增哈希表命令分发
func (h *CommandHandler) HandleCommand(value protocol.Value) protocol.Value {
	fmt.Printf("%s,%s", value.Type, protocol.TypeArray)
	if value.Type != protocol.TypeArray {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR invalid command format"}
	}
	if len(value.Array) == 0 {
		return protocol.Value{Type: protocol.TypeError, Str: "ERR empty command"}
	}
	cmd := strings.ToUpper(value.Array[0].Str)
	args := value.Array[1:]
	fmt.Printf("cmd: %v,\n", cmd)
	fmt.Printf("args: %v,\n", args)
	switch cmd {
	// 原有命令（保持不变）
	case "PING":
		return h.handlePing(args)
	case "SET", "SETNX", "SETXX", "MSET", "MSETNX", "GET", "GETSET", "MGET", "GETRANGE", "STRLEN", "DEL", "EXISTS", "CLUSTER":
		// 沿用之前实现的命令
		return h.dispatchOriginalCommands(cmd, args)
	// 原有哈希表命令
	case "HSET", "HGET", "HGETALL":
		return h.dispatchHashCommands(cmd, args)
	// 新增哈希表命令
	case "HLEN", "HSETNX", "HSETMULTI", "HINCRBY", "HMGET", "HKEYS", "HVALS", "HDEL", "HDELMULTI", "HCLEAR", "HDELHASH", "HASEXISTS", "HEXISTS", "HASHCOUNT", "HSTRLEN":
		return h.dispatchHashCommands(cmd, args)
	default:
		return protocol.Value{Type: protocol.TypeError, Str: "ERR unknown command '" + cmd + "'"}
	}
}

// dispatchOriginalCommands 分发原有命令（避免代码冗余）
func (h *CommandHandler) dispatchOriginalCommands(cmd string, args []protocol.Value) protocol.Value {
	switch cmd {
	case "SET":
		return h.handleSet(args)
	case "SETNX":
		return h.handleSetNX(args)
	case "SETXX":
		return h.handleSetXX(args)
	case "MSET":
		return h.handleMSet(args)
	case "MSETNX":
		return h.handleMSetNX(args)
	case "GET":
		return h.handleGet(args)
	case "GETSET":
		return h.handleGetSet(args)
	case "MGET":
		return h.handleMGet(args)
	case "GETRANGE":
		return h.handleGetRange(args)
	case "STRLEN":
		return h.handleStrLen(args)
	case "DEL":
		return h.handleDel(args)
	case "EXISTS":
		return h.handleExists(args)
	case "CLUSTER":
		return h.handleCluster(args)
	default:
		return protocol.Value{Type: protocol.TypeError, Str: "ERR unknown command '" + cmd + "'"}
	}
}

// dispatchHashCommands 分发哈希表命令（原有+新增）
func (h *CommandHandler) dispatchHashCommands(cmd string, args []protocol.Value) protocol.Value {
	switch cmd {
	case "HSET":
		return h.handleHSet(args)
	case "HGET":
		return h.handleHGet(args)
	case "HGETALL":
		return h.handleHGetAll(args)
	case "HLEN":
		return h.handleHLen(args)
	case "HSETNX":
		return h.handleHSetNX(args)
	case "HSETMULTI":
		return h.handleHSetMulti(args)
	case "HINCRBY":
		return h.handleHIncrBy(args)
	case "HMGET":
		return h.handleHMGet(args)
	case "HKEYS":
		return h.handleHKeys(args)
	case "HVALS":
		return h.handleHVals(args)
	case "HDEL":
		return h.handleHDel(args)
	case "HDELMULTI":
		return h.handleHDelMulti(args)
	case "HCLEAR":
		return h.handleHClear(args)
	case "HDELHASH":
		return h.handleHDelHash(args)
	case "HASEXISTS":
		return h.handleHashExists(args)
	case "HEXISTS":
		return h.handleHExists(args)
	case "HASHCOUNT":
		return h.handleHashCount(args)
	case "HSTRLEN":
		return h.handleHStrLen(args)
	default:
		return protocol.Value{Type: protocol.TypeError, Str: "ERR unknown hash command '" + cmd + "'"}
	}
}
