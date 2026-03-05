package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// registerSyncHandlers 注册同步相关HTTP接口
func (c *Cluster) registerSyncHandlers() {
	// 1. 从节点请求主节点同步（带偏移量，支持断点续传）
	c.serveMux.HandleFunc("/syncWithOffset", func(w http.ResponseWriter, r *http.Request) {
		// 解析请求参数
		offsetStr := r.URL.Query().Get("offset")
		batchSizeStr := r.URL.Query().Get("batchSize")

		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			offset = 0
		}

		batchSize, err := strconv.Atoi(batchSizeStr)
		if err != nil || batchSize <= 0 {
			batchSize = SyncBatchSize
		}

		// 生成批次同步数据
		syncResp := c.generateBatchSyncData(offset, batchSize)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(syncResp)
	})

	// 2. 从节点接收主节点推送的增量数据（支持 String/Hash）
	c.serveMux.HandleFunc("/syncData", func(w http.ResponseWriter, r *http.Request) {
		// 解析通用参数
		dataType := r.URL.Query().Get("type") // string 或 hash
		key := r.URL.Query().Get("key")
		val := r.URL.Query().Get("val")
		field := r.URL.Query().Get("field") // Hash 专属参数

		if dataType == "" || key == "" {
			http.Error(w, "type和key不能为空", http.StatusBadRequest)
			return
		}

		// 按类型写入数据
		c.mu.Lock()
		defer c.mu.Unlock()
		switch dataType {
		case "string":
			c.dataStore.(RedisData).String[key] = val
			c.recentChanges[key] = time.Now()
		case "hash":
			if field == "" {
				http.Error(w, "hash类型必须指定field", http.StatusBadRequest)
				return
			}
			if _, exists := c.dataStore.(RedisData).Hash[key]; !exists {
				c.dataStore.(RedisData).Hash[key] = make(map[string]string)
			}
			c.dataStore.(RedisData).Hash[key][field] = val
			c.recentChanges[fmt.Sprintf("hash:%s:%s", key, field)] = time.Now()
		default:
			http.Error(w, "不支持的数据类型："+dataType, http.StatusBadRequest)
			return
		}

		// 从节点数据落盘
		go c.persistSlaveData()

		json.NewEncoder(w).Encode("同步成功")
	})

	// 3. 迁移槽位数据（原主节点向新主节点推送）
	c.serveMux.HandleFunc("/migrateSlotData", func(w http.ResponseWriter, r *http.Request) {
		slotStr := r.URL.Query().Get("slot")
		targetAddr := r.URL.Query().Get("targetAddr")

		slot, err := strconv.Atoi(slotStr)
		if err != nil || slot < 0 || slot >= TotalSlots {
			http.Error(w, "无效的槽位", http.StatusBadRequest)
			return
		}

		if targetAddr == "" {
			http.Error(w, "目标节点地址不能为空", http.StatusBadRequest)
			return
		}

		// 筛选该槽位的所有数据
		slotData := c.getSlotData(slot)
		if len(slotData) == 0 {
			log.Printf("槽位 %d 无数据可迁移", slot)
			json.NewEncoder(w).Encode("迁移成功（无数据）")
			return
		}

		// 推送到目标节点
		err = c.pushSlotData(targetAddr, slot, slotData)
		if err != nil {
			http.Error(w, fmt.Sprintf("数据推送失败：%v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode("迁移成功")
	})

	// 4. 接收迁移数据（新主节点导入，支持 String/Hash）
	c.serveMux.HandleFunc("/importSlotData", func(w http.ResponseWriter, r *http.Request) {
		slotStr := r.URL.Query().Get("slot")
		var slotData map[string]interface{}

		slot, err := strconv.Atoi(slotStr)
		if err != nil || slot < 0 || slot >= TotalSlots {
			http.Error(w, "无效的槽位", http.StatusBadRequest)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&slotData); err != nil {
			http.Error(w, fmt.Sprintf("数据解析失败：%v", err), http.StatusBadRequest)
			return
		}

		// 导入 String 数据
		if stringData, exists := slotData["string"].(map[string]interface{}); exists {
			c.mu.Lock()
			for k, v := range stringData {
				c.dataStore.(RedisData).String[k] = fmt.Sprintf("%v", v)
				c.recentChanges[k] = time.Now()
			}
			c.mu.Unlock()
		}

		// 导入 Hash 数据
		if hashData, exists := slotData["hash"].(map[string]interface{}); exists {
			c.mu.Lock()
			for key, fieldMap := range hashData {
				if _, exists := c.dataStore.(RedisData).Hash[key]; !exists {
					c.dataStore.(RedisData).Hash[key] = make(map[string]string)
				}
				for field, val := range fieldMap.(map[string]interface{}) {
					c.dataStore.(RedisData).Hash[key][field] = fmt.Sprintf("%v", val)
					c.recentChanges[fmt.Sprintf("hash:%s:%s", key, field)] = time.Now()
				}
			}
			c.mu.Unlock()
		}

		// 标记导入状态
		c.mu.Lock()
		c.importingSlots[slot] = r.RemoteAddr
		c.mu.Unlock()

		log.Printf("节点 %s 导入槽位 %d 数据（String:%d 条，Hash:%d 个）",
			c.LocalNode.Addr, slot,
			len(slotData["string"].(map[string]interface{})),
			len(slotData["hash"].(map[string]interface{})))
		json.NewEncoder(w).Encode("导入成功")
	})

	// 5. 确认槽位迁移完成
	c.serveMux.HandleFunc("/confirmSlotMigration", func(w http.ResponseWriter, r *http.Request) {
		slotStr := r.URL.Query().Get("slot")
		newMasterID := r.URL.Query().Get("newMasterID")

		slot, err := strconv.Atoi(slotStr)
		if err != nil || slot < 0 || slot >= TotalSlots {
			http.Error(w, "无效的槽位", http.StatusBadRequest)
			return
		}

		if newMasterID == "" {
			http.Error(w, "新主节点ID不能为空", http.StatusBadRequest)
			return
		}

		// 切换槽位映射
		c.mu.Lock()
		c.slotMap[slot] = newMasterID
		// 清除迁移状态
		delete(c.migratingSlots, slot)
		delete(c.importingSlots, slot)
		c.mu.Unlock()

		// 广播集群状态
		c.broadcastClusterState()
		log.Printf("槽位 %d 迁移确认完成，新主节点：%s", slot, newMasterID)
		json.NewEncoder(w).Encode("确认成功")
	})
}

// generateBatchSyncData 生成批次同步数据（支持断点续传）
func (c *Cluster) generateBatchSyncData(offset int64, batchSize int) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 提取所有key并排序（保证偏移量一致性）
	redisData := c.dataStore.(RedisData)
	keys := make([]string, 0, len(redisData.String))
	for k := range redisData.String {
		keys = append(keys, k)
	}
	// 简单排序（确保每次返回顺序一致）
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// 计算批次范围
	start := int(offset)
	end := start + batchSize
	if end > len(keys) {
		end = len(keys)
	}

	// 构建批次数据
	batchData := make(map[string]string, end-start)
	for i := start; i < end; i++ {
		k := keys[i]
		batchData[k] = redisData.String[k]
	}

	return map[string]interface{}{
		"data":     batchData,
		"offset":   int64(end),
		"finished": end >= len(keys),
	}
}

// syncFromMaster 从节点初始化同步（全量+增量+断点续传）
func (c *Cluster) syncFromMaster(masterAddr string) error {
	log.Printf("从节点 %s 开始同步主节点 %s", c.LocalNode.Addr, masterAddr)

	// 循环同步直到完成全量数据
	for {
		// 带偏移量请求同步数据
		//TODO:
		url := fmt.Sprintf("http://%s/syncWithOffset?offset=%d&batchSize=%d",
			masterAddr, c.lastSyncOffset, SyncBatchSize)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("同步请求失败：%v，1秒后重试", err)
			time.Sleep(1 * time.Second)
			continue
		}

		var syncResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
			log.Printf("响应解析失败：%v，1秒后重试", err)
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		// 解析批次数据
		batchData := syncResp["data"].(map[string]interface{})
		// JSON 解析的数字默认是 float64，需要先转换
		offsetFloat := syncResp["offset"].(float64)
		c.lastSyncOffset = int64(offsetFloat)
		finished := syncResp["finished"].(bool)

		// 写入本地存储
		c.mu.Lock()
		for k, v := range batchData {
			c.setStringData(k, v.(string))
		}
		c.mu.Unlock()

		log.Printf("从节点 %s 同步批次完成：偏移量=%d，数据量=%d",
			c.LocalNode.Addr, c.lastSyncOffset, len(batchData))

		// 全量同步完成则退出循环
		if finished {
			c.mu.RLock()
			redisData := c.dataStore.(RedisData)
			log.Printf("从节点 %s 全量同步完成（共%d条数据）", c.LocalNode.Addr, len(redisData.String))
			c.mu.RUnlock()
			// 全量同步完成后持久化数据
			go c.persistSlaveData()
			break
		}

		// 批次间隔，避免压垮主节点
		time.Sleep(100 * time.Millisecond)
	}

	// 启动增量同步循环
	go c.incrementalSyncLoop(masterAddr)
	return nil
}

// incrementalSyncLoop 增量同步循环（拉取+推送双模式）
func (c *Cluster) incrementalSyncLoop(masterAddr string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// 拉取增量变更（兜底，防止推送丢失）
			changes, err := c.fetchMasterChanges(masterAddr)
			if err != nil {
				log.Printf("增量拉取失败：%v", err)
				continue
			}

			// 应用增量变更
			c.mu.Lock()
			for k, v := range changes {
				// 处理不同类型的增量数据
				if changeData, ok := v.(map[string]interface{}); ok {
					switch changeData["type"].(string) {
					case "string":
						if val, ok := changeData["val"].(string); ok {
							c.setStringData(k, val)
						}
					case "hash":
						if key, ok := changeData["key"].(string); ok {
							if field, ok := changeData["field"].(string); ok {
								if val, ok := changeData["val"].(string); ok {
									c.setHashData(key, field, val)
								}
							}
						}
					}
				}
			}
			c.mu.Unlock()

			if len(changes) > 0 {
				log.Printf("从节点 %s 增量同步 %d 条数据", c.LocalNode.Addr, len(changes))
				// 增量数据落盘
				go c.persistSlaveData()
			}
		}
	}
}

// fetchMasterChanges 从主节点拉取增量变更
func (c *Cluster) fetchMasterChanges(masterAddr string) (map[string]interface{}, error) {
	//TODO:
	// url := fmt.Sprintf("http://localhost%s/incrementalChanges", masterAddr)
	url := fmt.Sprintf("http://%s/incrementalChanges", masterAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	var changes map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return nil, fmt.Errorf("解析失败：%v", err)
	}
	return changes, nil
}

// syncToReplicas 主节点推送数据到所有从节点
// 适配：更新 syncToReplicas 支持 String/Hash 双类型推送
func (c *Cluster) syncToReplicas(changeKey, val string) {
	c.mu.RLock()
	replicas := c.replicas[c.LocalNode.ID]
	redisData := c.dataStore.(RedisData)
	c.mu.RUnlock()

	if len(replicas) == 0 {
		return
	}

	// 解析变更类型（String 直接是 key，Hash 是 hash:key:field）
	var dataType, key, field string
	if strings.HasPrefix(changeKey, "hash:") {
		parts := strings.Split(changeKey, ":")
		if len(parts) != 3 {
			log.Printf("无效的Hash变更key：%s", changeKey)
			return
		}
		dataType = "hash"
		key = parts[1]
		field = parts[2]
		// 重新获取val（避免val参数仅传值，丢失key-field关联）
		val, _ = redisData.Hash[key][field]
	} else {
		dataType = "string"
		key = changeKey
		// 重新获取val（确保同步最新值）
		val, _ = redisData.String[key]
	}

	// 并行同步到所有从节点
	var wg sync.WaitGroup
	for _, replica := range replicas {
		if replica.Status == Online {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				//TODO: 构建带类型参数的同步URL
				url := fmt.Sprintf("http://%s/syncData?type=%s&key=%s&field=%s&val=%s",
					replica.Addr, dataType, key, field, val)
				//url := fmt.Sprintf("http://127.0.0.1:9000/syncData?type=%s&key=%s&field=%s&val=%s",
				//	dataType, key, field, val)
				resp, err := http.Get(url)
				if err != nil {
					log.Printf("同步到从节点 %s 失败：%v", addr, err)
					return
				}
				defer resp.Body.Close()
				var result string
				json.NewDecoder(resp.Body).Decode(&result)
				log.Printf("同步到从节点 %s 结果：%s（%s:%s）", addr, result, dataType, key)
			}(replica.Addr)
		}
	}
	wg.Wait()
}

// getSlotData 获取指定槽位的所有数据
// 适配：更新 getSlotData 支持 String/Hash 槽位数据筛选
func (c *Cluster) getSlotData(slot int) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	slotData := make(map[string]interface{})
	redisData := c.dataStore.(RedisData)

	// 筛选 String 类型（按 key 计算槽位）
	stringData := make(map[string]string)
	for k, v := range redisData.String {
		if c.calcSlot(k) == slot {
			stringData[k] = v
		}
	}
	if len(stringData) > 0 {
		slotData["string"] = stringData
	}

	// 筛选 Hash 类型（按 key 计算槽位）
	hashData := make(map[string]map[string]string)
	for k, fieldMap := range redisData.Hash {
		if c.calcSlot(k) == slot {
			hashData[k] = fieldMap
		}
	}
	if len(hashData) > 0 {
		slotData["hash"] = hashData
	}

	return slotData
}

// pushSlotData 推送槽位数据到目标节点
// 适配：更新 pushSlotData 支持 String/Hash 槽位数据迁移
func (c *Cluster) pushSlotData(targetAddr string, slot int, data map[string]interface{}) error {
	url := fmt.Sprintf("http://%s/%s/importSlotData?slot=%d", targetAddr, targetAddr, slot)
	reqBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("数据序列化失败：%v", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("请求目标节点失败：%v", err)
	}
	defer resp.Body.Close()

	var result string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("响应解析失败：%v", err)
	}

	if result != "导入成功" {
		return fmt.Errorf("目标节点导入失败：%s", result)
	}
	return nil
}
