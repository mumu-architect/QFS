package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// JSONFormatter 是默认的 JSON 格式化器
type JSONFormatter struct{}

// Format 将 Entry 格式化为 JSON 字节流
func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	// 使用 json.Marshal 直接序列化
	data, err := json.Marshal(entry)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"json marshal failed: %v"}`, err)), nil
	}
	return data, nil
}

// TextFormatter 是一个简单的文本格式化器
type TextFormatter struct{}

// Format 将 Entry 格式化为人类可读的文本
func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(entry.Time.Format("2006-01-02 15:04:05.000"))
	buf.WriteString(" [")
	buf.WriteString(entry.Level)
	buf.WriteString("] ")

	if entry.Caller != "" {
		buf.WriteString("(")
		buf.WriteString(entry.Caller)
		buf.WriteString(") ")
	}

	buf.WriteString(entry.Message)

	if len(entry.Fields) > 0 {
		buf.WriteString(" [")
		first := true
		for k, v := range entry.Fields {
			if !first {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%s=%v", k, v)
			first = false
		}
		buf.WriteString("]")
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
