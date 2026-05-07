package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// registerAllHandlers 注册所有HTTP接口
// registerAllHandlers 注册所有HTTP接口（新增缩容接口）
func (c *Cluster) registerAllHandlers() {
	c.registerDataHandlers()
	c.registerSyncHandlers()
}

// registerDataHandlers 注册数据操作接口（新增HGet/HSet）
func (c *Cluster) registerDataHandlers() {
	// 1. String - Set
	c.serveMux.HandleFunc("/Set", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		val := r.URL.Query().Get("val")

		if key == "" {
			http.Error(w, "key不能为空", http.StatusBadRequest)
			return
		}

		//err := c.Set(key, val)
		err := c.Set(w, r, key, val)
		if err != nil {
			http.Error(w, fmt.Sprintf("Set失败：%v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"result": "Set成功"})
		//json.NewEncoder(w).Encode("Set成功")
	})

	// 2. String - Get
	c.serveMux.HandleFunc("/Get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key不能为空", http.StatusBadRequest)
			return
		}

		val, err := c.Get(w, r, key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Get失败：%v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"key": key, "val": val})
	})

	// 3. Hash - HSet（新增）
	c.serveMux.HandleFunc("/HSet", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		field := r.URL.Query().Get("field")
		val := r.URL.Query().Get("val")
		if key == "" || field == "" {
			http.Error(w, "key和field不能为空", http.StatusBadRequest)
			return
		}

		err := c.HSet(w, r, key, field, val)
		if err != nil {
			http.Error(w, fmt.Sprintf("HSet失败：%v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "HSet成功"})
	})

	// 4. Hash - HGet（新增）
	c.serveMux.HandleFunc("/HGet", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		field := r.URL.Query().Get("field")
		if key == "" || field == "" {
			http.Error(w, "key和field不能为空", http.StatusBadRequest)
			return
		}

		val, err := c.HGet(w, r, key, field)
		if err != nil {
			http.Error(w, fmt.Sprintf("HGet失败：%v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"key": key, "val": val})
		//json.NewEncoder(w).Encode(val)
	})
	// 3. Hash - HMSet（批量设置）
	c.serveMux.HandleFunc("/HMSet", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		// 字段值对格式：field1=val1&field2=val2（支持多个）
		fieldVals := make(map[string]string)
		for k, v := range r.URL.Query() {
			if k != "key" && len(v) > 0 {
				fieldVals[k] = v[0]
			}
		}

		if key == "" || len(fieldVals) == 0 {
			http.Error(w, "key和至少一个field-val对不能为空", http.StatusBadRequest)
			return
		}

		err := c.HMSet(w, r, key, fieldVals)
		if err != nil {
			http.Error(w, fmt.Sprintf("HMSet失败：%v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"result": "HMSet成功"})
		//json.NewEncoder(w).Encode("HMSet成功")
	})

	// 4. Hash - HMGet（批量获取）
	c.serveMux.HandleFunc("/HMGet", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		fieldsStr := r.URL.Query().Get("fields") // 格式：field1,field2,field3
		if key == "" || fieldsStr == "" {
			http.Error(w, "key和fields不能为空", http.StatusBadRequest)
			return
		}
		fields := strings.Split(fieldsStr, ",")

		valMap, err := c.HMGet(w, r, key, fields)
		if err != nil {
			http.Error(w, fmt.Sprintf("HMGet失败：%v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"key":    key,
			"values": valMap,
		})
	})
	// 5. 增量变更查询（适配新数据结构）
	c.serveMux.HandleFunc("/incrementalChanges", func(w http.ResponseWriter, r *http.Request) {
		changes := c.getIncrementalChanges()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(changes)
	})

	// 6. 获取全量数据（适配新结构）
	c.serveMux.HandleFunc("/getFullData", func(w http.ResponseWriter, r *http.Request) {
		fullData := c.getFullData()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fullData)
	})

}

// 适配：更新 getIncrementalChanges 支持 Hash 增量同步
func (c *Cluster) getIncrementalChanges() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	changes := make(map[string]interface{})
	now := time.Now()
	redisData := interface{}(c.dataStore).(RedisData)

	// String 类型增量
	for k := range redisData.String {
		if t, exists := c.recentChanges[k]; !exists || now.Sub(t) <= 10*time.Second {
			changes[k] = map[string]interface{}{
				"type": "string",
				"val":  redisData.String[k],
			}
		}
	}

	// Hash 类型增量（按 key:field 拆分）
	for key, fieldMap := range redisData.Hash {
		for field, val := range fieldMap {
			changeKey := fmt.Sprintf("hash:%s:%s", key, field)
			if t, exists := c.recentChanges[changeKey]; !exists || now.Sub(t) <= 10*time.Second {
				changes[changeKey] = map[string]interface{}{
					"type":  "hash",
					"key":   key,
					"field": field,
					"val":   val,
				}
			}
		}
	}

	return changes
}

// 适配：更新 getFullData 支持返回 String/Hash 全量数据
func (c *Cluster) getFullData() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	redisData := interface{}(c.dataStore).(RedisData)
	return map[string]interface{}{
		"string": redisData.String,
		"hash":   redisData.Hash,
	}
}

// startHTTPAPI 启动HTTP服务
func (c *Cluster) startHTTPAPI() {
	// 监听地址（与节点地址一致）
	//TODO: listenAddr := c.LocalNode.Addr
	//listenAddr := "1" + strings.Split(c.LocalNode.Addr, ":")[1]
	listenAddr := c.LocalNode.Addr
	// 创建HTTP服务器实例
	c.httpServer = &http.Server{
		Addr:    listenAddr,
		Handler: c.serveMux,
	}
	log.Printf("HTTP服务启动，监听：%s", listenAddr)
	// 启动HTTP服务
	err := c.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP服务启动失败：%v", err)
	}
}

// Set 写入数据（核心方法，路由到对应主节点）
func (c *Cluster) Set(w http.ResponseWriter, r *http.Request, key, val string) error {

	c.mu.Lock()
	defer c.mu.Unlock()
	//TODO: 计算槽位，路由到目标主节点
	slot := c.calcSlot(key)
	//masterID := c.slotMap[slot]
	//TODO:根据槽获取leaderID
	fmt.Println(111111111111111111)
	fmt.Printf("1111====== %v", slot)
	masterID := c.GetSlotToLeaderID(slot)
	// 从节点拒绝写操作
	if c.LocalNode.LeaderID != c.LocalNode.NodeID {
		//return fmt.Errorf("从节点不支持写操作（节点类型：%v \n", c.LocalNode.LeaderID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "Set", key, val)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}
	fmt.Printf("1111====== %v", masterID)
	log.Printf("Set操作：key=%s, val=%s, 计算槽位=%d, 目标主节点ID=%s, 本地节点ID=%s", key, val, slot, masterID, c.LocalNode.NodeID)
	//masterID2, _ := strconv.Atoi(masterID)
	// 目标主节点是自身，直接写入
	if masterID == c.LocalNode.NodeID {
		c.setStringData(key, val)
		c.recentChanges[key] = time.Now()
		log.Printf("本地写入数据：key=%s, val=%s（槽位：%d）", key, val, slot)
		// 同步到所有从节点
		go c.syncToReplicas(key, val)
		return nil
	}

	// 目标主节点是其他节点，路由转发
	log.Printf("目标主节点不是自身，从Nodes中查找目标主节点：masterID=%s", masterID)
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		log.Printf("目标主节点不存在：masterID=%d", masterID)
		//return fmt.Errorf("目标主节点1 %d 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "Set", key, val)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	} else {
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getTargetUrl(targetAddr, "Set", key, val)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}
	//TODO:暂时屏蔽，重启之后没有把状态改为上线
	//if targetNode.Status != Online {
	//	return "", fmt.Errorf("目标主节点 %d 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//_, err := c.routeToNode(addr, "Set", key, val)
	//return err
}

// Get 读取数据（核心方法，路由到对应节点）
func (c *Cluster) Get(w http.ResponseWriter, r *http.Request, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 计算槽位，找到主节点
	slot := c.calcSlot(key)
	//masterID := c.slotMap[slot]
	masterID := c.GetSlotToLeaderID(slot)

	// 检查本地是否有该数据（主节点/从节点缓存）
	if val, exists := c.getStringData(key); exists {
		log.Printf("本地读取数据：key=%s, val=%s（槽位：%d）", key, val, slot)
		return val, nil
	}
	//TODO:如果本身是主节点没数据就返回没有
	if masterID == c.LocalNode.NodeID {
		return "", nil
	}
	//masterID2, _ := strconv.Atoi(masterID)
	// 本地无数据，路由到主节点读取
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		//return "", fmt.Errorf("目标主节点 %d 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "Get", key, "")
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return "", nil
	} else {
		//shard中的主节点
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getTargetUrl(targetAddr, "Get", key, "")
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return "", nil
	}

	////TODO:暂时屏蔽，重启之后没有把状态改为上线
	//if targetNode.Status != Online {
	//	return "", fmt.Errorf("目标主节点 %d 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//log.Printf("路由读取数据到节点 %s：key=%s（槽位：%d）", addr, key, slot)
	//
	//val, err := c.routeToNode(addr, "Get", key, "")
	//if err != nil {
	//	return "", err
	//}
	//
	//// 解析响应（Get接口返回JSON格式）
	//var getResp map[string]string
	//if err := json.Unmarshal([]byte(val), &getResp); err != nil {
	//	return "", fmt.Errorf("响应解析失败：%v", err)
	//}
	//return getResp["val"], nil
}

// 新增：HSet 核心方法（路由+存储）
func (c *Cluster) HSet(w http.ResponseWriter, r *http.Request, key, field, val string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 哈希槽路由（按key计算槽位，与String共用路由逻辑）
	slot := c.calcSlot(key)
	log.Printf("HSet：计算槽位：key=%s, slot=%d", key, slot)
	//masterID := c.slotMap[slot]
	masterID := c.GetSlotToLeaderID(slot)
	if c.LocalNode.LeaderID != c.LocalNode.NodeID {
		//return fmt.Errorf("从节点不支持写操作")
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "HSet", key, val, "field", field)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}

	log.Printf("HSet：槽位映射：slot=%d, masterID=%s", slot, masterID)
	log.Printf("HSet：本地节点ID：%s", c.LocalNode.NodeID)
	//masterID2, _ := strconv.Atoi(masterID)
	if masterID == c.LocalNode.NodeID {
		// 本地存储
		log.Printf("HSet：本地存储：key=%s, field=%s, val=%s", key, field, val)
		c.setHashData(key, field, val)
		log.Printf("本地HSet：key=%s, field=%s, val=%s（槽位：%d）", key, field, val, slot)
		// 同步到从节点
		log.Printf("HSet：同步到从节点：key=%s, field=%s, val=%s", key, field, val)
		go c.syncToReplicas(fmt.Sprintf("hash:%s:%s", key, field), val)
		return nil
	}

	// 路由到其他主节点
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		log.Printf("HSet：目标主节点不可用：masterID=%s", masterID)
		//return fmt.Errorf("目标主节点 %s 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "HSet", key, val, "field", field)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	} else {
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getTargetUrl(targetAddr, "HSet", key, val, "field", field)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}
	//if targetNode.Status != Online {
	//	return fmt.Errorf("目标主节点 %d 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//log.Printf("路由HSet到节点 %s：key=%s（槽位：%d）", targetNode.Addr, key, slot)
	//// 构造Hash参数路由（扩展routeToNode支持多参数）
	//_, err := c.routeToNode(addr, "HSet", key, val, "field", field)
	//return err
}

// 新增：HGet 核心方法（路由+查询）
func (c *Cluster) HGet(w http.ResponseWriter, r *http.Request, key, field string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 哈希槽路由（按key计算槽位）
	slot := c.calcSlot(key)
	//masterID := c.slotMap[slot]
	masterID := c.GetSlotToLeaderID(slot)
	//masterID2, _ := strconv.Atoi(masterID)
	fmt.Printf("111=========%v \n", masterID)
	//masterID2, _ := strconv.Atoi(masterID)
	// 本地查询
	if val, exists := c.getHashData(key, field); exists {
		log.Printf("本地HGet：key=%s, field=%s, val=%s（槽位：%d）", key, field, val, slot)
		return val, nil
	}
	fmt.Printf("2222=========%v \n", masterID)
	//TODO:如果本身是主节点没数据就返回没有
	if masterID == c.LocalNode.NodeID {
		return "", nil
	}
	// 路由到主节点查询
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		//return "", fmt.Errorf("目标主节点 %s 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getTargetUrl(targetAddr, "HGet", key, "", "field", field)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return "", nil
	} else {
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getTargetUrl(targetAddr, "HGet", key, "", "field", field)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return "", nil
	}
	//if targetNode.Status != Online {
	//	return "", fmt.Errorf("目标主节点 %s 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//fmt.Printf("3333=========%v \n", masterID)
	//log.Printf("路由HGet到节点 %s：key=%s（槽位：%d）", addr, key, slot)
	//val, err := c.routeToNode(addr, "HGet", key, "", "field", field)
	//if err != nil {
	//	return "", err
	//}
	//
	//fmt.Printf("4444=========%v \n", masterID)
	//// 解析Hash响应
	//var hgetResp map[string]string
	//if err := json.Unmarshal([]byte(val), &hgetResp); err != nil {
	//	return "", fmt.Errorf("响应解析失败：%v", err)
	//}
	//return hgetResp["val"], nil
}

// 新增：HMSet 核心方法（路由+批量存储）
func (c *Cluster) HMSet(w http.ResponseWriter, r *http.Request, key string, fieldVals map[string]string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 批量操作路由：按key计算主节点（确保同一key的批量操作路由到同一节点）
	slot := c.calcSlot(key)
	//masterID := c.slotMap[slot]
	masterID := c.GetSlotToLeaderID(slot)
	if c.LocalNode.LeaderID != c.LocalNode.NodeID {
		//return fmt.Errorf("从节点不支持写操作")
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getHMSetTargetUrl(targetAddr, "HMSet", key, fieldVals)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}

	//masterID2, _ := strconv.Atoi(masterID)
	if masterID == c.LocalNode.NodeID {
		// 本地批量存储
		// 逐字段写入本地 Hash
		for field, val := range fieldVals {
			c.setHashData(key, field, val)
		}
		log.Printf("本地HMSet：key=%s，字段数：%d", key, len(fieldVals))
		// 批量同步到从节点
		for field, val := range fieldVals {
			go c.syncToReplicas(fmt.Sprintf("hash:%s:%s", key, field), val)
		}
		return nil
	}
	//masterID2, _ = strconv.Atoi(masterID)
	// 路由到其他主节点（构造批量参数）
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		//return fmt.Errorf("目标主节点 %s 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getHMSetTargetUrl(targetAddr, "HMSet", key, fieldVals)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	} else {
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getHMSetTargetUrl(targetAddr, "HMSet", key, fieldVals)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil
	}
	//if targetNode.Status != Online {
	//	return fmt.Errorf("目标主节点 %s 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//log.Printf("路由HMSet到节点 %s：key=%s", addr, key)
	//_, err := c.routeHashBatchToNode(addr, "HMSet", key, fieldVals)
	//return err
}

// 新增：HMGet 核心方法（路由+批量查询）
func (c *Cluster) HMGet(w http.ResponseWriter, r *http.Request, key string, fields []string) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 批量查询路由：按key计算主节点
	slot := c.calcSlot(key)
	//masterID := c.slotMap[slot]
	masterID := c.GetSlotToLeaderID(slot)

	// 本地批量查询
	localValMap := make(map[string]string)
	for _, field := range fields {
		if v, ok := c.getHashData(key, field); ok {
			localValMap[field] = v
		}
	}
	// 检查是否所有字段都在本地（集群环境下可能部分分片在其他节点？不，同一key的批量操作路由到同一节点，所有分片都在该节点）
	log.Printf("本地HMGet：key=%s，命中字段数：%d", key, len(localValMap))
	if len(localValMap) == len(fields) {
		return localValMap, nil
	}

	//TODO:如果本身是主节点没数据就返回没有
	if masterID == c.LocalNode.NodeID {
		return nil, nil
	}
	// 路由到主节点查询（补充未命中字段）
	//masterID2, _ := strconv.Atoi(masterID)
	targetNode, exists := c.ShardNodeInfo[masterID]
	if !exists {
		//return nil, fmt.Errorf("目标主节点 %d 不可用", masterID)
		//TODO:跳转307,到目标主节点
		targetAddr := c.GetNodeIdToNodeArr(masterID)
		targetUrl, _ := c.getHMGetTargetUrl(targetAddr, "HMGet", key, fields)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil, nil
	} else {
		//TODO:跳转307,到目标主节点
		targetAddr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
		targetUrl, _ := c.getHMGetTargetUrl(targetAddr, "HMGet", key, fields)
		fmt.Printf("111====%v \n", targetUrl)
		http.Redirect(w, r, targetUrl, http.StatusTemporaryRedirect)
		return nil, nil
	}
	//if targetNode.Status != Online {
	//	return nil, fmt.Errorf("目标主节点 %d 不可用", masterID)
	//}
	//addr := fmt.Sprintf("%s:%d", targetNode.IP, targetNode.Port)
	//log.Printf("路由HMGet到节点 %s：key=%s", addr, key)
	//remoteValMap, err := c.routeHashBatchGetToNode(addr, key, fields)
	//if err != nil {
	//	return nil, err
	//}
	//
	//return remoteValMap, nil
}

//// routeToNode 路由请求到目标节点
//func (c *Cluster) routeToNode(addr, cmd, key, val string, extra ...interface{}) (string, error) {
//	// 构建请求URL
//	url := fmt.Sprintf("http://%s/%s?key=%s&val=%s", addr, cmd, key, val)
//
//	println("================url:" + url)
//	// 添加额外参数（如槽位、目标地址）
//	for i := 0; i < len(extra); i += 2 {
//		if i+1 >= len(extra) {
//			break
//		}
//		paramKey := fmt.Sprintf("%v", extra[i])
//		paramVal := fmt.Sprintf("%v", extra[i+1])
//		url += fmt.Sprintf("&%s=%s", paramKey, paramVal)
//	}
//	fmt.Printf("3333333333333333333")
//	// 发送请求
//	resp, err := http.Get(url)
//	if err != nil {
//		return "", fmt.Errorf("请求节点 %s 失败：%v", addr, err)
//	}
//	defer resp.Body.Close()
//	fmt.Printf("4444444444444444444")
//	// 读取响应
//	var result map[string]string
//	err2 := json.NewDecoder(resp.Body).Decode(&result)
//	if err2 != nil {
//		// 尝试将响应解析为一个字符串
//		fmt.Printf("5555555555555555555")
//		var strResult string
//		err3 := json.NewDecoder(resp.Body).Decode(&strResult)
//		if err3 != nil {
//			return "", fmt.Errorf("解析节点 %s 响应失败：%v", addr, err)
//		}
//		fmt.Printf("66666666666666666")
//		return strResult, nil
//	}
//	fmt.Printf("77777777777777777")
//	// 如果是 HGet 命令，返回 val 字段
//	if cmd == "HGet" {
//		return result["val"], nil
//	}
//	fmt.Printf("888888888888888888")
//	return "", nil
//}

//// 新增：批量Hash写路由（适配多field参数）
//func (c *Cluster) routeHashBatchToNode(addr, cmd, key string, fieldVals map[string]string) (string, error) {
//	// 构建URL：key=xxx&field1=val1&field2=val2...
//	url := fmt.Sprintf("http://%s/%s?key=%s", addr, cmd, key)
//	for field, val := range fieldVals {
//		url += fmt.Sprintf("&%s=%s", field, val)
//	}
//
//	resp, err := http.Get(url)
//	if err != nil {
//		return "", fmt.Errorf("请求节点 %s 失败：%v", addr, err)
//	}
//	defer resp.Body.Close()
//
//	var result string
//	json.NewDecoder(resp.Body).Decode(&result)
//	return result, nil
//}
//
//// 新增：批量Hash读路由（适配多field参数）
//func (c *Cluster) routeHashBatchGetToNode(addr, key string, fields []string) (map[string]string, error) {
//	fieldsStr := strings.Join(fields, ",")
//	url := fmt.Sprintf("http://%s/%s/HMGet?key=%s&fields=%s", addr, addr, key, fieldsStr)
//
//	resp, err := http.Get(url)
//	if err != nil {
//		return nil, fmt.Errorf("请求节点 %s 失败：%v", addr, err)
//	}
//	defer resp.Body.Close()
//
//	var hmgetResp map[string]interface{}
//	json.NewDecoder(resp.Body).Decode(&hmgetResp)
//	valMap := hmgetResp["values"].(map[string]string)
//	return valMap, nil
//}

// TODO:通过参数生成URL:|get|set|Hget|Hset
func (c *Cluster) getTargetUrl(addr string, cmd string, key string, val string, extra ...interface{}) (string, error) {
	// 构建请求URL
	url := fmt.Sprintf("http://%s/%s?key=%s&val=%s", addr, cmd, key, val)
	println("================url:" + url)
	// 添加额外参数（如槽位、目标地址）
	for i := 0; i < len(extra); i += 2 {
		if i+1 >= len(extra) {
			break
		}
		paramKey := fmt.Sprintf("%v", extra[i])
		paramVal := fmt.Sprintf("%v", extra[i+1])
		url += fmt.Sprintf("&%s=%s", paramKey, paramVal)
	}
	// 发送请求
	return url, nil
}

// TODO:通过参数生成URL:HMset
func (c *Cluster) getHMSetTargetUrl(addr string, cmd string, key string, fieldVals map[string]string) (string, error) {
	// 构建请求URL
	url := fmt.Sprintf("http://%s/%s?key=%s", addr, cmd, key)
	println("================url:" + url)
	// 添加额外参数（如槽位、目标地址）
	for field, val := range fieldVals {
		url += fmt.Sprintf("&%s=%s", field, val)
	}
	// 发送请求
	return url, nil
}

// TODO:通过参数生成URL:HMget
func (c *Cluster) getHMGetTargetUrl(addr string, cmd string, key string, fields []string) (string, error) {
	// 构建请求URL
	str := strings.Join(fields, ",")
	url := fmt.Sprintf("http://%s/%s?key=%s&fields=%s", addr, cmd, key, str)
	println("================url:" + url)
	// 发送请求
	return url, nil
}
