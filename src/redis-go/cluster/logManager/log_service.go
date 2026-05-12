package logManager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// StartHTTPAPI  启动HTTP服务
func (nl *NodeLog) startLogHTTPServer() {
	// 监听地址（与节点地址一致）
	//TODO: listenAddr := c.LocalNode.Addr
	//listenAddr := "1" + strings.Split(c.LocalNode.Addr, ":")[1]
	//listenAddr := cl.LocalNode.Addr

	listenAddr := fmt.Sprintf(":%d", nl.LogPort)
	// 创建HTTP服务器实例
	nl.HttpServer = &http.Server{
		Addr:    listenAddr,
		Handler: nl.ServeMux,
	}
	log.Printf("HTTP服务启动，监听：%s", listenAddr)
	// 启动HTTP服务
	err := nl.HttpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

func (nl *NodeLog) RegisterLogHandlers() {
	nl.ServeMux.HandleFunc("/syncLog", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("lastIndex")
		if key == "" {
			http.Error(w, "key不能为空", http.StatusBadRequest)
			return
		}
		//获取本地节点的log的lastIndex
		lastIndex, _ := strconv.ParseUint(key, 10, 64)
		// 查出所有大于lastIdx的日志
		//firstIdx, _ := LocalLog.FirstIndex()
		maxIdx, _ := nl.LocalLog.LastIndex()
		// 无增量数据
		if lastIndex >= maxIdx {
			_ = json.NewEncoder(w).Encode(SyncResponse{HasMore: false})
			return
		}
		start := lastIndex + 1
		end := start + BatchLimit - 1
		// 边界截断
		if end > maxIdx {
			end = maxIdx
		}
		dataMap := make(map[uint64][]byte)
		// 批量读取区间数据
		for idx := start; idx <= end; idx++ {
			data, err := nl.LocalLog.Read(idx)
			if err != nil {
				continue
			}
			dataMap[idx] = data
		}
		// 判断是否还有下一批
		hasMore := end < maxIdx
		resp := SyncResponse{
			DataMap:    dataMap,
			HasMore:    hasMore,
			DataNumber: uint64(len(dataMap)),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}
