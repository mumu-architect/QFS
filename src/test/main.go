package main

import (
	"common/logger"
	"encoding/json"
	"fmt"
	"github.com/cornelk/hashmap"
	"strconv"
	"sync"
	"time"
)

type P struct {
	nodeMap hashmap.Map[string, P1]
}
type P1 struct {
	name string
	age  int
}

var instance *P
var once sync.Once

func GetInstance() *P {
	once.Do(func() {
		instance = &P{}
	})
	return instance
}

type Person struct {
	Name string
	Age  int
}

func main() {
	// 创建一个Person对象
	person := Person{
		Name: "John",
		Age:  30,
	}

	// 将Person对象转换为JSON字符串
	jsonData, err := json.Marshal(person)
	if err != nil {
		fmt.Println("转换为JSON时发生错误:", err)
		return
	}

	// 打印JSON字符串
	fmt.Println(string(jsonData))

	// 获取当前时间
	now := time.Now()

	// 输出时间戳（秒）
	fmt.Println("时间戳（秒）:", now.Unix())
	now = time.Now()
	// 输出时间戳（纳秒）
	fmt.Println("时间戳（毫秒）:", now.UnixMilli())
	now = time.Now()
	// 输出时间戳（微秒）
	fmt.Println("时间戳（微秒）:", now.UnixMicro())
	now = time.Now()
	// 输出时间戳（纳秒）
	fmt.Println("时间戳（纳秒）:", now.UnixNano())
	now = time.Now()
	// 输出时间戳（纳秒）
	fmt.Println("时间戳（纳秒）:", now.UnixNano())
	logger.Trace.Println("Trace31231231")
	logger.Info.Println("Info31231231")
	logger.Warning.Printf("debug %s", "secret")
	logger.Error.Println("Error31231231")
	logger.Info.Printf("debug %s", "Info31231231")
	m := hashmap.New[string, int]()
	m.Set("amount", 123)
	m.Set("amount", 234)
	//a := GetInstance()

	p1 := P1{}
	var b *hashmap.Map[string, P1] = hashmap.New[string, P1]()
	b.Set("amount", p1)
	value, _ := b.Get("amount")

	logger.Info.Println(value)

	logger.Info.Println(m.Get("Aamount"))
	for i := 0; i < 10000; i++ {
		m.Set("A"+strconv.Itoa(i), i)
	}

	t := time.Now().UnixNano()
	logger.Info.Println(t)
	logger.Info.Println(m.Get("A5000"))
	e := time.Now().UnixNano()
	logger.Info.Println(e)
	tt := e - t
	logger.Info.Println(tt)
}
