package main

import (
	"logger"
	"net/http"
)

var log logger.Logger

func init() {
	// 初始化日志器
	var err error
	log, err = logger.New(logger.Config{Level: logger.InfoLevel})
	if err != nil {
		panic(err)
	}
}

func main3() {
	// 暴露 HTTP 接口用于调整日志级别
	http.HandleFunc("/admin/set-log-level", func(w http.ResponseWriter, r *http.Request) {
		// 从请求参数获取目标级别（如 ?level=debug）
		levelStr := r.URL.Query().Get("level")
		if levelStr == "" {
			http.Error(w, "缺少 level 参数（支持 debug/info/warn/error/fatal）", http.StatusBadRequest)
			return
		}

		// 解析级别字符串
		level, err := logger.ParseLevel(levelStr)
		if err != nil {
			http.Error(w, "无效的 level 值", http.StatusBadRequest)
			return
		}

		// 动态调整级别
		log.SetLevel(level)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("日志级别已更新为: " + level.String()))
	})

	log.Info("管理服务启动，监听 :8080")
	http.ListenAndServe(":8080", nil)
}
