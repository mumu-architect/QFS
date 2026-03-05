package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"mumu.com/redis-go/cmd/redis-cli/config"
)

func parseArgs(host string, port string) (string, string) {
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "6379"
	}

	if len(os.Args) > 1 {
		parts := strings.Split(os.Args[1], ":")
		host = parts[0]
		if len(parts) > 1 {
			port = parts[1]
		}
	}

	return host, port
}

// 修复中文编码问题：使用字节长度而非字符长度
func encodeCommand(args []string) []byte {
	var buf bytes.Buffer

	// 数组头部，*<count>\r\n
	buf.WriteString(fmt.Sprintf("*%d\r\n", len(args)))

	// 每个参数的长度和值（关键修复：使用len([]byte(arg))计算字节长度）
	for _, arg := range args {
		argBytes := []byte(arg)                                // 将字符串转换为字节数组
		buf.WriteString(fmt.Sprintf("$%d\r\n", len(argBytes))) // 计算字节长度
		buf.Write(argBytes)                                    // 直接写入字节数组
		buf.WriteString("\r\n")
	}

	return buf.Bytes()
}

func decodeResponse(reader *bufio.Reader) (interface{}, error) {
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
		return nil, fmt.Errorf("redis-cli.go,unknown RESP type: %c", t)
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

func decodeBulkString(reader *bufio.Reader) (interface{}, error) {
	lenStr, err := readLine(reader)
	if err != nil {
		return nil, err
	}

	var length int
	fmt.Sscanf(lenStr, "%d", &length)

	if length == -1 {
		return nil, nil
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, err
	}

	_, err = reader.Discard(2)
	if err != nil {
		return nil, err
	}

	// 尝试将字节转换为字符串（支持UTF-8）
	return string(buf), nil
}

func decodeArray(reader *bufio.Reader) (interface{}, error) {
	lenStr, err := readLine(reader)
	if err != nil {
		return nil, err
	}

	var length int
	fmt.Sscanf(lenStr, "%d", &length)

	if length == -1 {
		return nil, nil
	}

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

func getProjectRootDir() (string, error) {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return execPath, err
}

func main() {
	//projectRootDir, _ := getProjectRootDir()
	//println(projectRootDir)
	// 1. 解析命令行参数
	configPath := flag.String("config", "configs/redis-cli.yaml", "Path to the configuration file")
	flag.Parse()

	// 2. 加载配置文件，如果不存在则自动创建并使用默认配置
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("Failed to load or create configuration: %v", err)
	}

	host, port := parseArgs(cfg.Server.IP, cfg.Server.Port)
	address := fmt.Sprintf("%s:%s", host, port)

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

		if input == "quit" || input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		args := strings.Fields(input)

		encoded := encodeCommand(args)
		_, err := conn.Write(encoded)
		if err != nil {
			fmt.Printf("Error sending command: %v\n", err)
			fmt.Print("> ")
			continue
		}

		resp, err := decodeResponse(reader)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			fmt.Print("> ")
			continue
		}

		printResponse(resp)
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}
