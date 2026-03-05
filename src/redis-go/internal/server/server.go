package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"mumu.com/config"
	"mumu.com/redis-go/internal/cluster"
	"mumu.com/redis-go/internal/db"
	"mumu.com/redis-go/internal/protocol"
)

// Server 是整个服务的核心结构体，整合了配置、数据库、集群等所有组件
type Server struct {
	config         *config.Config
	listener       net.Listener
	db             *db.MemoryDB
	hashTableStore *db.HashTable //新增hashtable 2025-09-28
	cluster        *cluster.ClusterState
	gossip         *cluster.Gossip
	shutdown       chan struct{}
	wg             sync.WaitGroup
	cmdHandler     *CommandHandler
}

// NewServer 创建一个新的服务器实例
func NewServer(cfg *config.Config) (*Server, error) {
	s := &Server{
		config:         cfg,
		db:             db.NewMemoryDB(),
		hashTableStore: db.NewHashTable(16, 0.75), //新增hashtable 2025-09-28
		shutdown:       make(chan struct{}),
	}

	// 初始化集群状态
	if cfg.Server.NodeID == "" {
		// 为了演示，我们从地址生成一个简单的ID
		cfg.Server.NodeID = generateNodeIDFromAddr(cfg.Server.Address)
	}
	s.cluster = cluster.NewClusterState(cfg.Server.NodeID, cfg.Server.Address)

	// 初始化命令处理器
	s.cmdHandler = NewCommandHandler(s)

	// 如果启用集群模式，启动Gossip协议
	if cfg.Server.ClusterMode {
		s.gossip = cluster.NewGossip(s.cluster, cfg.Cluster.GossipInterval, cfg.Cluster.NodeTimeout)
	}

	return s, nil
}

// generateNodeIDFromAddr 从地址生成一个简单的节点ID，仅用于演示
func generateNodeIDFromAddr(addr string) string {
	clean := strings.ReplaceAll(addr, ":", "")
	clean = strings.ReplaceAll(clean, ".", "")
	if len(clean) > 16 {
		clean = clean[:16]
	}
	return clean
}

// Start 启动服务器，开始监听端口并接受连接
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.Server.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Server.Address, err)
	}
	s.listener = listener
	fmt.Printf("Server listening on %s...\n", s.config.Server.Address)

	if s.config.Server.ClusterMode {
		fmt.Println("Cluster mode is enabled. Starting Gossip protocol...")
		s.gossip.Start()
	}

	// 启动一个goroutine来接受连接，避免阻塞主goroutine
	go s.acceptLoop()
	return nil
}

// acceptLoop 循环接受客户端连接
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				fmt.Println("Server is shutting down. Accept loop closed.")
			default:
				fmt.Printf("Accept error: %v\n", err)
			}
			return
		}
		s.wg.Add(1)
		// 为每个连接启动一个独立的goroutine进行处理
		go s.handleConnection(conn)
	}
}

// handleConnection 处理单个客户端连接的生命周期
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("New connection from %s\n", remoteAddr)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	decoder := protocol.NewDecoder(reader)
	encoder := protocol.NewEncoder(writer)

	// 设置初始超时
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	for {
		value, err := decoder.Decode()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "read timeout") {
				fmt.Printf("Connection from %s closed or timed out.\n", remoteAddr)
			} else {
				fmt.Printf("Decode error from %s: %v\n", remoteAddr, err)
				_ = encoder.Encode(protocol.Value{Type: protocol.TypeError, Str: "ERR invalid protocol format"})
				_ = encoder.Flush()
			}
			break
		}

		// 刷新读超时
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// 处理命令并获取响应
		response := s.cmdHandler.HandleCommand(value)

		// 发送响应给客户端
		if err := encoder.Encode(response); err != nil {
			fmt.Printf("Encode error to %s: %v\n", remoteAddr, err)
			break
		}
		if err := encoder.Flush(); err != nil {
			fmt.Printf("Flush error to %s: %v\n", remoteAddr, err)
			break
		}

		// 刷新写超时
		conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}
}

// Shutdown 优雅地关闭服务器
func (s *Server) Shutdown() {
	close(s.shutdown)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	fmt.Println("Server gracefully shut down.")
}

// GetDB 返回服务器的内存数据库实例
func (s *Server) GetDB() *db.MemoryDB {
	return s.db
}

// GetClusterState 返回服务器的集群状态实例
func (s *Server) GetClusterState() *cluster.ClusterState {
	return s.cluster
}

// IsClusterMode 返回服务器是否处于集群模式
func (s *Server) IsClusterMode() bool {
	return s.config.Server.ClusterMode
}

// 新增hashtable 2025-09-28
// GetHashTableStore 返回服务器的内存数据库实例
func (s *Server) GetHashTableStore() *db.HashTable {
	return s.hashTableStore
}
