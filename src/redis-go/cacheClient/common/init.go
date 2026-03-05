package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Client Redis客户端结构体（保持原有字段）
type Client struct {
	conn     net.Conn
	reader   *bufio.Reader
	addr     string        // Redis服务地址（如 "127.0.0.1:6379"）
	password string        // 认证密码（可选）
	timeout  time.Duration // 连接超时
}

// NewClient/Do/parseResponse 等基础方法保持不变（沿用之前实现）
func NewClient(addr string, password string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	client := &Client{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		addr:     addr,
		password: password,
		timeout:  timeout,
	}

	if password != "" {
		if _, err := client.Do("AUTH", password); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("认证失败: %w", err)
		}
	}

	return client, nil
}

func (c *Client) Do(cmd string, args ...string) (interface{}, error) {
	var req strings.Builder
	req.WriteString(fmt.Sprintf("*%d\r\n", len(args)+1))
	req.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(cmd), cmd))
	for _, arg := range args {
		req.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	if _, err := c.conn.Write([]byte(req.String())); err != nil {
		return nil, fmt.Errorf("发送命令失败: %w", err)
	}

	return c.parseResponse()
}

// -------------------------- 原有辅助方法（补充数组解析） --------------------------
// parseResponse 完善数组响应解析（支持MGET）
func (c *Client) parseResponse() (interface{}, error) {
	respType, err := c.reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	switch respType {
	case '+': // 简单字符串
		return c.readLine()
	case '$': // 批量字符串
		return c.readBulk()
	case ':': // 整数
		return c.readInt()
	case '-': // 错误
		msg, _ := c.readLine()
		return nil, errors.New(msg)
	case '*': // 数组（新增解析，支持MGET）
		return c.readArray()
	default:
		return nil, fmt.Errorf("未知响应类型: %c", respType)
	}
}

// readArray 解析数组响应
func (c *Client) readArray() ([]interface{}, error) {
	lenStr, err := c.readLine()
	if err != nil {
		return nil, err
	}
	lenVal, err := strconv.Atoi(lenStr)
	if err != nil || lenVal < 0 {
		return nil, errors.New("数组长度无效")
	}

	arr := make([]interface{}, lenVal)
	for i := 0; i < lenVal; i++ {
		// 递归解析每个元素（支持嵌套，适配MGET的批量字符串）
		elem, err := c.parseResponse()
		if err != nil {
			return nil, err
		}
		arr[i] = elem
	}
	return arr, nil
}

// readLine 读取一行（不变）
func (c *Client) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

// readBulk 读取批量字符串（不变）
func (c *Client) readBulk() (string, error) {
	lenStr, err := c.readLine()
	if err != nil {
		return "", err
	}
	lenVal, err := strconv.Atoi(lenStr)
	if err != nil {
		return "", err
	}
	if lenVal == -1 { // nil值（键不存在）
		return "", nil
	}

	buf := make([]byte, lenVal+2)
	// 修复点：替换 c.reader.ReadFull 为 io.ReadFull(c.reader, buf)
	if _, err := io.ReadFull(c.reader, buf); err != nil {
		return "", err
	}
	return string(buf[:lenVal]), nil
}

// readInt 读取整数（不变）
func (c *Client) readInt() (int64, error) {
	intStr, err := c.readLine()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(intStr, 10, 64)
}

// Close 关闭连接（不变）
func (c *Client) Close() error {
	return c.conn.Close()
}
