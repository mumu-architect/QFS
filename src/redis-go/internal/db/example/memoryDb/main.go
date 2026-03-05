package main

import (
	"fmt"

	"mumu.com/redis-go/internal/db"
)

func main() {
	mdb := db.NewMemoryDB()

	// 1. SetNX/SetXX
	fmt.Println("SetNX key1 123:", mdb.SetNX("key1", "123")) // true
	fmt.Println("SetNX key1 456:", mdb.SetNX("key1", "456")) // false（已存在）
	fmt.Println("SetXX key1 789:", mdb.SetXX("key1", "789")) // true（已存在）
	fmt.Println("SetXX key2 000:", mdb.SetXX("key2", "000")) // false（不存在）

	// 2. MSet/MSetNX
	kv := map[string]string{"a": "1", "b": "2", "c": "3"}
	mdb.MSet(kv)
	fmt.Println("MGet a b c:", mdb.MGet("a", "b", "c")) // [1 2 3]

	kv2 := map[string]string{"a": "10", "d": "4"}
	fmt.Println("MSetNX kv2:", mdb.MSetNX(kv2)) // false（a 已存在）
	kv3 := map[string]string{"d": "4", "e": "5"}
	fmt.Println("MSetNX kv3:", mdb.MSetNX(kv3)) // true（d、e 均不存在）

	// 3. GetSet
	oldVal, exists := mdb.GetSet("a", "100")
	fmt.Println("GetSet a 100: oldVal=", oldVal, "exists=", exists) // oldVal=1, exists=true
	oldVal2, exists2 := mdb.GetSet("f", "6")
	fmt.Println("GetSet f 6: oldVal=", oldVal2, "exists=", exists2) // oldVal=, exists=false（键不存在）

	// 4. GetRange/StrLen
	mdb.Set("name", "张三abc123")
	fmt.Println("StrLen name:", mdb.StrLen("name"))                   // 8（2个中文+3个字母+3个数字）
	fmt.Println("GetRange name 0 2:", mdb.GetRange("name", 0, 2))     // 张三a（索引0-2，包含结束索引）
	fmt.Println("GetRange name -3 -1:", mdb.GetRange("name", -3, -1)) // 123（倒数3个字符）

	// 5. 批量操作结果
	fmt.Println("MGet d e f:", mdb.MGet("d", "e", "f")) // [4 5 6]
	fmt.Println("KeyCount:", mdb.KeyCount())            // 7（key1、a、b、c、d、e、f、name？实际是 8 个，根据操作调整）
}
