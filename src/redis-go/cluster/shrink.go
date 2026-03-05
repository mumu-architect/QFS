package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// 缩容核心逻辑：移除目标主节点，将其槽位迁移至其他主节点，最后下线节点
func (c *Cluster) shrinkCluster(removeMasterID string) error {
	c.reshardingLock.Lock()
	defer c.reshardingLock.Unlock()

	c.mu.RLock()
	// 1. 校验目标节点（必须是主节点且存在）
	removeMaster, exists := c.Nodes[removeMasterID]
	if !exists {
		c.mu.RUnlock()
		return fmt.Errorf("待移除节点 %s 不存在", removeMasterID)
	}
	if removeMaster.Type != Master {
		c.mu.RUnlock()
		return fmt.Errorf("待移除节点 %s 不是主节点", removeMaster.Addr)
	}
	if len(removeMaster.Slots) == 0 {
		c.mu.RUnlock()
		return fmt.Errorf("待移除节点 %s 无分配槽位，无需缩容", removeMaster.Addr)
	}

	// 2. 收集剩余主节点（用于接收迁移的槽位）
	remainingMasters := make([]*Node, 0)
	for _, node := range c.Nodes {
		if node.Type == Master && node.ID != removeMasterID && node.Status == Online {
			remainingMasters = append(remainingMasters, node)
		}
	}
	if len(remainingMasters) == 0 {
		c.mu.RUnlock()
		return fmt.Errorf("无可用剩余主节点，无法接收槽位")
	}
	c.mu.RUnlock()

	log.Printf("开始缩容：移除主节点 %s，槽位数量：%d，目标接收主节点数：%d",
		removeMaster.Addr, len(removeMaster.Slots), len(remainingMasters))

	// 3. 均分槽位到剩余主节点
	slotPerMaster := len(removeMaster.Slots) / len(remainingMasters)
	extraSlots := len(removeMaster.Slots) % len(remainingMasters)
	slotIndex := 0

	for i, targetMaster := range remainingMasters {
		// 计算当前主节点需要接收的槽位数量
		receiveCount := slotPerMaster
		if i < extraSlots {
			receiveCount += 1
		}
		if receiveCount <= 0 {
			continue
		}

		// 截取需要迁移的槽位
		endIndex := slotIndex + receiveCount
		if endIndex > len(removeMaster.Slots) {
			endIndex = len(removeMaster.Slots)
		}
		slotsToMigrate := removeMaster.Slots[slotIndex:endIndex]
		slotIndex = endIndex

		// 逐个迁移槽位
		var wg sync.WaitGroup
		for _, slot := range slotsToMigrate {
			wg.Add(1)
			go func(s int, target *Node) {
				defer wg.Done()
				err := c.migrateSlotToMaster(removeMaster, target, s)
				if err != nil {
					log.Printf("槽位 %d 从 %s 迁移到 %s 失败：%v", s, removeMaster.Addr, target.Addr, err)
				} else {
					log.Printf("槽位 %d 从 %s 迁移到 %s 成功", s, removeMaster.Addr, target.Addr)
				}
			}(slot, targetMaster)
		}
		wg.Wait()
	}

	// 4. 验证所有槽位迁移完成
	c.mu.Lock()
	if len(removeMaster.Slots) > 0 {
		c.mu.Unlock()
		return fmt.Errorf("部分槽位迁移失败，剩余未迁移槽位：%d", len(removeMaster.Slots))
	}
	c.mu.Unlock()

	// 5. 下线目标节点（标记为离线+广播状态+关闭节点）
	err := c.offlineMasterNode(removeMasterID)
	if err != nil {
		return fmt.Errorf("节点下线失败：%v", err)
	}

	log.Printf("缩容成功：主节点 %s 已移除，所有槽位迁移完成", removeMaster.Addr)
	return nil
}

// migrateSlotToMaster 单个槽位从源主节点迁移到目标主节点
func (c *Cluster) migrateSlotToMaster(sourceMaster, targetMaster *Node, slot int) error {
	// 1. 标记槽位为迁移中
	c.mu.Lock()
	c.migratingSlots[slot] = targetMaster.ID
	c.mu.Unlock()

	// 2. 迁移槽位数据（源主节点 -> 目标主节点）
	slotData := c.getSlotData(slot)
	err := c.pushSlotData(targetMaster.Addr, slot, slotData)
	if err != nil {
		return fmt.Errorf("数据迁移失败：%v", err)
	}

	// 3. 切换槽位映射（更新集群内所有节点的槽位归属）
	c.mu.Lock()
	// 更新槽位映射
	c.slotMap[slot] = targetMaster.ID
	// 目标主节点添加槽位
	targetMaster.Slots = append(targetMaster.Slots, slot)
	// 源主节点移除槽位
	for i, s := range sourceMaster.Slots {
		if s == slot {
			sourceMaster.Slots = append(sourceMaster.Slots[:i], sourceMaster.Slots[i+1:]...)
			break
		}
	}
	// 清除迁移状态
	delete(c.migratingSlots, slot)
	c.mu.Unlock()

	// 4. 广播集群状态（确保所有节点同步槽位变更）
	c.broadcastClusterState()
	return nil
}

// offlineMasterNode 下线主节点（标记状态+关闭服务）
func (c *Cluster) offlineMasterNode(masterID string) error {
	c.mu.Lock()
	masterNode, exists := c.Nodes[masterID]
	if !exists {
		c.mu.Unlock()
		return fmt.Errorf("节点不存在")
	}
	// 标记为离线
	masterNode.Status = Offline
	c.mu.Unlock()

	// 广播节点下线状态
	c.broadcastClusterState()

	// 发送关闭命令到目标节点
	url := fmt.Sprintf("http://localhost%s/%s/shutdown", masterNode.Addr, masterNode.Addr)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("发送关闭命令到 %s 失败（可能已离线）：%v", masterNode.Addr, err)
		return nil // 允许节点已离线的情况
	}
	defer resp.Body.Close()

	var result string
	json.NewDecoder(resp.Body).Decode(&result)
	log.Printf("节点 %s 关闭结果：%s", masterNode.Addr, result)
	return nil
}

// registerShrinkHandlers 注册缩容相关HTTP接口
func (c *Cluster) registerShrinkHandlers() {
	// 触发集群缩容（指定要移除的主节点ID）
	http.HandleFunc(fmt.Sprintf("/%s/shrink", c.LocalNode.Addr), func(w http.ResponseWriter, r *http.Request) {
		removeMasterID := r.URL.Query().Get("masterID")
		if removeMasterID == "" {
			http.Error(w, "待移除主节点ID不能为空", http.StatusBadRequest)
			return
		}

		// 校验当前节点是否有权限（必须是主节点）
		c.mu.RLock()
		isMaster := c.LocalNode.Type == Master
		c.mu.RUnlock()
		if !isMaster {
			http.Error(w, "仅主节点可发起缩容操作", http.StatusForbidden)
			return
		}

		err := c.shrinkCluster(removeMasterID)
		if err != nil {
			http.Error(w, fmt.Sprintf("缩容失败：%v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode("缩容成功")
	})

	// 查询所有主节点信息（便于选择缩容目标）
	http.HandleFunc(fmt.Sprintf("/%s/listMasters", c.LocalNode.Addr), func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		masters := make(map[string]map[string]interface{})
		for id, node := range c.Nodes {
			if node.Type == Master {
				masters[id] = map[string]interface{}{
					"addr":      node.Addr,
					"status":    node.Status,
					"slotCount": len(node.Slots),
				}
			}
		}
		c.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masters)
	})
}
