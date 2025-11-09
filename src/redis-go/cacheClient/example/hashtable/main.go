package common

import (
	"fmt"
	"time"
)

func main() {
	client, err := NewClient("127.0.0.1:6379", "", 5*time.Second)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	hashKey := "user:1003"

	// 1. HSetNX（字段不存在时设置）
	ok, _ := client.HSetNX(hashKey, "name", "王五")
	fmt.Println("HSetNX name:", ok) // 第一次true，重复调用false

	// 2. HSetMulti（批量设置）
	_ = client.HSetMulti(hashKey, map[string]string{
		"age":  "28",
		"city": "上海",
		"job":  "工程师",
	})

	// 3. HMGet（批量获取）
	vals, _ := client.HMGet(hashKey, "name", "age", "gender")
	fmt.Println("HMGet result:", vals) // 输出: [王五 28 ""]

	// 4. HGetAll（获取所有字段和值）
	all, _ := client.HGetAll(hashKey)
	fmt.Println("HGetAll result:", all) // 输出: map[age:28 city:上海 job:工程师 name:王五]

	// 5. HKeys/HVals（字段名/值集合）
	keys, _ := client.HKeys(hashKey)
	valsAll, _ := client.HVals(hashKey)
	fmt.Println("HKeys:", keys)    // 输出: [name age city job]
	fmt.Println("HVals:", valsAll) // 输出: [王五 28 上海 工程师]

	// 6. HLen/HExists（长度/存在性判断）
	lenVal, _ := client.HLen(hashKey)
	exists, _ := client.HExists(hashKey, "city")
	fmt.Println("HLen:", lenVal)         // 输出: 4
	fmt.Println("HExists city:", exists) // 输出: true

	// 7. HDel（删除字段）
	delCount, _ := client.HDel(hashKey, "job", "gender")
	fmt.Println("HDel count:", delCount) // 输出: 1（仅job存在）
}
