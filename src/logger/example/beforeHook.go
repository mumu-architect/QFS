package main

import "logger"

// 1. 定义 Hook 实现
type ErrorAlertHook struct{}

// Levels：指定当前 Hook 关心的日志级别
func (h *ErrorAlertHook) Levels() []logger.Level {
	return []logger.Level{logger.ErrorLevel, logger.FatalLevel}
}

// Fire：日志触发时执行的逻辑
func (h *ErrorAlertHook) Fire(entry *logger.Entry) error {
	// 此处可实现：发送邮件、短信、调用告警 API 等
	println("========== 错误告警 ==========")
	println("时间:", entry.Time.Format("2006-01-02 15:04:05"))
	println("消息:", entry.Message)
	println("详情:", entry.Fields)
	println("==============================")
	return nil
}

// 2. 使用 Hook
func main5() {
	cfg := logger.Config{
		Hooks: []logger.Hook{&ErrorAlertHook{}}, // 注册 Hook
	}
	log, _ := logger.New(cfg)
	defer log.Close()

	// 触发 Hook
	log.Error("支付失败", logger.Fields{"order_id": "123456", "amount": 99.9})
}
