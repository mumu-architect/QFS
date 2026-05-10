package main

import (
	"os"
	"time"

	"mumu.com/redis-go/qfsSdk"
)

func main() {
	// 1. 初始化SDK
	qfsSdk.InitSDK(&qfsSdk.QFSConfig{
		ClusterEntry: []string{
			"127.0.0.1:9001",
			"127.0.0.1:9002",
			"127.0.0.1:9003",
		},
		Timeout: 15 * time.Second,
	})

	// 2. 读取测试文件
	file, err := os.Open("./11.jpeg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 3. 直接调用SDK上传
	resp, err := qfsSdk.Upload(file, "11.jpeg")
	if err != nil {
		panic(err)
	}

	println("上传结果:", string(resp))
}
