package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func getMonitorData(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "Welcome to the main page!")
	//修改监控服务节点信息
	MonitorNodeData := GetMonitorNodeInstance()
	//logger.Info.Println(MonitorNodeData.monitorNodeMap.String())
	nodeDataMap := MonitorNodeData.monitorNodeMap
	var dataList []MonitorNodeAddr
	nodeDataMap.Range(func(k string, v MonitorNodeAddr) bool {
		//addrString := k

		nodeData := v
		dataList = append(dataList, nodeData)
		return true
	})
	Success(w, dataList)
}

// Result json返回数据格式
type Result struct {
	Res     bool   `json:"res"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// 对象转json
func jsonEncode(w http.ResponseWriter, data any) {
	jsonData, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func Success(w http.ResponseWriter, data any) {
	result := Result{
		Res:     true,
		Code:    200,
		Message: "ok",
		Data:    data,
	}
	jsonEncode(w, result)
}
func Error(w http.ResponseWriter, data any) {
	result := Result{
		Res:     false,
		Code:    0,
		Message: "ok",
		Data:    data,
	}
	jsonEncode(w, result)
}

func MonitorServer() {
	// 定义处理函数
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!") // 将 "Hello, World!" 作为响应返回
	}

	// 注册处理函数，并监听端口
	http.HandleFunc("/", handler)
	http.HandleFunc("/Monitor", getMonitorData)

	http.ListenAndServe(":9002", nil)
}
