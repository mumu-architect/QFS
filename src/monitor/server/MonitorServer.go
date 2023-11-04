package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func getMonitorNodeData(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "Welcome to the main page!")
	monitorRaftSelectionObject, err := MonitorRaftSelectionRun()
	if err == nil {
		var data []string
		Error(w, data)
	}
	Success(w, monitorRaftSelectionObject)
}
func getNodeData(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "Welcome to the main page!")
	nodeHeartData := GETNodeHeartInstance()
	nodeDataMap := nodeHeartData.GetNodeMap()

	var dataList []NodeData
	nodeDataMap.Range(func(k string, v NodeData) bool {
		//addrString := k
		nodeData := v
		nodeData.NewHeartTime = nodeData.NewHeartTime / 1e6
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
	return
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
	http.HandleFunc("/node", getNodeData)

	http.ListenAndServe(":9001", nil)
}
