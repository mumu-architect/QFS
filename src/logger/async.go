package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// asyncLogger 包装了一个基础 logger，并通过 channel 实现异步写入
type asyncLogger struct {
	baseLogger Logger
	entryChan  chan *Entry
	done       chan struct{}
	policy     OverflowPolicy
}

// newAsyncLogger 创建一个新的异步日志器
func newAsyncLogger(baseLogger Logger, bufferSize int, policy OverflowPolicy) Logger {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	if policy == "" {
		policy = OverflowBlock
	}
	al := &asyncLogger{
		baseLogger: baseLogger,
		entryChan:  make(chan *Entry, bufferSize),
		done:       make(chan struct{}),
		policy:     policy,
	}
	go al.startWorker()
	return al
}

// startWorker 是后台 goroutine，负责实际的日志写入 (批量)
func (al *asyncLogger) startWorker() {
	ticker := time.NewTicker(5 * time.Millisecond) // 定期批量写入的滴答器
	defer ticker.Stop()

	batch := make([]*Entry, 0, 100) // 批量缓冲区

	for {
		select {
		case entry, ok := <-al.entryChan:
			if !ok {
				// Channel 被关闭，处理剩余日志后退出
				if len(batch) > 0 {
					al.flushBatch(batch)
				}
				return
			}
			batch = append(batch, entry)
			// 当批量达到达到容量上限时，立即刷新
			if len(batch) >= cap(batch) {
				al.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			// 定期刷新，防止止日志长时间滞留
			if len(batch) > 0 {
				al.flushBatch(batch)
				batch = batch[:0]
			}
		case <-al.done:
			// 收到退出信号，处理完剩余日志后退出
			if len(batch) > 0 {
				al.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch 将批次中的日志条目写入底层的 baseLogger
func (al *asyncLogger) flushBatch(batch []*Entry) {
	if l, ok := al.baseLogger.(*logger); ok {
		l.mu.Lock()
		for _, entry := range batch {
			l.writeEntry(entry)
		}
		l.mu.Unlock()
	}
	// 将 entry 放回对象池
	for _, entry := range batch {
		putEntry(entry)
	}
}

// --- 实现 Logger 接口 ---

// Close 关闭异步日志器，确保所有日志都被处理
func (al *asyncLogger) Close() error {
	close(al.done)               // 通知 worker 退出
	close(al.entryChan)          // 关闭条目通道
	return al.baseLogger.Close() // 关闭基础 logger
}

// SetLevel 动态调整日志级别
func (al *asyncLogger) SetLevel(level Level) {
	al.baseLogger.SetLevel(level)
}

// 通用的异步日志发送函数
func (al *asyncLogger) sendToChannel(entry *Entry, fatal bool) {
	if fatal {
		// Fatal 日志必须阻塞等待，确保在退出前被处理
		al.entryChan <- entry
		al.Close() // 关闭日志器，触发 worker 退出和批次刷新
		os.Exit(1)
	}

	switch al.policy {
	case OverflowDiscard:
		select {
		case al.entryChan <- entry:
		default:
			// 缓冲区满，直接丢弃，并将 entry 放回池中
			putEntry(entry)
		}
	default: // OverflowBlock
		al.entryChan <- entry
	}
}

// 创建 Entry 并发送到 channel 的辅助函数
func (al *asyncLogger) prepareAndSend(level Level, msg string, fields Fields, fatal bool) {
	// 级别检查在异步端做一次，避免不必要的对象创建
	if l, ok := al.baseLogger.(*logger); ok && level < Level(l.level.Load()) {
		return
	}
	entry := getEntry()
	entry.Time = time.Now()
	entry.Level = level.String()
	entry.Message = msg
	if fields != nil {
		entry.Fields = fields
	}
	al.sendToChannel(entry, fatal)
}

// Debug logs a message at DebugLevel.
func (al *asyncLogger) Debug(args ...interface{}) {
	al.prepareAndSend(DebugLevel, fmt.Sprint(args...), nil, false)
}

// Debugf logs a message at DebugLevel.
func (al *asyncLogger) Debugf(format string, args ...interface{}) {
	al.prepareAndSend(DebugLevel, fmt.Sprintf(format, args...), nil, false)
}

// Info logs a message at InfoLevel.
func (al *asyncLogger) Info(args ...interface{}) {
	al.prepareAndSend(InfoLevel, fmt.Sprint(args...), nil, false)
}

// Infof logs a message at InfoLevel.
func (al *asyncLogger) Infof(format string, args ...interface{}) {
	al.prepareAndSend(InfoLevel, fmt.Sprintf(format, args...), nil, false)
}

// Warn logs a message at WarnLevel.
func (al *asyncLogger) Warn(args ...interface{}) {
	al.prepareAndSend(WarnLevel, fmt.Sprint(args...), nil, false)
}

// Warnf logs a message at WarnLevel.
func (al *asyncLogger) Warnf(format string, args ...interface{}) {
	al.prepareAndSend(WarnLevel, fmt.Sprintf(format, args...), nil, false)
}

// Error logs a message at ErrorLevel.
func (al *asyncLogger) Error(args ...interface{}) {
	al.prepareAndSend(ErrorLevel, fmt.Sprint(args...), nil, false)
}

// Errorf logs a message at ErrorLevel.
func (al *asyncLogger) Errorf(format string, args ...interface{}) {
	al.prepareAndSend(ErrorLevel, fmt.Sprintf(format, args...), nil, false)
}

// Fatal logs a message at FatalLevel and exits.
func (al *asyncLogger) Fatal(args ...interface{}) {
	al.prepareAndSend(FatalLevel, fmt.Sprint(args...), nil, true)
}

// Fatalf logs a message at FatalLevel and exits.
func (al *asyncLogger) Fatalf(format string, args ...interface{}) {
	al.prepareAndSend(FatalLevel, fmt.Sprintf(format, args...), nil, true)
}

// --- Context 版本方法实现 ---

func (al *asyncLogger) DebugContext(ctx context.Context, args ...interface{}) {
	al.debugContext(ctx, fmt.Sprint(args...))
}

func (al *asyncLogger) DebugfContext(ctx context.Context, format string, args ...interface{}) {
	al.debugContext(ctx, fmt.Sprintf(format, args...))
}

func (al *asyncLogger) InfoContext(ctx context.Context, args ...interface{}) {
	al.infoContext(ctx, fmt.Sprint(args...))
}

func (al *asyncLogger) InfofContext(ctx context.Context, format string, args ...interface{}) {
	al.infoContext(ctx, fmt.Sprintf(format, args...))
}

func (al *asyncLogger) WarnContext(ctx context.Context, args ...interface{}) {
	al.warnContext(ctx, fmt.Sprint(args...))
}

func (al *asyncLogger) WarnfContext(ctx context.Context, format string, args ...interface{}) {
	al.warnContext(ctx, fmt.Sprintf(format, args...))
}

func (al *asyncLogger) ErrorContext(ctx context.Context, args ...interface{}) {
	al.errorContext(ctx, fmt.Sprint(args...))
}

func (al *asyncLogger) ErrorfContext(ctx context.Context, format string, args ...interface{}) {
	al.errorContext(ctx, fmt.Sprintf(format, args...))
}

func (al *asyncLogger) FatalContext(ctx context.Context, args ...interface{}) {
	al.fatalContext(ctx, fmt.Sprint(args...))
}

func (al *asyncLogger) FatalfContext(ctx context.Context, format string, args ...interface{}) {
	al.fatalContext(ctx, fmt.Sprintf(format, args...))
}

// 带 context 的私有方法实现
func (al *asyncLogger) debugContext(ctx context.Context, msg string) {
	al.prepareContextEntry(ctx, DebugLevel, msg, nil, false)
}

func (al *asyncLogger) infoContext(ctx context.Context, msg string) {
	al.prepareContextEntry(ctx, InfoLevel, msg, nil, false)
}

func (al *asyncLogger) warnContext(ctx context.Context, msg string) {
	al.prepareContextEntry(ctx, WarnLevel, msg, nil, false)
}

func (al *asyncLogger) errorContext(ctx context.Context, msg string) {
	al.prepareContextEntry(ctx, ErrorLevel, msg, nil, false)
}

func (al *asyncLogger) fatalContext(ctx context.Context, msg string) {
	al.prepareContextEntry(ctx, FatalLevel, msg, nil, true)
}

// 为带 context 的日志准备条目
func (al *asyncLogger) prepareContextEntry(ctx context.Context, level Level, msg string, fields Fields, fatal bool) {
	if l, ok := al.baseLogger.(*logger); ok && level < Level(l.level.Load()) {
		return
	}

	entry := getEntry()
	entry.Time = time.Now()
	entry.Level = level.String()
	entry.Message = msg
	if fields != nil {
		entry.Fields = fields
	}

	// 从 context 提取追踪信息
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		spanCtx := span.SpanContext()
		entry.TraceID = spanCtx.TraceID().String()
		entry.SpanID = spanCtx.SpanID().String()
	}

	al.sendToChannel(entry, fatal)
}
