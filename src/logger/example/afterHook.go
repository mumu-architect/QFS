package main

import (
	"fmt"
	"time"

	"logger" // 替换为你的日志框架路径
)

// 测试用后置 Hook
type TestPostHook struct {
	name string
}

func (h *TestPostHook) Levels() []logger.Level {
	// 匹配所有级别，确保能被触发
	return []logger.Level{
		logger.DebugLevel,
		logger.InfoLevel,
		logger.ErrorLevel,
	}
}

func (h *TestPostHook) FireAfter(entry *logger.Entry) error {
	fmt.Printf("[后置Hook-%s] 已执行，日志: %s %s\n",
		h.name, entry.Level, entry.Message)
	//如果日志级别是ERROR就把错误消息发邮件给管理员
	return nil
}

func main() {
	// 配置日志器，启用同步/异步模式均可测试
	cfg := logger.Config{
		Level: logger.DebugLevel,
		OutputPaths: []string{
			"stdout",
			"logs/app.log",
		},

		// 【文件大小轮转核心参数】
		RotationPolicy: logger.RotationPolicySize, // 仅按大小轮转
		MaxSize:        1,                         // 单个文件最大尺寸（单位：MB）
		MaxBackups:     10,                        // 最多保留10个备份文件
		MaxAge:         30,                        // 备份文件保留30天
		Compress:       true,                      // 压缩超过MaxAge的备份文件（.gz）

		// 可选：启用异步写入提升性能
		Async:       true,
		AsyncBuffer: 1000,
		PostHooks: []logger.PostHook{
			&TestPostHook{name: "测试1"},
			&TestPostHook{name: "测试2"},
		},
	}

	log, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}
	defer log.Close()

	// 输出日志，触发后置 Hook
	log.Info("这是一条测试日志")
	log.Error("这是一条错误日志")
	// 批量写入日志测试统计效果
	for i := 0; i < 2000; i++ {
		log.Info("普通操作日志")
		if i%10 == 0 {
			log.Error("错误日志")
		}
	}

	// 异步模式下需要等待片刻，确保后台协程执行完成
	time.Sleep(100 * time.Millisecond)
}
