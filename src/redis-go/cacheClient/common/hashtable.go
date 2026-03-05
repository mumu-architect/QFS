package common

import (
	"errors"
)

// -------------------------- 完善的 HSet 系列命令 --------------------------
// HSet 单字段设置（已存在，保持兼容）
func (c *Client) HSet(hashKey, field, value string) error {
	_, err := c.Do("HSET", hashKey, field, value)
	return err
}

// HSetMulti 批量设置多字段（已存在，保持兼容）
func (c *Client) HSetMulti(hashKey string, fieldValues map[string]string) error {
	args := []string{hashKey}
	for field, val := range fieldValues {
		args = append(args, field, val)
	}
	_, err := c.Do("HSETMULTI", args...)
	return err
}

// HSetNX 字段不存在时设置（原子操作，避免覆盖）
func (c *Client) HSetNX(hashKey, field, value string) (bool, error) {
	res, err := c.Do("HSETNX", hashKey, field, value)
	if err != nil {
		return false, err
	}

	val, ok := res.(int64)
	if !ok {
		return false, errors.New("响应格式错误")
	}
	return val == 1, nil // 1=设置成功，0=字段已存在
}

// HDel 批量删除哈希字段
func (c *Client) HDel(hashKey string, fields ...string) (int64, error) {
	args := append([]string{hashKey}, fields...)

	res, err := c.Do("HDEL", args...)
	if err != nil {
		return 0, err
	}

	val, ok := res.(int64)
	if !ok {
		return 0, errors.New("响应格式错误")
	}
	return val, nil // 返回删除的字段数
}

// -------------------------- 完善的 HGet 系列命令 --------------------------
// HGet 单字段获取（已存在，保持兼容）
func (c *Client) HGet(hashKey, field string) (string, error) {
	res, err := c.Do("HGET", hashKey, field)
	if err != nil {
		return "", err
	}

	val, ok := res.(string)
	if !ok {
		return "", errors.New("响应格式错误")
	}
	return val, nil
}

// HMGet 批量获取多字段（返回顺序与传入字段一致，不存在的字段返回空字符串）
func (c *Client) HMGet(hashKey string, fields ...string) ([]string, error) {
	args := append([]string{hashKey}, fields...)
	res, err := c.Do("HMGET", args...)
	if err != nil {
		return nil, err
	}

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

// HGetAll 获取哈希所有字段和值（返回 map[字段]值）
func (c *Client) HGetAll(hashKey string) (map[string]string, error) {
	res, err := c.Do("HGETALL", hashKey)
	if err != nil {
		return nil, err
	}

	arrRes, ok := res.([]interface{})
	if !ok {
		return nil, errors.New("响应格式错误")
	}

	result := make(map[string]string)
	for i := 0; i < len(arrRes); i += 2 {
		if i+1 >= len(arrRes) {
			break // 格式异常时容错
		}
		field, _ := arrRes[i].(string)
		value, _ := arrRes[i+1].(string)
		result[field] = value
	}
	return result, nil
}

// HKeys 获取哈希所有字段名
func (c *Client) HKeys(hashKey string) ([]string, error) {
	res, err := c.Do("HKEYS", hashKey)
	if err != nil {
		return nil, err
	}

	arrRes, ok := res.([]interface{})
	if !ok {
		return nil, errors.New("响应格式错误")
	}

	result := make([]string, len(arrRes))
	for i, item := range arrRes {
		result[i], _ = item.(string)
	}
	return result, nil
}

// HVals 获取哈希所有字段值
func (c *Client) HVals(hashKey string) ([]string, error) {
	res, err := c.Do("HVALS", hashKey)
	if err != nil {
		return nil, err
	}

	arrRes, ok := res.([]interface{})
	if !ok {
		return nil, errors.New("响应格式错误")
	}

	result := make([]string, len(arrRes))
	for i, item := range arrRes {
		result[i], _ = item.(string)
	}
	return result, nil
}

// HLen 获取哈希字段总数
func (c *Client) HLen(hashKey string) (int64, error) {
	res, err := c.Do("HLEN", hashKey)
	if err != nil {
		return 0, err
	}

	val, ok := res.(int64)
	if !ok {
		return 0, errors.New("响应格式错误")
	}
	return val, nil
}

// HExists 判断哈希字段是否存在
func (c *Client) HExists(hashKey, field string) (bool, error) {
	res, err := c.Do("HEXISTS", hashKey, field)
	if err != nil {
		return false, err
	}

	val, ok := res.(int64)
	if !ok {
		return false, errors.New("响应格式错误")
	}
	return val == 1, nil // 1=存在，0=不存在
}
