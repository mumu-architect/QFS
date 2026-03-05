package logger

import (
	"fmt"
	"strings"
	"time"
)

// Level 定义日志级别
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String 返回级别名称
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 将字符串转换为 Level 类型
func ParseLevel(levelStr string) (Level, error) {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return DebugLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "WARN":
		return WarnLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	case "FATAL":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}

// Fields 定义额外的日志字段
type Fields map[string]interface{}

// Entry 定义一个完整的日志条目
type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Fields  Fields    `json:"fields,omitempty"`
	Caller  string    `json:"caller,omitempty"`
	TraceID string    `json:"trace_id,omitempty"` // 分布式追踪ID
	SpanID  string    `json:"span_id,omitempty"`  // 分布式追踪SpanID
}

// Formatter 定义了日志格式化接口
type Formatter interface {
	Format(entry *Entry) ([]byte, error)
}

// Hook 接口定义了日志钩子的行为
type Hook interface {
	// Levels 返回该 Hook 关心的日志级别
	Levels() []Level
	// Fire 在日志记录时被调用
	Fire(entry *Entry) error
}

// PostHook 接口定义了日志后置钩子的行为
type PostHook interface {
	// Levels 返回该后置 Hook 关心的日志级别
	Levels() []Level
	// FireAfter 在日志写入后被调用
	FireAfter(entry *Entry) error
}

// OverflowPolicy 定义了异步日志缓冲区满时的策略
type OverflowPolicy string

const (
	OverflowBlock   OverflowPolicy = "block"   // 阻塞等待
	OverflowDiscard OverflowPolicy = "discard" // 直接丢弃
)

// RotationPolicyType 定义了日志轮转策略
type RotationPolicyType string

const (
	RotationPolicySize   RotationPolicyType = "size"   // 仅按大小
	RotationPolicyTime   RotationPolicyType = "time"   // 仅按时间
	RotationPolicyHybrid RotationPolicyType = "hybrid" // 混合策略
)

// Config 定义日志配置
type Config struct {
	Level            Level
	Format           string // "text" or "json"
	OutputPaths      []string
	MaxSize          int  // MB, 用于大小轮转
	MaxBackups       int  // 最大备份文件数
	MaxAge           int  // 最大保留天数
	Compress         bool // 是否压缩备份文件
	RotationPolicy   RotationPolicyType
	RotationInterval string // 时间轮转间隔, e.g., "daily", "hourly", "1h"

	Async          bool           // 是否启用异步写入
	AsyncBuffer    int            // 异步 channel 缓冲大小
	OverflowPolicy OverflowPolicy // 缓冲区满时策略

	ReportCaller bool       // 是否报告调用者信息
	Hooks        []Hook     // 注册的钩子
	PostHooks    []PostHook // 注册的后置钩子

	CustomFormatter Formatter // 自定义格式化器
}
