package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// 解析命令行参数，获取主机和端口
func parseArgs() (host, port string) {
	host = "127.0.0.1"
	port = "6379"

	if len(os.Args) > 1 {
		parts := strings.Split(os.Args[1], ":")
		host = parts[0]
		if len(parts) > 1 {
			port = parts[1]
		}
	}

	return host, port
}

// 将Redis命令编码为RESP协议格式
func encodeCommand(args []string) []byte {
	var buf bytes.Buffer

	// 数组头部，*<count>\r\n
	buf.WriteString(fmt.Sprintf("*%d\r\n", len(args)))

	// 每个参数的长度和值
	for _, arg := range args {
		buf.WriteString(fmt.Sprintf("$%d\r\n", len(arg)))
		buf.WriteString(fmt.Sprintf("%s\r\n", arg))
	}

	return buf.Bytes()
}

// 从RESP协议格式解码响应
func decodeResponse(reader *bufio.Reader) (interface{}, error) {
	// 读取第一个字节判断响应类型
	t, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch t {
	case '+': // 简单字符串
		return readLine(reader)
	case '-': // 错误
		msg, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		return fmt.Errorf("redis error: %s", msg), nil
	case ':': // 整数
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		var num int64
		fmt.Sscanf(line, "%d", &num)
		return num, nil
	case '$': // 批量字符串
		return decodeBulkString(reader)
	case '*': // 数组
		return decodeArray(reader)
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", t)
	}
}

// 读取一行数据（直到\r\n）
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// 去除末尾的\r\n
	return strings.TrimSuffix(line, "\r\n"), nil
}

// 解码批量字符串
func decodeBulkString(reader *bufio.Reader) (interface{}, error) {
	// 读取长度
	lenStr, err := readLine(reader)
	if err != nil {
		return nil, err
	}

	var length int
	fmt.Sscanf(lenStr, "%d", &length)

	if length == -1 { // 表示空
		return nil, nil
	}

	// 读取指定长度的字符串
	buf := make([]byte, length)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, err
	}

	// 读取末尾的\r\n
	_, err = reader.Discard(2)
	if err != nil {
		return nil, err
	}

	return string(buf), nil
}

// 解码数组
func decodeArray(reader *bufio.Reader) (interface{}, error) {
	// 读取数组长度
	lenStr, err := readLine(reader)
	if err != nil {
		return nil, err
	}

	var length int
	fmt.Sscanf(lenStr, "%d", &length)

	if length == -1 { // 表示空数组
		return nil, nil
	}

	// 逐个解码数组元素
	array := make([]interface{}, length)
	for i := 0; i < length; i++ {
		elem, err := decodeResponse(reader)
		if err != nil {
			return nil, err
		}
		array[i] = elem
	}

	return array, nil
}

// 打印响应结果
func printResponse(resp interface{}) {
	if err, ok := resp.(error); ok {
		fmt.Printf("(error) %s\n", err)
		return
	}

	switch v := resp.(type) {
	case string:
		fmt.Printf("\"%s\"\n", v)
	case int64:
		fmt.Printf("%d\n", v)
	case []interface{}:
		fmt.Println("[")
		for i, elem := range v {
			fmt.Printf("  ")
			printResponse(elem)
			if i < len(v)-1 {
				fmt.Println(",")
			}
		}
		fmt.Println("\n]")
	case nil:
		fmt.Println("(nil)")
	default:
		fmt.Printf("Unknown response type: %T\n", v)
		jsonResp, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(jsonResp))
	}
}

func main() {
	host, port := parseArgs()
	address := fmt.Sprintf("%s:%s", host, port)

	// 连接到Redis服务器
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Could not connect to Redis at %s: %v\n", address, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Connected to Redis at %s\n", address)
	fmt.Println("Type 'quit' or 'exit' to quit")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	reader := bufio.NewReader(conn)

	for scanner.Scan() {
		input := scanner.Text()
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Print("> ")
			continue
		}

		// 处理退出命令
		if input == "quit" || input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		// 分割命令和参数
		args := strings.Fields(input)

		// 编码命令并发送
		encoded := encodeCommand(args)
		_, err := conn.Write(encoded)
		if err != nil {
			fmt.Printf("Error sending command: %v\n", err)
			fmt.Print("> ")
			continue
		}

		// 接收并解码响应
		resp, err := decodeResponse(reader)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			fmt.Print("> ")
			continue
		}

		// 打印响应
		printResponse(resp)
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}
