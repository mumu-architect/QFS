package snowflake

import (
	"flag"
	"fmt"
	"log"
	"time"

	"mumu.com/config"
)

type SnowFlakeGenerate struct {
	Snowflake *Snowflake
}

// NewSnowFlakeGenerate 初始化雪花对象
func NewSnowFlakeGenerate() *SnowFlakeGenerate {
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
	sf, err := newSnowflake(cfg.Server.CenterId, cfg.Server.WorkerId, epoch)
	if err != nil {
		fmt.Printf("初始化失败：%v\n", err)
		return nil
	}
	sfg := &SnowFlakeGenerate{
		Snowflake: sf,
	}
	return sfg
}

// GetFlowID 获取唯一流水ID
func (sfg *SnowFlakeGenerate) GetFlowID() int64 {
	id, err := sfg.Snowflake.NextID()
	if err != nil {
		fmt.Printf("生成ID失败：%v\n", err)
		return 0
	}
	return id
}
