package main

import (
	"context"
	"logger"

	"go.opentelemetry.io/otel"
)

func main2() {
	log, _ := logger.New(logger.Config{
		Format:       "json",
		ReportCaller: true,
	})
	defer log.Close()

	// 模拟分布式追踪上下文（实际由框架注入，如 HTTP 中间件）
	tracer := otel.Tracer("demo")
	ctx, span := tracer.Start(context.Background(), "main")
	defer span.End()

	// 使用带 Context 的日志方法
	log.InfofContext(ctx, "处理用户请求", logger.Fields{"user_id": 1001})
	processOrder(ctx, 2001)
}

func processOrder(ctx context.Context, orderID int) {
	log, _ := logger.New(logger.Config{Format: "json"})
	log.DebugContext(ctx, "订单处理开始", logger.Fields{"order_id": orderID})
	// 业务逻辑...
	log.InfoContext(ctx, "订单处理完成", logger.Fields{"order_id": orderID, "status": "success"})
}
