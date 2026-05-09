package main

import (
	"fmt"
	"strconv"
	"sync"

	"mumu.com/redis-go/distributedId/snowflake"
)

// -------------------------- 5. 测试代码 --------------------------
func main() {
	sfg := snowflake.NewSnowFlakeGenerate()

	// 多协程测试：验证无锁安全与ID唯一性
	var wg sync.WaitGroup
	idSet := make(map[int64]struct{}) // 存储生成的ID，用于检测重复
	var mu sync.Mutex                 // 保护map的并发读写（仅测试用）

	// 启动10个协程，每个协程生成200个ID
	for worker := 0; worker < 10; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				id, err := sfg.Snowflake.NextID()
				if err != nil {
					fmt.Printf("协程%d生成ID失败：%v\n", workerID, err)
					return
				}
				// 检查ID是否重复
				mu.Lock()
				if _, exists := idSet[id]; exists {
					fmt.Printf("警告：协程%d生成重复ID：%d\n", workerID, id)
				}
				idSet[id] = struct{}{}
				mu.Unlock()
				// 可选：打印ID（高并发时可注释，避免输出混乱）
				// fmt.Printf("协程%d生成ID：%d\n", workerID, id)
			}
		}(worker)
	}

	wg.Wait()

	id := sfg.GetFlowID()

	fmt.Printf("生成ID：%v\n", id)
	id = sfg.GetFlowID()

	fmt.Printf("生成ID：%v\n", strconv.FormatInt(id, 10))
	fmt.Printf("生成ID：%v\n", id)

	fmt.Printf("测试完成：共生成%d个唯一ID（预期2000个）\n", len(idSet))
}
