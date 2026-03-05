package main

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Config 配置结构体
type Config struct {
	ServerHost string `yaml:"server_host"`
	ServerPort int    `yaml:"server_port"`
	RedisAddr  string `yaml:"redis_addr"`
	LogLevel   string `yaml:"log_level"`
}

// 关键：使用指针接收器实现yaml.Marshaler接口
// 方法签名必须严格匹配：func (*T) MarshalYAML() (interface{}, error)
func (c *Config) MarshalYAML() (interface{}, error) {
	root := yaml.Node{Kind: yaml.MappingNode}
	val := reflect.ValueOf(*c)
	typ := reflect.TypeOf(*c)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 获取yaml标签
		key := fieldType.Tag.Get("yaml")
		if key == "" {
			key = fieldType.Name
		}

		// 构建键节点
		keyNode := yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}

		// 构建值节点
		valueNode := yaml.Node{}
		if err := valueNode.Encode(field.Interface()); err != nil {
			return nil, err
		}

		// 字符串类型强制双引号
		if fieldType.Type.Kind() == reflect.String {
			valueNode.Style = yaml.DoubleQuotedStyle
		}

		root.Content = append(root.Content, &keyNode, &valueNode)
	}

	return &root, nil
}

// Save 保存配置到文件
func (c *Config) Save(filePath string) error {
	// 关键：传入指针，确保Marshal能识别到Marshaler接口实现
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func main() {
	c := &Config{
		ServerHost: "127.0.0.1",
		ServerPort: 123,
		RedisAddr:  "4444444",
		LogLevel:   "debug",
	}
	c.Save("configs/1.yaml")
}
