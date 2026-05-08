package monitor

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

//
//var nodes = []string{
//	"127.0.0.1:9080",
//	"127.0.0.1:9081",
//	"127.0.0.1:9082",
//	"127.0.0.1:9083",
//	"127.0.0.1:9084",
//	"127.0.0.1:9085",
//}

// 集群节点信息
type ClusterNode struct {
	NodeID uint64
	Addr   string // 格式: "IP:监控端口"
}

// 分片节点状态
type ShardNodeStatus struct {
	ShardID   uint64 `json:"shardId"`
	NodeID    uint64 `json:"nodeId"`
	HasLeader bool   `json:"hasLeader"` // 对应 has_leader 指标
	Term      uint64 `json:"term"`      // 当前任期
	// 你还可以根据截图里的其他指标继续加字段
}

// 正则适配你真实的指标格式：shardid / replicaid
var (
	regHasLeader = regexp.MustCompile(`dragonboat_raftnode_has_leader\{shardid="(\d+)",replicaid="(\d+)"\}\s+(\d)`)
	regTerm      = regexp.MustCompile(`dragonboat_raftnode_term\{shardid="(\d+)",replicaid="(\d+)"\}\s+(\d+)`)
)

// 单个节点拉取并解析metrics
func fetchSingleNodeMetrics(addr string) ([]ShardNodeStatus, error) {
	url := fmt.Sprintf("http://%s/metrics", addr)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	raw := string(data)

	// 用 map 存解析结果，key 是 "shardid_replicaid"
	dataMap := make(map[string]*ShardNodeStatus)

	// 1. 解析 has_leader 指标
	for _, m := range regHasLeader.FindAllStringSubmatch(raw, -1) {
		shardID, _ := strconv.ParseUint(m[1], 10, 64)
		nodeID, _ := strconv.ParseUint(m[2], 10, 64)
		hasLeader, _ := strconv.Atoi(m[3])
		key := fmt.Sprintf("%d_%d", shardID, nodeID)
		dataMap[key] = &ShardNodeStatus{
			ShardID:   shardID,
			NodeID:    nodeID,
			HasLeader: hasLeader == 1,
		}
	}

	// 2. 解析 term 指标
	for _, m := range regTerm.FindAllStringSubmatch(raw, -1) {
		shardID, _ := strconv.ParseUint(m[1], 10, 64)
		nodeID, _ := strconv.ParseUint(m[2], 10, 64)
		term, _ := strconv.ParseUint(m[3], 10, 64)
		key := fmt.Sprintf("%d_%d", shardID, nodeID)
		if v, ok := dataMap[key]; ok {
			v.Term = term
		}
	}

	// 转成切片返回
	var res []ShardNodeStatus
	for _, v := range dataMap {
		res = append(res, *v)
	}
	return res, nil
}

// 批量采集全集群所有节点的metrics
func FetchAllClusterMetrics(nodes []ClusterNode) ([]ShardNodeStatus, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var total []ShardNodeStatus

	for _, node := range nodes {
		wg.Add(1)
		go func(n ClusterNode) {
			defer wg.Done()
			list, err := fetchSingleNodeMetrics(n.Addr)
			mu.Lock()
			if err != nil {
				// 请求失败，标记节点状态（可选）
				fmt.Printf("节点 %d (%s) 拉取失败: %v\n", n.NodeID, n.Addr, err)
			} else {
				total = append(total, list...)
			}
			mu.Unlock()
		}(node)
	}
	wg.Wait()
	return total, nil
}
