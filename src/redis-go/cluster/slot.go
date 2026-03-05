package cluster

import "log"

// calcSlot 计算key对应的哈希槽（与Redis一致：CRC16简化版）
func (c *Cluster) calcSlot(key string) int {
	crc := 0
	for _, ch := range key {
		crc = (crc << 8) ^ int(ch)
	}
	return crc % TotalSlots
}

//	initSlots 主节点初始化哈希槽（3主均分16384槽）
//
// TODO: 初始化槽位根据集群中主节点平分槽位
func (c *Cluster) initSlots() {
	log.Printf("初始化槽位映射，本地节点地址：%s", c.LocalNode.Addr)
	slotStep := TotalSlots / 1
	startSlot, endSlot := 0, 0
	switch {
	case c.LocalNode.Addr == ":9001": // 主1：0-5461
		startSlot, endSlot = 0, slotStep-1
		log.Printf("主节点 :9001 负责的槽位范围：%d-%d", startSlot, endSlot)
	default: // 新增主节点：初始无槽位
		log.Printf("新增主节点，初始无槽位")
		return
	}
	for i := startSlot; i <= endSlot; i++ {
		c.LocalNode.Slots = append(c.LocalNode.Slots, i)
		c.slotMap[i] = c.LocalNode.ID
	}
	log.Printf("槽位映射初始化完成，共映射 %d 个槽位", len(c.LocalNode.Slots))
}
