package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"mumu.com/redis-go/monitor"
	_ "mumu.com/redis-go/monitor"
)

func main() {

	fmt.Printf("aaaa%d\n", 11)
	monitor.GetCpuInfo()
	fmt.Printf("============%d\n", 11)
	monitor.GetMemInfo()
	fmt.Printf("============%d\n", 11)
	monitor.GetSysLoad()
	fmt.Printf("============%d\n", 11)
	monitor.GetHostInfo()
	fmt.Printf("============%d\n", 11)
	monitor.GetDiskInfo()

	clusterNodes := []monitor.ClusterNode{
		{NodeID: 1, Addr: "127.0.0.1:9080"}, // 替换成你真实的IP和监控端口
		{NodeID: 2, Addr: "127.0.0.1:9081"}, // 替换成你真实的IP和监控端口
		{NodeID: 3, Addr: "127.0.0.1:9082"}, // 替换成你真实的IP和监控端口
		{NodeID: 4, Addr: "127.0.0.1:9083"}, // 替换成你真实的IP和监控端口
		{NodeID: 5, Addr: "127.0.0.1:9084"}, // 替换成你真实的IP和监控端口
		{NodeID: 6, Addr: "127.0.0.1:9085"}, // 替换成你真实的IP和监控端口

	}

	http.HandleFunc("/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data, err := monitor.FetchAllClusterMetrics(clusterNodes)
		if err != nil {
			http.Error(w, `{"error":"拉取失败"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	fmt.Println("监控服务启动在 :9999，访问 /cluster/status 查看数据")
	http.ListenAndServe(":9999", nil)
	select {}

}
