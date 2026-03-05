package cluster

import (
	"fmt"
	"log"
	"sync"
)

// 扩容核心逻辑：将现有主节点的槽位均分至新主节点，同步数据并更新集群映射
func (c *Cluster) scaleCluster(newMasterID string) error {
	c.reshardingLock.Lock()
	defer c.reshardingLock.Unlock()

	c.mu.RLock()
	// 1. 校验新主节点（必须存在、是主节点且初始无槽位）
	newMaster, exists := c.Nodes[newMasterID]
	if !exists {
		c.mu.RUnlock()
		return fmt.Errorf("新主节点 %s 不存在", newMasterID)
	}
	if newMaster.Type != Master {
		c.mu.RUnlock()
		return fmt.Errorf("节点 %s 不是主节点，无法参与扩容", newMaster.Addr)
	}
	if len(newMaster.Slots) > 0 {
		c.mu.RUnlock()
		return fmt.Errorf("新主节点 %s 已分配槽位，无需重复扩容", newMaster.Addr)
	}

	// 2. 收集现有在线主节点（排除新主节点）
	existingMasters := make([]*Node, 0)
	for _, node := range c.Nodes {
		if node.Type == Master && node.ID != newMasterID && node.Status == Online {
			existingMasters = append(existingMasters, node)
		}
	}
	if len(existingMasters) == 0 {
		c.mu.RUnlock()
		return fmt.Errorf("无可用现有主节点，无法进行槽位迁移")
	}
	c.mu.RUnlock()

	log.Printf("开始扩容：新主节点 %s，现有主节点数：%d，总主节点数：%d",
		newMaster.Addr, len(existingMasters), len(existingMasters)+1)

	// 3. 计算每个主节点需迁移的槽位数量（总槽位均分）
	totalSlots := TotalSlots
	totalMasters := len(existingMasters) + 1
	slotsPerMaster := totalSlots / totalMasters
	remainingSlots := totalSlots % totalMasters // 余数槽位优先分配给前N个主节点

	// 4. 逐个现有主节点迁移槽位
	var globalWg sync.WaitGroup
	for masterIdx, master := range existingMasters {
		c.mu.RLock()
		// 复制当前主节点槽位（避免循环中修改原切片）
		currentSlots := make([]int, len(master.Slots))
		copy(currentSlots, master.Slots)
		c.mu.RUnlock()

		// 计算当前主节点需迁移的槽位数量
		migrateCount := slotsPerMaster
		if masterIdx < remainingSlots {
			migrateCount += 1
		}
		if migrateCount <= 0 || len(currentSlots) <= 0 {
			log.Printf("主节点 %s 无需迁移槽位", master.Addr)
			continue
		}

		// 截取需迁移的槽位（从末尾截取，不影响前端读写路由）
		startIdx := len(currentSlots) - migrateCount
		if startIdx < 0 {
			startIdx = 0
			migrateCount = len(currentSlots)
		}
		slotsToMigrate := currentSlots[startIdx:]

		log.Printf("主节点 %s 需迁移槽位数量：%d，槽位列表：%v",
			master.Addr, migrateCount, slotsToMigrate)

		// 并行迁移单个主节点的多个槽位
		for _, slot := range slotsToMigrate {
			globalWg.Add(1)
			go func(s int, sourceMaster *Node) {
				defer globalWg.Done()
				err := c.migrateSlotForScale(sourceMaster, newMaster, s)
				if err != nil {
					log.Printf("扩容槽位 %d 迁移失败：%v", s, err)
				} else {
					log.Printf("扩容槽位 %d 迁移成功（%s -> %s）", s, sourceMaster.Addr, newMaster.Addr)
				}
			}(slot, master)
		}
	}

	// 等待所有槽位迁移完成
	globalWg.Wait()

	// 5. 校验扩容结果（新主节点槽位数量是否符合预期）
	c.mu.RLock()
	actualSlotsCount := len(newMaster.Slots)
	expectedSlotsCount := slotsPerMaster*len(existingMasters) + remainingSlots
	c.mu.RUnlock()

	if actualSlotsCount != expectedSlotsCount {
		return fmt.Errorf("扩容校验失败：预期槽位 %d，实际槽位 %d", expectedSlotsCount, actualSlotsCount)
	}

	log.Printf("扩容成功：新主节点 %s 已分配 %d 个槽位，集群总主节点数：%d",
		newMaster.Addr, actualSlotsCount, totalMasters)
	return nil
}

// migrateSlotForScale 扩容场景下的单个槽位迁移（源主节点 -> 新主节点）
func (c *Cluster) migrateSlotForScale(sourceMaster, newMaster *Node, slot int) error {
	// 1. 标记槽位为迁移中（防止并发操作）
	c.mu.Lock()
	c.migratingSlots[slot] = newMaster.ID
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.migratingSlots, slot)
		c.mu.Unlock()
	}()

	// 2. 提取槽位数据（源主节点）
	slotData := c.getSlotData(slot)
	log.Printf("槽位 %d 待迁移数据量：%d 条", slot, len(slotData))

	// 3. 推送数据到新主节点
	err := c.pushSlotData(newMaster.Addr, slot, slotData)
	if err != nil {
		return fmt.Errorf("数据推送失败：%v", err)
	}

	// 4. 更新集群槽位映射与节点槽位列表
	c.mu.Lock()
	// 新主节点添加槽位
	newMaster.Slots = append(newMaster.Slots, slot)
	// 源主节点移除槽位
	for i, s := range sourceMaster.Slots {
		if s == slot {
			sourceMaster.Slots = append(sourceMaster.Slots[:i], sourceMaster.Slots[i+1:]...)
			break
		}
	}
	// 更新全局槽位映射
	c.slotMap[slot] = newMaster.ID
	c.mu.Unlock()

	// 5. 广播集群状态（确保所有节点同步槽位变更）
	c.broadcastClusterState()
	return nil
}
