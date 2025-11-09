package snowflake

import (
	"fmt"
	"sync"
	"time"
)

// -------------------------- 5. 测试代码 --------------------------
func main() {
	// 自定义起始时间：2024-01-01 00:00:00（毫秒时间戳）
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	// 初始化雪花实例（数据中心ID=1，机器ID=3）
	sf, err := NewSnowflake(1, 1, epoch)
	if err != nil {
		fmt.Printf("初始化失败：%v\n", err)
		return
	}

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
				id, err := sf.NextID()
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
	fmt.Printf("测试完成：共生成%d个唯一ID（预期2000个）\n", len(idSet))
}
