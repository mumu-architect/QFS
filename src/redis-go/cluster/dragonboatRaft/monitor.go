package dragonboatRaft

import (
	"fmt"
	"net/http"

	"github.com/lni/dragonboat/v4"
)

// StartMonitor 启动监控
// http:127.0.0.1:9090/metrics
// TODO:后期每个节点端口一样，修改未唯一端口9090
func StartMonitor(replicaID int) {
	port := 9090 + replicaID - 1
	go func() {
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			dragonboat.WriteHealthMetrics(w)
		})
		fmt.Printf(" Metrics: http://127.0.0.1:%d/metrics\n", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			fmt.Printf("Error starting http server on port 9090:%v \n", err)
		}
	}()

}
