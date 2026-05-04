package cluster

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// IsStringInSliceIgnoreCase 进阶版：忽略大小写匹配
func IsStringInSliceIgnoreCase(target string, slice []string) bool {
	targetLower := strings.ToLower(target)
	for _, item := range slice {
		if strings.ToLower(item) == targetLower {
			return true
		}
	}
	return false
}

// generateNodeID 生成8位随机节点ID（16进制）
func generateNodeID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// curl 简化HTTP GET请求工具（模拟客户端调用）
func Curl(url string) {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("curl %s 失败：%v", url, err)
		return
	}
	defer resp.Body.Close()

	// 尝试解码为字符串
	var result string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// 如果解码失败，尝试解码为其他类型
		resp.Body.Close()
		resp, _ = http.Get(url)
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}

		var resultObj interface{}
		if err := json.NewDecoder(resp.Body).Decode(&resultObj); err != nil {
			log.Printf("curl %s 响应解析失败：%v", url, err)
			return
		}

		log.Printf("curl %s 结果：%v", url, resultObj)
		return
	}

	log.Printf("curl %s 结果：%s", url, result)
}
