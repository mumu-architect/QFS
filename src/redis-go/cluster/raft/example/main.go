package main

import (
	"fmt"
	"testing"

	"mumu.com/redis-go/cluster/raft"
)

func main() {
	fmt.Println("启动 HashiCorp Raft 集群交互式测试...")
	raft.TestRaftLeaderElection(&testing.T{})
}
