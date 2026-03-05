package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"mumu.com/config"
	"mumu.com/redis-go/distributedId/snowflake"
)

// -------------------------- 5. 测试代码 --------------------------
func main() {
	// 1. 解析命令行参数
	configPath := flag.String("config", "configs/redis-cluster.yaml", "Path to the configuration file")
	flag.Parse()

	// 2. 加载配置文件，如果不存在则自动创建并使用默认配置
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("Failed to load or create configuration: %v", err)
	}
	// 自定义起始时间：2024-01-01 00:00:00（毫秒时间戳）
	epoch := time.Date(2025, 11, 1, 1, 1, 1, 1, time.UTC).Unix()
	fmt.Printf("Failed to load or create configuration: %v|%v|%v \r\n", cfg.Server.CenterId, cfg.Server.WorkerId, epoch)
	// 初始化雪花实例（数据中心ID=1，机器ID=3）
	sf, err := snowflake.NewSnowflake(cfg.Server.CenterId, cfg.Server.WorkerId, epoch)
	if err != nil {
		fmt.Printf("初始化失败：%v\n", err)
		return
	}

	// 多协程测试：验证无锁安全与ID唯一性
	var wg sync.WaitGroup
	idSet := make(map[int64]struct{}) // 存储生成的ID，用于检测重复
	var mu sync.Mutex                 // 保护map的并发读写（仅测试用）

	// 启动10个协程，每个协程生成200个ID
	for worker := 0; worker < 10; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				id, err := sf.NextID()
				if err != nil {
					fmt.Printf("协程%d生成ID失败：%v\n", workerID, err)
					return
				}
				// 检查ID是否重复
				mu.Lock()
				if _, exists := idSet[id]; exists {
					fmt.Printf("警告：协程%d生成重复ID：%d\n", workerID, id)
				}
				idSet[id] = struct{}{}
				mu.Unlock()
				// 可选：打印ID（高并发时可注释，避免输出混乱）
				// fmt.Printf("协程%d生成ID：%d\n", workerID, id)
			}
		}(worker)
	}

	wg.Wait()

	id, err := sf.NextID()

	fmt.Printf("生成ID：%v\n", id)
	id, err = sf.NextID()

	fmt.Printf("生成ID：%v\n", id)

	fmt.Printf("测试完成：共生成%d个唯一ID（预期2000个）\n", len(idSet))
}
