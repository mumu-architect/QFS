#!/bin/bash
echo "🚀 启动 2 个 Raft 集群：每个 1 主 2 从"
echo "集群 1：节点 1 2 3"
echo "集群 2：节点 4 5 6"

#!/bin/bash
rm -rf ./data_node_*
echo "启动 2主4从 磁盘状态机集群"
go run main.go -id 1 &
go run main.go -id 2 &
go run main.go -id 3 &
go run main.go -id 4 &
go run main.go -id 5 &
go run main.go -id 6 &
wait
