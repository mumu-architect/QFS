package uploadManager

import (
	"strconv"
	"strings"

	"mumu.com/redis-go/distributedId/snowflake"
)

type Node struct {
	ShardID int    `json:"shardID"`
	NodeID  int    `json:"id"`   // 节点唯一ID
	IP      string `json:"ip"`   // 地址（IP）
	Port    int    `json:"port"` // 地址（Port）
}

// FileManager 文件管理器
type NodeUpload struct {
	ShardId           int                          `json:"shardId"`
	NodeId            int                          `json:"nodeId"`
	UploadIp          string                       `json:"fileIp"`
	UploadPort        int                          `json:"filePort"`
	LeaderId          int                          `json:"leaderId"`
	ShardNodeInfo     map[int]*Node                `json:"shardNodeInfo"`
	AllNodeInfos      string                       `json:"allNodeInfos"`
	LocalRoot         string                       `json:"localRoot"`
	SnowflakeGenerate *snowflake.SnowFlakeGenerate `json:"snowflakeGenerate"`
}

func NewNodeUpload(shardId int, nodeId int, leaderId int, uploadIp string, uploadPort int, uploadPeers string, nodeInfos string) *NodeUpload {
	shardNodeInfo := stringToMapNodes(shardId, uploadPeers)
	snowflakeGenerate := snowflake.NewSnowFlakeGenerate()
	nu := &NodeUpload{
		ShardId:           shardId,
		NodeId:            nodeId,
		UploadIp:          uploadIp,
		UploadPort:        uploadPort,
		LeaderId:          leaderId,
		ShardNodeInfo:     shardNodeInfo,
		AllNodeInfos:      nodeInfos,
		SnowflakeGenerate: snowflakeGenerate,
	}

	return nu
}

// ---------------------- 你已有的Raft集群方法（自行替换成真实实现）----------------------

func RandomStr(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[Intn(len(letterBytes))]
	}
	return string(b)
}

func Intn(max int) int {
	return 0
}

// 字符串转map的node节点
func stringToMapNodes(shardID int, peers string) map[int]*Node {
	var shardNodeInfo = make(map[int]*Node)
	peerList := strings.Split(peers, ",")
	for _, peer := range peerList {
		parts := strings.Split(peer, "=")
		if len(parts) != 2 {
			continue
		}
		peerNodeID, _ := strconv.Atoi(parts[0])
		nodeAddr := parts[1]
		parts2 := strings.Split(nodeAddr, ":")
		if len(parts) != 2 {
			continue
		}
		peerNodeIP := parts2[0]
		peerNodePort, _ := strconv.Atoi(parts2[1])
		peerNodeInfo := &Node{
			ShardID: shardID,
			NodeID:  peerNodeID,
			IP:      peerNodeIP,
			Port:    peerNodePort,
		}
		//fmt.Printf("111====%d======%v \n", peerNodeID, peerNodeInfo)
		shardNodeInfo[peerNodeID] = peerNodeInfo
	}
	return shardNodeInfo
}
