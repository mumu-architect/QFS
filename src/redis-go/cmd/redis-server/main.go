package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mumu.com/config"
	"mumu.com/redis-go/internal/server"
)

func main() {
	// 1. 解析命令行参数
	configPath := flag.String("config", "configs/redis-cluster.yaml", "Path to the configuration file")
	flag.Parse()

	// 2. 加载配置文件，如果不存在则自动创建并使用默认配置
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("Failed to load or create configuration: %v", err)
	}

	// 3. 创建服务器实例
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// 4. 启动服务器
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// 5. 设置信号处理，实现优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞等待退出信号
	<-sigChan

	// 6. 关闭服务器
	srv.Shutdown()
	log.Println("Server has been shut down.")
}
