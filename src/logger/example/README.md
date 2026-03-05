# Logger - 企业级 Go 日志框架
Logger 是一款功能完备、高性能的 Go 语言日志框架，专为生产环境设计，提供灵活的日志管理能力与丰富的企业级特性，适配从小型应用到大型分布式系统的全场景日志需求


## 📋核心特性
| 特性分类 | 具体能力 |
| ----- | ----- |
| 日志级别 |	支持 Debug、Info、Warn、Error、Fatal 五级日志 |
| 输出格式 |	内置 Text（人类可读）和 JSON（机器解析）格式，支持自定义格式化器 |
| 输出目标 |	可同时输出到控制台（stdout/stderr）和文件，支持多文件并行输出 |
| 文件轮转 |	支持按大小、按时间（时 / 日 / 自定义间隔）、混合策略（任一条件触发）轮转 |
| 性能优化 |	异步写入、对象池复用、批量 I/O 写入，降低业务阻塞与 GC 压力 |
| 溢出策略 |	异步缓冲区满时，支持「阻塞等待」或「直接丢弃」，平衡日志完整性与性能 |
| 扩展能力 |	支持 Hook 机制（异步执行），可扩展告警、监控等自定义逻辑 |
| 动态配置 |	运行时可调整日志级别，无需重启服务，便于在线问题排查 |
| 追踪集成 |	自动从 context 提取 TraceID/SpanID，适配分布式追踪系统（如 OTel） |
| 调用者信息 |	可选记录日志调用的文件名和行号，精准定位代码位置 |


## 🚀 安装
```bash
# 替换为实际模块路径（如 GitHub 仓库地址）
go get github.com/yourname/logger
```

## 🔍 快速开始
### 基础用法
```go
package main

import "github.com/yourname/logger"

func main() {
    // 1. 创建默认配置日志器（输出到 stdout，级别为 Info）
    log, err := logger.New(logger.Config{})
    if err != nil {
        panic(err)
    }
    // 2. 程序退出前关闭日志器，确保缓存日志写入完成
    defer log.Close()

    // 3. 输出不同级别的日志
    log.Debug("调试信息（默认不输出）")
    log.Info("应用启动成功")
    log.Warnf("磁盘空间不足：%.2f%%", 15.6)
    log.Error("数据库连接失败")
    // log.Fatal("致命错误，程序将退出") // 会触发 os.Exit(1)
}
```


### 输出示例（Text 格式）
```plaintext
2024-06-01 10:30:00.123 [INFO] (main.go:16) 应用启动成功
2024-06-01 10:30:00.124 [WARN] (main.go:17) 磁盘空间不足：15.60%
2024-06-01 10:30:00.124 [ERROR] (main.go:18) 数据库连接失败
```

## ⚙️ 配置详解
Config 结构体是日志框架的核心配置入口，支持精细化控制所有功能：
```go
// 完整配置示例
cfg := logger.Config{
    // 1. 基础配置
    Level:           logger.DebugLevel,  // 日志级别（默认 InfoLevel）
    Format:          "json",             // 输出格式："text" 或 "json"（默认 "text"）
    OutputPaths:     []string{           // 输出目标（支持多个）
        "stdout",                        // 控制台输出
        "logs/app.log",                  // 文件输出（自动创建目录）
    },

    // 2. 文件轮转配置
    MaxSize:         10,                 // 单个日志文件最大尺寸（MB）
    MaxBackups:      5,                  // 最多保留备份文件数
    MaxAge:          7,                  // 日志文件保留天数
    Compress:        true,               // 是否压缩备份文件（.gz 格式）
    RotationPolicy:  logger.RotationPolicyHybrid,  // 轮转策略
    RotationInterval: "daily",           // 时间轮转间隔（"hourly"/"daily"/"12h" 等）

    // 3. 异步配置
    Async:           true,               // 是否启用异步写入（默认 false）
    AsyncBuffer:     10000,              // 异步缓冲区大小（默认 1000）
    OverflowPolicy:  logger.OverflowDiscard,  // 缓冲区满时策略

    // 4. 扩展配置
    ReportCaller:    true,               // 是否记录调用者（文件名:行号）
    Hooks:           []logger.Hook{&MyErrorHook{}},  // 注册自定义 Hook
    // CustomFormatter: &MyFormatter{},   // 自定义格式化器（优先级高于 Format）
}

// 创建日志器
log, err := logger.New(cfg)
if err != nil {
    // 处理初始化错误
}
defer log.Close()
```

### 关键配置说明
|配置项|	可选值 / 类型|	说明|
| ----- | ----- | ----- |
| RotationPolicy |	RotationPolicySize/RotationPolicyTime/RotationPolicyHybrid |	轮转策略：按大小、按时间、混合（任一条件触发） |
| OverflowPolicy |	OverflowBlock/OverflowDiscard |	缓冲区满时策略：阻塞等待（保证不丢日志）/ 直接丢弃（保证业务性能） |
| RotationInterval |	字符串（如 "hourly"/"12h"） |	时间轮转间隔，支持标准 Go 时间格式（如 "1h30m"） |

## 📚 高级用法
### 1. 自定义 Hook（错误告警示例）
   Hook 机制允许在特定级别日志输出时触发自定义逻辑（如发送告警），异步执行避免阻塞主流程。
   
```go
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
func main() {
    cfg := logger.Config{
        Hooks: []logger.Hook{&ErrorAlertHook{}},  // 注册 Hook
    }
    log, _ := logger.New(cfg)
    defer log.Close()

    // 触发 Hook
    log.Error("支付失败", logger.Fields{"order_id": "123456", "amount": 99.9})
}
```

### 2. 动态调整日志级别
   支持运行时修改日志级别，无需重启服务，适合在线调试（如临时开启 Debug 日志）。
   
```go
package main

import (
    "net/http"
    "github.com/yourname/logger"
)

var log logger.Logger

func init() {
    // 初始化日志器
    var err error
    log, err = logger.New(logger.Config{Level: logger.InfoLevel})
    if err != nil {
        panic(err)
    }
}

func main() {
    // 暴露 HTTP 接口用于调整日志级别
    http.HandleFunc("/admin/set-log-level", func(w http.ResponseWriter, r *http.Request) {
        // 从请求参数获取目标级别（如 ?level=debug）
        levelStr := r.URL.Query().Get("level")
        if levelStr == "" {
            http.Error(w, "缺少 level 参数（支持 debug/info/warn/error/fatal）", http.StatusBadRequest)
            return
        }

        // 解析级别字符串
        level, err := logger.ParseLevel(levelStr)
        if err != nil {
            http.Error(w, "无效的 level 值", http.StatusBadRequest)
            return
        }

        // 动态调整级别
        log.SetLevel(level)
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("日志级别已更新为: " + level.String()))
    })

    log.Info("管理服务启动，监听 :8080")
    http.ListenAndServe(":8080", nil)
}
```

#### 使用方式
```bash
# 临时开启 Debug 日志
curl "http://localhost:8080/admin/set-log-level?level=debug"

# 恢复为 Info 级别
curl "http://localhost:8080/admin/set-log-level?level=info"
```

### 3. 分布式追踪集成
   自动从 context.Context 中提取 TraceID 和 SpanID（适配 OpenTelemetry 等追踪系统），实现跨服务日志串联。

```go
package main

import (
    "context"
    "github.com/yourname/logger"
    "go.opentelemetry.io/otel"
)

func main() {
    log, _ := logger.New(logger.Config{
        Format: "json",
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
```
#### JSON 输出示例
```json
{
    "time": "2024-06-01T10:35:00.123456Z",
    "level": "INFO",
    "message": "处理用户请求",
    "fields": {"user_id": 1001},
    "caller": "main.go:22",
    "trace_id": "a1b2c3d4e5f67890a1b2c3d4e5f67890",
    "span_id": "fedcba9876543210"
}
```
### 4. 自定义格式化器
   若内置格式不满足需求，可通过实现 Formatter 接口自定义输出格式。

```go
// 1. 实现自定义 Formatter
type MyFormatter struct{}

func (f *MyFormatter) Format(entry *logger.Entry) ([]byte, error) {
    // 自定义输出格式：[时间] [级别] 消息 (附加信息)
    formatStr := "[%s] [%s] %s (trace=%s, span=%s)\n"
    return []byte(fmt.Sprintf(
        formatStr,
        entry.Time.Format("2006-01-02 15:04:05"),
        entry.Level,
        entry.Message,
        entry.TraceID,
        entry.SpanID,
    )), nil
}

// 2. 使用自定义 Formatter
func main() {
    cfg := logger.Config{
        CustomFormatter: &MyFormatter{},  // 启用自定义格式化器
    }
    log, _ := logger.New(cfg)
    defer log.Close()

    log.Info("自定义格式日志")
}
```


## 📖 API 参考
### 核心接口 Logger
所有日志操作通过 Logger 接口完成，确保扩展性和一致性。
#### 基础日志方法

| 方法签名| 	描述  |
| ----- |------|
| Debug(args ...interface{})|	输出 Debug 级别日志|
| Debugf(format string, args ...interface{})|	输出格式化 Debug 级别日志|
| Info(args ...interface{})|	输出 Info 级别日志|
| Infof(format string, args ...interface{})|	输出格式化 Info 级别日志|
| Warn(args ...interface{})|	输出 Warn 级别日志|
| Warnf(format string, args ...interface{})|	输出格式化 Warn 级别日志|
| Error(args ...interface{})|	输出 Error 级别日志|
| Errorf(format string, args ...interface{})|	输出格式化 Error 级别日志|
| Fatal(args ...interface{})|	输出 Fatal 级别日志并退出程序|
| Fatalf(format string, args ...interface{})|	输出格式化 Fatal 级别日志并退出程序|


#### 带 Context 的方法
为分布式追踪场景设计，方法名以 Context 结尾：
1.DebugContext(ctx context.Context, args ...interface{})
2.InfofContext(ctx context.Context, format string, args ...interface{})
3.（其他级别方法类似，均支持从 ctx 提取追踪信息）

#### 控制方法
|方法签名|	描述|
| ----- |------|
|SetLevel(level Level)|	动态设置日志级别|
|Close() error|	关闭日志器，释放文件句柄等资源|


## 🎯 生产环境最佳实践
1.启用异步写入
生产环境建议开启 Async: true，避免磁盘 I/O 阻塞业务逻辑。根据日志量调整 AsyncBuffer（建议为高峰期 1-2 秒的日志条数）。

2.合理选择溢出策略
- 关键业务（如支付、订单）：使用 OverflowBlock，确保日志不丢失。
- 非关键业务（如访问日志）：使用 OverflowDiscard，优先保障业务性能。

3.日志文件管理
- 单个日志文件大小建议设置为 10-100MB（MaxSize），便于日志分割和分析。
- 备份文件数（MaxBackups）和保留天数（MaxAge）根据磁盘空间调整，建议保留 7-30 天。

4.Hook 性能优化
Hook 中避免执行耗时操作（如网络请求），若需执行，建议在 Hook 内部启动 goroutine 异步处理。

5.本地开发调试
本地开发时可关闭异步（Async: false），并开启 ReportCaller: true，便于快速定位问题。















