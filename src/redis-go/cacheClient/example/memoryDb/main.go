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

	// 1. SetNX（键不存在时设置）
	ok, _ := client.SetNX("lock:order", "123", 10*time.Second)
	fmt.Println("SetNX lock:order:", ok) // 第一次true，重复调用false

	// 2. MSet/MGet（批量操作）
	_ = client.MSet(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	})
	vals, _ := client.MGet("a", "b", "c", "d")
	fmt.Println("MGet result:", vals) // 输出: [1 2 3 ""]

	// 3. GetSet（替换并返回旧值）
	oldVal, _ := client.GetSet("a", "100")
	fmt.Println("GetSet oldVal:", oldVal) // 输出: 1
	newVal, _ := client.Get("a")
	fmt.Println("Get newVal:", newVal) // 输出: 100

	// 4. GetRange（截取子串）
	_ = client.Set("phone", "13800138000", 0, "")
	subStr, _ := client.GetRange("phone", 0, 2)
	fmt.Println("GetRange phone:", subStr) // 输出: 138

	// 5. StrLen（获取长度）
	lenVal, _ := client.StrLen("phone")
	fmt.Println("StrLen phone:", lenVal) // 输出: 11
}
