package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Logger 是日志记录器接口
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	DebugContext(ctx context.Context, args ...interface{})
	DebugfContext(ctx context.Context, format string, args ...interface{})
	InfoContext(ctx context.Context, args ...interface{})
	InfofContext(ctx context.Context, format string, args ...interface{})
	WarnContext(ctx context.Context, args ...interface{})
	WarnfContext(ctx context.Context, format string, args ...interface{})
	ErrorContext(ctx context.Context, args ...interface{})
	ErrorfContext(ctx context.Context, format string, args ...interface{})
	FatalContext(ctx context.Context, args ...interface{})
	FatalfContext(ctx context.Context, format string, args ...interface{})

	SetLevel(level Level)
	Close() error
}

// entryPool 是一个 Entry 对象池，用于性能优化
var entryPool = sync.Pool{
	New: func() interface{} {
		return &Entry{
			Fields: make(Fields),
		}
	},
}

// getEntry 从池中获取一个 Entry
func getEntry() *Entry {
	return entryPool.Get().(*Entry)
}

// putEntry 将 Entry 放回池中
func putEntry(e *Entry) {
	entryPool.Put(e)
}

// logger 实现了 Logger 接口
type logger struct {
	mu           sync.Mutex
	level        atomic.Int32
	formatter    Formatter
	outputs      []io.WriteCloser
	reportCaller bool
	hooks        []Hook     //前置钩子
	postHooks    []PostHook // 新增：后置钩子
}

// New 创建一个新的日志记录器
func New(cfg Config) (Logger, error) {
	l := &logger{
		reportCaller: cfg.ReportCaller,
		hooks:        cfg.Hooks,
		postHooks:    cfg.PostHooks, // 绑定后置钩子
	}
	l.level.Store(int32(cfg.Level))

	// 初始化格式化器
	if cfg.CustomFormatter != nil {
		l.formatter = cfg.CustomFormatter
	} else {
		if cfg.Format == "json" {
			l.formatter = &JSONFormatter{}
		} else {
			l.formatter = &TextFormatter{}
		}
	}

	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}

	// 初始化输出
	for _, path := range cfg.OutputPaths {
		var out io.WriteCloser
		switch path {
		case "stdout":
			out = os.Stdout
		case "stderr":
			out = os.Stderr
		default:
			rotator, err := newFileRotator(path, cfg)
			if err != nil {
				return nil, err
			}
			out = rotator
		}
		l.outputs = append(l.outputs, out)
	}

	// 如果启用异步，则返回一个 asyncLogger 包装器
	if cfg.Async {
		return newAsyncLogger(l, cfg.AsyncBuffer, cfg.OverflowPolicy), nil
	}

	return l, nil
}

// --- 核心日志逻辑 ---

func (l *logger) log(level Level, msg string, fields Fields) {
	if level < Level(l.level.Load()) {
		return
	}
	entry := getEntry()
	defer putEntry(entry)

	entry.Time = time.Now()
	entry.Level = level.String()
	entry.Message = msg
	if fields != nil {
		entry.Fields = fields
	}
	if l.reportCaller {
		entry.Caller = getCaller()
	}

	l.fireHooks(entry)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeEntry(entry)

	if level == FatalLevel {
		os.Exit(1)
	}
}

func (l *logger) logContext(ctx context.Context, level Level, msg string, fields Fields) {
	if level < Level(l.level.Load()) {
		return
	}
	entry := getEntry()
	defer putEntry(entry)

	entry.Time = time.Now()
	entry.Level = level.String()
	entry.Message = msg
	if fields != nil {
		entry.Fields = fields
	}
	if l.reportCaller {
		entry.Caller = getCaller()
	}

	// 从 context 提取追踪信息
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		spanCtx := span.SpanContext()
		entry.TraceID = spanCtx.TraceID().String()
		entry.SpanID = spanCtx.SpanID().String()
	}

	l.fireHooks(entry)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeEntry(entry)

	if level == FatalLevel {
		os.Exit(1)
	}
}

// fireHooks 触发所有匹配的钩子 (异步执行)
func (l *logger) fireHooks(entry *Entry) {
	if len(l.hooks) == 0 {
		return
	}
	entryLevel, _ := ParseLevel(entry.Level)
	for _, hook := range l.hooks {
		for _, lvl := range hook.Levels() {
			if entryLevel == lvl {
				go func(h Hook, e *Entry) {
					if err := h.Fire(e); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to execute hook: %v\n", err)
					}
				}(hook, entry)
				break
			}
		}
	}
}

// writeEntry 将条目写入所有输出
func (l *logger) writeEntry(entry *Entry) {
	b, err := l.formatter.Format(entry)
	if err != nil {
		b = []byte(fmt.Sprintf("Failed to format log entry: %v", err))
	}
	for _, out := range l.outputs {
		_, _ = out.Write(b)
		// 确保每条日志后都有换行
		if _, ok := l.formatter.(*JSONFormatter); ok {
			_, _ = out.Write([]byte("\n"))
		}
	}

	// 触发后置 Hook（在日志写入后执行）
	l.firePostHooks(entry)
}

// 添加 firePostHooks 方法
func (l *logger) firePostHooks(entry *Entry) {
	if len(l.postHooks) == 0 {
		return
	}
	entryLevel, _ := ParseLevel(entry.Level)
	for _, hook := range l.postHooks {
		for _, lvl := range hook.Levels() {
			if entryLevel == lvl {
				go func(h PostHook, e *Entry) {
					if err := h.FireAfter(e); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to execute post hook: %v\n", err)
					}
				}(hook, entry)
				break
			}
		}
	}
}

// getCaller 获取调用者信息
func getCaller() string {
	_, file, line, ok := runtime.Caller(4) // 跳过 log, logf, log*, getCaller 四层
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

// --- Logger 接口方法实现 ---
// SetLevel 动态调整日志级别
func (l *logger) SetLevel(level Level) {
	l.level.Store(int32(level))
}

// Close 关闭所有输出
func (l *logger) Close() error {
	var err error
	for _, out := range l.outputs {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// Debug logs a message at DebugLevel.
func (l *logger) Debug(args ...interface{}) {
	l.log(DebugLevel, fmt.Sprint(args...), nil)
}

// Debugf logs a message at DebugLevel.
func (l *logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info logs a message at InfoLevel.
func (l *logger) Info(args ...interface{}) {
	l.log(InfoLevel, fmt.Sprint(args...), nil)
}

// Infof logs a message at InfoLevel.
func (l *logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn logs a message at WarnLevel.
func (l *logger) Warn(args ...interface{}) {
	l.log(WarnLevel, fmt.Sprint(args...), nil)
}

// Warnf logs a message at WarnLevel.
func (l *logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error logs a message at ErrorLevel.
func (l *logger) Error(args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprint(args...), nil)
}

// Errorf logs a message at ErrorLevel.
func (l *logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// Fatal logs a message at FatalLevel and exits.
func (l *logger) Fatal(args ...interface{}) {
	l.log(FatalLevel, fmt.Sprint(args...), nil)
}

// Fatalf logs a message at FatalLevel and exits.
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
}

// --- Context 版本方法实现 ---

func (l *logger) DebugContext(ctx context.Context, args ...interface{}) {
	l.logContext(ctx, DebugLevel, fmt.Sprint(args...), nil)
}

func (l *logger) DebugfContext(ctx context.Context, format string, args ...interface{}) {
	l.logContext(ctx, DebugLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) InfoContext(ctx context.Context, args ...interface{}) {
	l.logContext(ctx, InfoLevel, fmt.Sprint(args...), nil)
}

func (l *logger) InfofContext(ctx context.Context, format string, args ...interface{}) {
	l.logContext(ctx, InfoLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) WarnContext(ctx context.Context, args ...interface{}) {
	l.logContext(ctx, WarnLevel, fmt.Sprint(args...), nil)
}

func (l *logger) WarnfContext(ctx context.Context, format string, args ...interface{}) {
	l.logContext(ctx, WarnLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) ErrorContext(ctx context.Context, args ...interface{}) {
	l.logContext(ctx, ErrorLevel, fmt.Sprint(args...), nil)
}

func (l *logger) ErrorfContext(ctx context.Context, format string, args ...interface{}) {
	l.logContext(ctx, ErrorLevel, fmt.Sprintf(format, args...), nil)
}

func (l *logger) FatalContext(ctx context.Context, args ...interface{}) {
	l.logContext(ctx, FatalLevel, fmt.Sprint(args...), nil)
}

func (l *logger) FatalfContext(ctx context.Context, format string, args ...interface{}) {
	l.logContext(ctx, FatalLevel, fmt.Sprintf(format, args...), nil)
}
