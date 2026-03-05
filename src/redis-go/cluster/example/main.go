package main

import (
	"fmt"
	"log"
	"time"

	"mumu.com/redis-go/cluster"
)

func main() {
	// 初始化数据目录（在storage.go中实现）
	cluster.InitDataDir()
	// 启动1主2从集群，用于测试从节点选举
	log.Println("=== 开始启动集群节点 ===")
	log.Println("启动主节点 :9001")
	go cluster.NewCluster(":9001", cluster.Master, "", "127.0.0.1:19001") // 主1
	log.Println("启动从节点 :9002")
	go cluster.NewCluster(":9002", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19002") // 从1（主1）
	log.Println("启动从节点 :9003")
	go cluster.NewCluster(":9003", cluster.Slave, "127.0.0.1:9001", "127.0.0.1:19003") // 从2（主1）
	log.Println("=== 集群节点启动完成 ===")

	// 模拟主1（:6379）故障
	//go func() {
	//	time.Sleep(10 * time.Second) // 等待集群稳定运行
	//	log.Println("=== 启动模拟主节点故障函数 ===")
	//
	//	simulateMasterFailure(":6379")
	//}()

	// 故障转移后，测试新主节点的数据写入、同步和落盘
	go func() {
		//time.Sleep(35 * time.Second) // 等待主节点故障和故障转移完成
		time.Sleep(5 * time.Second) // 等待主节点故障和故障转移完成
		log.Println("=== 开始测试新主节点的数据写入、同步和落盘 ===")

		// 测试向从节点 6382 写入数据，它应该已经升级为新的主节点
		log.Println("----------测试向从节点 9001 写入数据-----------")
		cluster.Curl("http://localhost:9001/Set?key=test:failover1&val=failover_value1")
		// 测试向从节点 6382 写入 Hash 类型数据
		//cluster.Curl("http://localhost:6379/HSet?key=test:failover&field=status&val=success")
		//cluster.Curl("http://localhost:6379/HSet?key=test:failover&field=time&val=2026-01-22")
		//cluster.Curl("http://localhost:6379/HSet?key=test:failover2&field=time&val=2026-01-22")
		// 测试向从节点 6385 写入数据，它也可能被选举为新的主节点
		//log.Println("测试向从节点 6385 写入数据")
		//cluster.Curl("http://localhost:6379/Set?key=test:failover2&val=failover_value2")
		// 测试向从节点 6385 写入 Hash 类型数据
		//cluster.Curl("http://127.0.0.1:6379/HSet?key=test:failover&field=status&val=success")
		//cluster.Curl("http://127.0.0.1:6379/HSet?key=test:failover&field=time&val=2026-01-22")

		// 等待数据同步和落盘完成
		time.Sleep(1 * time.Second)

		// 检查从节点 6382 的数据文件
		//log.Println("=== 检查从节点 6382 的数据文件 ===")
		//cluster.Curl("http://127.0.0.1:6382/Get?key=test:failover")
		//cluster.Curl("http://127.0.0.1:6382/HGet?key=test:failover&field=status")
		//cluster.Curl("http://127.0.0.1:6382/HGet?key=test:failover&field=time")
		//
		//// 检查从节点 6385 的数据文件
		//log.Println("=== 检查从节点 6385 的数据文件 ===")
		//cluster.Curl("http://127.0.0.1:6385/Get?key=test:failover")
		//cluster.Curl("http://127.0.0.1:6385/HGet?key=test:failover&field=status")
		//cluster.Curl("http://127.0.0.1:6385/HGet?key=test:failover&field=time")
	}()

	// 测试数据同步和落盘
	/*
		go func() {
			time.Sleep(5 * time.Second) // 等待集群启动完成
			log.Println("=== 开始测试数据同步和落盘 ===")

			// 向主节点 6379 写入数据（使用槽位在 0-5460 范围内的 key）
			log.Println("向主节点 6379 写入数据")
			cluster.Curl("http://127.0.0.1:6379/Set?key=aaa&val=aaa_value")

			// 向主节点 6380 写入数据（使用槽位在 5461-10921 范围内的 key）
			log.Println("向主节点 6380 写入数据")
			cluster.Curl("http://127.0.0.1:6380/Set?key=bbb&val=bbb_value")

			// 向主节点 6381 写入数据（使用槽位在 10922-16383 范围内的 key）
			log.Println("向主节点 6381 写入数据")
			cluster.Curl("http://127.0.0.1:6381/Set?key=ccc&val=ccc_value")

			// 测试 Hash 类型数据写入
			log.Println("=== 开始测试 Hash 类型数据写入 ===")
			// 通过 HTTP 请求测试 Hash 类型数据写入
			log.Println("向主节点 6380 写入 Hash 类型数据")
			// 使用槽位在 5461-10921 范围内的 key
			cluster.Curl("http://127.0.0.1:6380/HSet?key=bbb&field=name&val=zhangsan")
			cluster.Curl("http://127.0.0.1:6380/HSet?key=bbb&field=age&val=25")

			// 等待数据同步和落盘完成
			time.Sleep(1 * time.Second)

			// 检查从节点的数据文件
			log.Println("=== 检查从节点的数据文件 ===")
			cluster.Curl("http://127.0.0.1:6382/Get?key=aaa")
			cluster.Curl("http://127.0.0.1:6383/Get?key=bbb")
			cluster.Curl("http://127.0.0.1:6384/Get?key=ccc")
			cluster.Curl("http://127.0.0.1:6383/HGet?key=user:1&field=name")
			cluster.Curl("http://127.0.0.1:6383/HGet?key=user:1&field=age")
		}()
	*/
	// 保持主线程运行
	log.Println("=== 主线程开始运行 ===")
	for i := 0; i < 30; i++ {
		log.Printf("主线程运行中，第 %d 秒", i)
		time.Sleep(1 * time.Second)
	}
	log.Println("=== 主线程运行结束 ===")
}

// simulateMasterFailure 模拟主节点故障（停止指定主节点）
func simulateMasterFailure(masterAddr string) {
	log.Printf("=== 模拟主节点 %s 故障 start===", masterAddr)
	// 延迟10秒，让集群先稳定运行，并有足够时间写入数据
	time.Sleep(3 * time.Second)
	log.Printf("=== 模拟主节点 %s 故障 ===", masterAddr)

	// 发送停止命令到主节点
	url := fmt.Sprintf("http://localhost%s/shutdown", masterAddr)
	log.Printf("发送停止命令到主节点 %s，URL：%s", masterAddr, url)
	log.Printf("=== 模拟主节点 %s 故障 start2===", masterAddr)
	cluster.Curl(url)
	log.Printf("=== 模拟主节点 %s 故障 start3===", masterAddr)
	log.Printf("停止命令执行完成")
}
