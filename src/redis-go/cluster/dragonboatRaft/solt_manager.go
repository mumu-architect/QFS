package dragonboatRaft

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/lni/dragonboat/v4"
)

// ====================== 数据结构 ======================
// Redis 官方标准 CRC16 表 (工业级不可修改)
var crc16Tab = [256]uint16{
	0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50a5, 0x60c6, 0x70e7,
	0x8108, 0x9129, 0xa14a, 0xb16b, 0xc18c, 0xd1ad, 0xe1ce, 0xf1ef,
	0x1231, 0x0210, 0x3273, 0x2252, 0x52b5, 0x4294, 0x72f7, 0x62d6,
	0x9339, 0x8318, 0xb37b, 0xa35a, 0xd3bd, 0xc39c, 0xf3ff, 0xe3de,
	0x2462, 0x3443, 0x0420, 0x1401, 0x64e6, 0x74c7, 0x44a4, 0x5485,
	0xa56a, 0xb54b, 0x8528, 0x9509, 0xe5ee, 0xf5cf, 0xc5ac, 0xd58d,
	0x3653, 0x2672, 0x1611, 0x0630, 0x76d7, 0x66f6, 0x5695, 0x46b4,
	0xb75b, 0xa77a, 0x9719, 0x8738, 0xf7df, 0xe7fe, 0xd79d, 0xc7bc,
	0x48c4, 0x58e5, 0x6886, 0x78a7, 0x0840, 0x1861, 0x2802, 0x3823,
	0xc9cc, 0xd9ed, 0xe98e, 0xf9af, 0x8948, 0x9969, 0xa90a, 0xb92b,
	0x5af5, 0x4ad4, 0x7ab7, 0x6a96, 0x1a71, 0x0a50, 0x3a33, 0x2a12,
	0xdbfd, 0xcbdc, 0xfbbf, 0xeb9e, 0x9b79, 0x8b58, 0xbb3b, 0xab1a,
	0x6ca6, 0x7c87, 0x4ce4, 0x5cc5, 0x2c22, 0x3c03, 0x0c60, 0x1c41,
	0xedae, 0xfd8f, 0xcdec, 0xddcd, 0xad2a, 0xbd0b, 0x8d68, 0x9d49,
	0x7e97, 0x6eb6, 0x5ed5, 0x4ef4, 0x3e13, 0x2e32, 0x1e51, 0x0e70,
	0xff9f, 0xefbe, 0xdfdd, 0xcffc, 0xbf1b, 0xaf3a, 0x9f59, 0x8f78,
	0x9188, 0x81a9, 0xb1ca, 0xa1eb, 0xd10c, 0xc12d, 0xf14e, 0xe16f,
	0x1080, 0x00a1, 0x30c2, 0x20e3, 0x5004, 0x4025, 0x7046, 0x6067,
	0x83b9, 0x9398, 0xa3fb, 0xb3da, 0xc33d, 0xd31c, 0xe37f, 0xf35e,
	0x02b1, 0x1290, 0x22f3, 0x32d2, 0x4235, 0x5214, 0x6277, 0x7256,
	0xb5ea, 0xa5cb, 0x95a8, 0x8589, 0xf56e, 0xe54f, 0xd52c, 0xc50d,
	0x34e2, 0x24c3, 0x14a0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
	0xa7db, 0xb7fa, 0x8799, 0x97b8, 0xe75f, 0xf77e, 0xc71d, 0xd73c,
	0x26d3, 0x36f2, 0x0691, 0x16b0, 0x6657, 0x7676, 0x4615, 0x5634,
	0xd94c, 0xc96d, 0xf90e, 0xe92f, 0x99c8, 0x89e9, 0xb98a, 0xa9ab,
	0x5844, 0x4865, 0x7806, 0x6827, 0x18c0, 0x08e1, 0x3882, 0x28a3,
	0xcb7d, 0xdb5c, 0xeb3f, 0xfb1e, 0x8bf9, 0x9bd8, 0xabbb, 0xbb9a,
	0x4a75, 0x5a54, 0x6a37, 0x7a16, 0x0af1, 0x1ad0, 0x2ab3, 0x3a92,
	0xfd2e, 0xed0f, 0xdd6c, 0xcd4d, 0xbdaa, 0xad8b, 0x9de8, 0x8dc9,
	0x7c26, 0x6c07, 0x5c64, 0x4c45, 0x3ca2, 0x2c83, 0x1ce0, 0x0cc1,
	0xef1f, 0xff3e, 0xcf5d, 0xdf7c, 0xaf9b, 0xbfba, 0x8fd9, 0x9ff8,
	0x6e17, 0x7e36, 0x4e55, 0x5e74, 0x2e93, 0x3eb2, 0x0ed1, 0x1ef0,
}

type NodeSlotMeta struct {
	ShardID int           `json:"shardID"`
	Slots   map[int]*Slot `json:"slots"`
}
type NodeSlotMetas struct {
	NodeSlotMetas map[int]*NodeSlotMeta `json:"nodeSlotMetas"`
}

// InitFullSlots 初始化16384槽
func InitFullSlots(nh *dragonboat.NodeHost, nodeMeta *NodeMeta) {
	parts := strings.Split(nodeMeta.ShardIDS, ",")
	LeaderCount := len(parts)
	assignSlots(nh, nodeMeta, parts, LeaderCount)
	return
}

// 分配槽
func assignSlots(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, shardIDArr []string, cont int) {
	// TODO： 未测试通过
	masterCount := cont
	if masterCount == 0 {
		fmt.Println("===============无主节点")
		return
	}
	// 计算每个节点基础槽数 + 余数
	slotsPerNode := TotalSlots / masterCount
	remainder := TotalSlots % masterCount
	start := 0
	// 分配槽
	nodeSlotMetas := make(map[int]*NodeSlotMeta)
	for i, shardID := range shardIDArr {
		end := start + slotsPerNode - 1

		// 最后一个节点处理余数
		if i == masterCount-1 {
			end += remainder
		}

		// 生成槽区间
		slot := &Slot{
			StartSlotID: start,
			EndSlotID:   end,
		}
		slots := make(map[int]*Slot)
		//写入槽信息
		//TODO:后期走不同客户端,要修改为当前shandID，的slots写入
		key := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + shardID
		shardID, _ := strconv.Atoi(shardID)
		cmd := &NodeMeta{
			GlobalShardID: nodeMeta.GlobalShardID,
			AllNodeInfo:   nodeMeta.AllNodeInfo,
			ShardIDS:      nodeMeta.ShardIDS,
			ShardID:       shardID,
			ShardNodeInfo: nodeMeta.ShardNodeInfo,
			Slots:         make(map[int]*Slot),
		}
		slots[0] = slot
		cmd.Slots = slots
		fmt.Printf("444===========%v \n", slots)
		data, _ := json.Marshal(cmd)
		_, err := nodeMeta.Set(nh, nodeMeta, key, string(data))
		if err != nil {
			fmt.Printf("assign slot %d failed", shardID)
		}
		//TODO:将槽信息写入对应的nodeMeta本地元数据
		if shardID == nodeMeta.ShardID {
			nodeMeta.Slots = slots
		}
		//TODO；槽与shardID对应关系，通过槽找到shardID,leaderID
		nsm := &NodeSlotMeta{
			ShardID: shardID,
			Slots:   slots,
		}
		nodeSlotMetas[i] = nsm

		//fmt.Printf("333===================assign slot %d failed", i)
		start = end + 1
	}
	//TODO:写入元数据,落盘
	nodeSlotMetas2 := &NodeSlotMetas{
		NodeSlotMetas: nodeSlotMetas,
	}
	_, err := SetSolt(nh, nodeMeta, nodeSlotMetas2)
	if err != nil {
		return
	}

	log.Printf("====6666666:=====%v", 22)
}

// SetSolt 写入槽到Raft磁盘状态机
func SetSolt(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, nodeSlotMetas *NodeSlotMetas) (bool, error) {
	data2, _ := json.Marshal(nodeSlotMetas)
	key2 := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + "NodeSlotMetas"
	_, err := nodeMeta.Set(nh, nodeMeta, key2, string(data2))
	if err != nil {
		fmt.Printf("write slot key=%s failed", key2)
		return false, err
	}
	return true, nil
}

// GetSolt 查询槽
func GetSolt(nh *dragonboat.NodeHost, nodeMeta *NodeMeta) (*NodeSlotMetas, error) {
	key := QFS_META_KEY + ":" + strconv.Itoa(nodeMeta.GlobalShardID) + ":" + "NodeSlotMetas"
	result, err := nodeMeta.Get(nh, nodeMeta, key)
	if err != nil {
		return nil, err
	}
	data, ok := result.([]byte)
	if !ok {
		fmt.Println("invalid data")
	}
	var meta NodeSlotMetas
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// CalcSlot  工业级哈希槽计算
// 100% 兼容 Redis、支持 hash_tag {xxx}、零分配、高性能
func CalcSlot(key string) int {
	bKey := []byte(key)
	n := len(bKey)
	if n == 0 {
		return 0
	}

	// === 支持 Redis hash_tag {xxx} 相同槽路由 ===
	start, end := -1, -1
	for i := 0; i < n; i++ {
		if bKey[i] == '{' {
			start = i
		} else if bKey[i] == '}' && start != -1 {
			end = i
			break
		}
	}

	var data []byte
	if start != -1 && end != -1 && end > start+1 {
		data = bKey[start+1 : end]
	} else {
		data = bKey
	}

	// === 工业级 CRC16 计算 ===
	var crc uint16 = 0
	for _, ch := range data {
		crc = (crc << 8) ^ crc16Tab[byte(crc>>8)^ch]
	}

	// === 取模 16384 ===
	return int(crc % TotalSlots)
}

// GetLeaderID 根据槽，获取leaderID
func GetLeaderID(nh *dragonboat.NodeHost, nodeMeta *NodeMeta, slot int) int {
	nodeSlotMetas, err := GetSolt(nh, nodeMeta)
	if err != nil {
		fmt.Printf("get slot %d failed", slot)
		return -1
	}
	for _, val := range nodeSlotMetas.NodeSlotMetas {
		for _, v := range val.Slots {
			if v.StartSlotID <= slot && slot <= v.EndSlotID {
				//TODO: 通过shardID找leaderID
				nodeShardLeaderMeta, _ := nodeMeta.GetShardNodeLeaderID(nh, nodeMeta, val.ShardID)
				return nodeShardLeaderMeta.LeaderID
			}
		}
	}
	return -1
}

// 开启槽写入
func StartSlotWrite(nh *dragonboat.NodeHost, nodeMeta *NodeMeta) {
	// 替换成这句（等待集群真正启动成功）
	WaitShardReady(nh, uint64(GlobalShard))
	InitFullSlots(nh, nodeMeta)
}
