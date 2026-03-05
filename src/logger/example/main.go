package main

import (
	"logger"
)

func main1() {
	// 创建一个默认的日志器
	log, err := logger.New(logger.Config{})
	if err != nil {
		panic(err)
	}
	defer log.Close()

	log.Info("程序启动成功")
	log.Debug("这是调试信息") // 默认级别为Info，此条不会输出
	log.Errorf("数据库连接失败: %v", "timeout")
}
