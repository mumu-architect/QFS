package dragonboatRaft

import (
	"maps"
	"slices"
)

// MinMapKey 获取map[int]string最小key
func MinMapKey(m map[uint64]string) uint64 {
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	return keys[0]
}
