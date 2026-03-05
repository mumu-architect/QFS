package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// fileRotator 实现了按大小和时间轮转的文件写入器
type fileRotator struct {
	path       string
	maxSize    int64 // MB -> Bytes
	maxBackups int
	maxAge     int // days
	compress   bool

	policy   RotationPolicyType
	interval time.Duration
	nextTime time.Time

	file *os.File
	size int64
	mu   sync.Mutex
}

// newFileRotator 创建一个新的文件轮转器
func newFileRotator(path string, cfg Config) (*fileRotator, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	fr := &fileRotator{
		path:       path,
		maxSize:    int64(cfg.MaxSize) * 1024 * 1024,
		maxBackups: cfg.MaxBackups,
		maxAge:     cfg.MaxAge,
		compress:   cfg.Compress,
		policy:     cfg.RotationPolicy,
	}

	if cfg.RotationPolicy == RotationPolicyTime || cfg.RotationPolicy == RotationPolicyHybrid {
		var err error
		fr.interval, err = parseRotationInterval(cfg.RotationInterval)
		if err != nil {
			return nil, err
		}
		fr.nextTime = calculateNextRotation(time.Now(), fr.interval)
	}

	if err := fr.openFile(); err != nil {
		return nil, err
	}

	// 启动一个 goroutine 定期清理旧文件
	go fr.startCleaner()

	return fr, nil
}

// parseRotationInterval 解析时间间隔字符串
func parseRotationInterval(intervalStr string) (time.Duration, error) {
	if intervalStr == "" {
		return 24 * time.Hour, nil // default to daily
	}
	switch strings.ToLower(intervalStr) {
	case "hourly":
		return time.Hour, nil
	case "daily":
		return 24 * time.Hour, nil
	default:
		return time.ParseDuration(intervalStr)
	}
}

// calculateNextRotation 计算下次轮转时间
func calculateNextRotation(now time.Time, interval time.Duration) time.Time {
	if interval == time.Hour {
		return now.Truncate(time.Hour).Add(time.Hour)
	}
	if interval == 24*time.Hour {
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	}
	return now.Truncate(interval).Add(interval)
}

// openFile 打开或创建当前日志文件
func (fr *fileRotator) openFile() error {
	file, err := os.OpenFile(fr.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	if fi, err := file.Stat(); err == nil {
		fr.size = fi.Size()
	}
	fr.file = file
	return nil
}

// Write 实现了 io.Writer 接口
func (fr *fileRotator) Write(p []byte) (n int, err error) {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if fr.needsRotation(len(p)) {
		if err := fr.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = fr.file.Write(p)
	fr.size += int64(n)
	return n, err
}

// Close 关闭文件
func (fr *fileRotator) Close() error {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if fr.file != nil {
		return fr.file.Close()
	}
	return nil
}

// needsRotation 判断是否需要轮转
func (fr *fileRotator) needsRotation(bytesToWrite int) bool {
	sizeCheck := fr.maxSize > 0 && (fr.size+int64(bytesToWrite)) >= fr.maxSize

	timeCheck := false
	if (fr.policy == RotationPolicyTime || fr.policy == RotationPolicyHybrid) && fr.interval > 0 {
		now := time.Now()
		if now.After(fr.nextTime) {
			fr.nextTime = calculateNextRotation(now, fr.interval)
			timeCheck = true
		}
	}

	switch fr.policy {
	case RotationPolicySize:
		return sizeCheck
	case RotationPolicyTime:
		return timeCheck
	case RotationPolicyHybrid:
		return sizeCheck || timeCheck
	default:
		return false
	}
}

// rotate 执行轮转操作
func (fr *fileRotator) rotate() error {
	if err := fr.file.Close(); err != nil {
		return err
	}

	timestamp := time.Now().Format("2006-01-02-15-04-05")
	newPath := fmt.Sprintf("%s.%s", fr.path, timestamp)

	if err := os.Rename(fr.path, newPath); err != nil {
		return err
	}

	if fr.compress {
		if err := fr.compressFile(newPath); err != nil {
			return err
		}
	}

	fr.cleanup()
	return fr.openFile()
}

// compressFile 压缩文件
func (fr *fileRotator) compressFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(path + ".gz")
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	_, err = io.Copy(gz, in)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// startCleaner 启动清理 goroutine
func (fr *fileRotator) startCleaner() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		fr.cleanup()
	}
}

// cleanup 清理过期或超出数量的备份文件
func (fr *fileRotator) cleanup() {
	files, err := filepath.Glob(fmt.Sprintf("%s.*", fr.path))
	if err != nil {
		return
	}

	sort.Strings(files)

	cutoff := time.Now().Add(-time.Duration(fr.maxAge) * 24 * time.Hour)

	for i, file := range files {
		if fr.maxBackups > 0 && i < len(files)-fr.maxBackups {
			os.Remove(file)
			continue
		}
		if fi, err := os.Stat(file); err == nil && fr.maxAge > 0 && fi.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}
