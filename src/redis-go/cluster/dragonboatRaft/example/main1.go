package main

import (
	"fmt"

	"mumu.com/redis-go/cluster/dragonboatRaft"
)

func main2() {
	aa := dragonboatRaft.CalcSlot("aaa:11:22")
	fmt.Printf("key====%v \n", aa)
}
