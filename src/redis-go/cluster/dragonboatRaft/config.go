package dragonboatRaft

const (
	Cluster1   uint64 = 1
	Cluster2   uint64 = 2
	TotalSlots        = 16384
)

var PeerAddrs = map[uint64]string{
	1: "127.0.0.1:9001",
	2: "127.0.0.1:9002",
	3: "127.0.0.1:9003",
	4: "127.0.0.1:9004",
	5: "127.0.0.1:9005",
	6: "127.0.0.1:9006",
}

// ==========================================
// 【核心】启动时只加载 LeaderID 节点
// 确保启动就固定 LeaderID，不选举、不漂移
// ==========================================
func GetInitialLeaderPeers(clusterID uint64) map[uint64]string {
	m := make(map[uint64]string)
	if clusterID == Cluster1 {
		m[1] = PeerAddrs[1] // 只启动 node1 作为 LeaderID
	} else {
		m[4] = PeerAddrs[4] // 只启动 node4 作为 LeaderID
	}
	return m
}

func GetCluster(nodeID uint64) uint64 {
	if nodeID <= 3 {
		return Cluster1
	}
	return Cluster2
}

func GetPeers(cid uint64) map[uint64]string {
	m := make(map[uint64]string)
	if cid == Cluster1 {
		m[1] = PeerAddrs[1]
		m[2] = PeerAddrs[2]
		m[3] = PeerAddrs[3]
	} else {
		m[4] = PeerAddrs[4]
		m[5] = PeerAddrs[5]
		m[6] = PeerAddrs[6]
	}
	return m
}
