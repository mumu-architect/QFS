package common

import (
	"errors"
	"strconv"
	"time"
)

// -------------------------- 完善的 Set 系列命令 --------------------------
// Set 基础设置（支持过期时间、NX/XX选项）
// option: "NX"（键不存在时设置）、"XX"（键存在时设置）、""（无限制）
func (c *Client) Set(key, value string, expire time.Duration, option string) error {
	args := []string{key, value}
	if expire > 0 {
		args = append(args, "EX", strconv.FormatInt(int64(expire.Seconds()), 10))
	}
	if option == "NX" || option == "XX" {
		args = append(args, option)
	}

	_, err := c.Do("SET", args...)
	return err
}

// SetNX 键不存在时设置（原子操作，分布式锁常用）
func (c *Client) SetNX(key, value string, expire time.Duration) (bool, error) {
	args := []string{key, value}
	if expire > 0 {
		args = append(args, "EX", strconv.FormatInt(int64(expire.Seconds()), 10))
	}

	res, err := c.Do("SET", append(args, "NX")...)
	if err != nil {
		return false, err
	}

	// 成功返回"OK"，失败返回nil
	return res == "OK", nil
}

// SetXX 键存在时更新（仅更新已有键）
func (c *Client) SetXX(key, value string, expire time.Duration) (bool, error) {
	args := []string{key, value}
	if expire > 0 {
		args = append(args, "EX", strconv.FormatInt(int64(expire.Seconds()), 10))
	}

	res, err := c.Do("SET", append(args, "XX")...)
	if err != nil {
		return false, err
	}

	return res == "OK", nil
}

// MSet 批量设置多个键值对（原子操作）
func (c *Client) MSet(kv map[string]string) error {
	args := make([]string, 0, len(kv)*2)
	for k, v := range kv {
		args = append(args, k, v)
	}

	_, err := c.Do("MSET", args...)
	return err
}

// MSetNX 批量设置（仅当所有键都不存在时生效，原子操作）
func (c *Client) MSetNX(kv map[string]string) (bool, error) {
	args := make([]string, 0, len(kv)*2)
	for k, v := range kv {
		args = append(args, k, v)
	}

	res, err := c.Do("MSETNX", args...)
	if err != nil {
		return false, err
	}

	// 返回1成功，0失败
	val, ok := res.(int64)
	if !ok {
		return false, errors.New("响应格式错误")
	}
	return val == 1, nil
}

// GetSet 设置新值并返回旧值（原子替换）
func (c *Client) GetSet(key, value string) (string, error) {
	res, err := c.Do("GETSET", key, value)
	if err != nil {
		return "", err
	}

	val, ok := res.(string)
	if !ok {
		return "", errors.New("响应格式错误")
	}
	return val, nil
}

// -------------------------- 完善的 Get 系列命令 --------------------------
// Get 基础获取（键不存在返回空字符串，无错误）
func (c *Client) Get(key string) (string, error) {
	res, err := c.Do("GET", key)
	if err != nil {
		return "", err
	}

	val, ok := res.(string)
	if !ok {
		return "", errors.New("响应格式错误")
	}
	return val, nil
}

// MGet 批量获取多个键值对（返回顺序与传入键一致，不存在的键返回空字符串）
func (c *Client) MGet(keys ...string) ([]string, error) {
	res, err := c.Do("MGET", keys...)
	if err != nil {
		return nil, err
	}

	// 解析数组响应（补充数组解析逻辑）
	arrRes, ok := res.([]interface{})
	if !ok {
		return nil, errors.New("响应格式错误")
	}

	result := make([]string, len(arrRes))
	for i, item := range arrRes {
		val, _ := item.(string)
		result[i] = val
	}
	return result, nil
}

// GetRange 获取字符串指定区间子串（start:起始索引，end:结束索引，负数表示倒数）
func (c *Client) GetRange(key string, start, end int) (string, error) {
	res, err := c.Do("GETRANGE", key, strconv.Itoa(start), strconv.Itoa(end))
	if err != nil {
		return "", err
	}

	val, ok := res.(string)
	if !ok {
		return "", errors.New("响应格式错误")
	}
	return val, nil
}

// StrLen 获取字符串值的长度
func (c *Client) StrLen(key string) (int64, error) {
	res, err := c.Do("STRLEN", key)
	if err != nil {
		return 0, err
	}

	val, ok := res.(int64)
	if !ok {
		return 0, errors.New("响应格式错误")
	}
	return val, nil
}
