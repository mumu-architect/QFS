package fileManager

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mumu.com/redis-go/cluster/dragonboatRaft"
)

// StartMasterHTTPServer
// 自动规则：监听端口 8086 → 程序运行目录下 data-8086
// 监听端口 8081 → 自动 data-8081
// 路由固定前缀 /files/
func (nf *NodeFile) startFileHttpServer() {
	// 拆分出端口 例如 0.0.0.0:8086 → 8086
	listenAddr := nf.LeaderAddr
	port := ""
	parts := strings.Split(listenAddr, ":")
	if len(parts) == 2 {
		port = parts[1]
	}
	// 自动按端口生成目录 data-端口
	bizDir := FileDataDir + "/data_" + port
	// 不存在自动创建
	err := os.MkdirAll(bizDir, 0755)
	if err != nil {
		_ = fmt.Sprintf("create file dir error:%v", err)
		return
	}
	// 静态文件服务 + 规范 /files/ 前缀
	fs := http.FileServer(http.Dir(bizDir))
	http.Handle("/files/", http.StripPrefix("/files/", fs))
	srv := &http.Server{
		Addr:         listenAddr,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

// DynamicallyModifyFileNodeLeaderID TODO:DynamicallyModifyFileNodeLeaderID 动态修改文件服务节点leaderID
func (nf *NodeFile) DynamicallyModifyFileNodeLeaderID(shardNodeLeaderID *dragonboatRaft.NodeShardLeaderMeta) {
	if nf.ShardId == shardNodeLeaderID.ShardID {
		nf.LeaderId = shardNodeLeaderID.LeaderID
	}
}
