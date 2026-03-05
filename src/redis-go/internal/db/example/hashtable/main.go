package main

import (
	"fmt"

	"mumu.com/redis-go/internal/db"
)

func main1() {
	// 创建哈希表（初始容量16，负载因子0.75）
	ht := db.NewHashTable(16, 0.75)
	key := "user:1001"

	// 1. 批量设置字段
	ht.HSetMulti(key, map[string]interface{}{
		"name": "张三",
		"age":  25,
		"city": "北京",
	})

	// 2. HSetNX（字段不存在时设置）
	ok := ht.HSetNX(key, "age", 30)
	fmt.Println("HSetNX age:", ok) // 输出 false（age已存在）
	ok = ht.HSetNX(key, "gender", "male")
	fmt.Println("HSetNX gender:", ok) // 输出 true

	// 3. HIncrBy（数值自增）
	age, _ := ht.HIncrBy(key, "age", 1)
	fmt.Println("HIncrBy age:", age) // 输出 26

	// 4. 批量获取字段
	vals := ht.HMGet(key, "name", "age", "city", "email")
	fmt.Println("HMGet result:", vals) // 输出 [张三 26 北京 <nil>]

	// 5. 获取字段名和值集合
	keys := ht.HKeys(key)
	valsAll := ht.HVals(key)
	fmt.Println("HKeys:", keys)    // 输出 [name age city gender]
	fmt.Println("HVals:", valsAll) // 输出 [张三 26 北京 male]

	// 6. 清空哈希表
	clearCount := ht.HClear(key)
	fmt.Println("HClear count:", clearCount)       // 输出 4
	fmt.Println("HLen after clear:", ht.HLen(key)) // 输出 0
}

func main() {
	// 创建哈希表
	ht := db.NewHashTable(16, 0.75)
	hashKey := "user:1001"

	// 1. 批量设置字段
	ht.HSetMulti(hashKey, map[string]interface{}{
		"name": "张三",
		"age":  25,
		"city": "北京",
		"job":  "工程师",
	})

	// 2. 单字段删除
	delOk := ht.HDel(hashKey, "job")
	fmt.Println("HDel job:", delOk)                   // 输出 true
	fmt.Println("HLen after HDel:", ht.HLen(hashKey)) // 输出 3

	// 3. 批量删除字段
	delCount := ht.HDelMulti(hashKey, "age", "city", "gender")
	fmt.Println("HDelMulti count:", delCount)              // 输出 2（gender不存在）
	fmt.Println("HLen after HDelMulti:", ht.HLen(hashKey)) // 输出 1

	// 4. 检查哈希表是否存在
	exists := ht.HashExists(hashKey)
	fmt.Println("HashExists:", exists) // 输出 true

	// 5. 删除整个哈希表
	delHashOk := ht.HDelHash(hashKey)
	fmt.Println("HDelHash:", delHashOk)                               // 输出 true
	fmt.Println("HashExists after HDelHash:", ht.HashExists(hashKey)) // 输出 false

	// 6. 获取哈希表统计信息
	stats := ht.Stats()
	fmt.Println("HashTable Stats:", stats)
	// 输出：map[capacity:16 hash_count:0 load_factor:0 size:0]
}
