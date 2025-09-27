package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 定义了应用的所有配置项
type Config struct {
	Server struct {
		Address         string `yaml:"address"`          // 服务监听地址，如 ":7000"
		ClusterMode     bool   `yaml:"cluster_mode"`     // 是否启用集群模式
		NodeID          string `yaml:"node_id"`          // 节点唯一标识符
		ClusterPassword string `yaml:"cluster_password"` // 集群管理命令的密码
	} `yaml:"server"`

	Replication struct {
		SlaveOf string `yaml:"slave_of"` // 主节点地址，仅从节点需要配置
	} `yaml:"replication"`

	Cluster struct {
		NodeTimeout    time.Duration `yaml:"node_timeout"`    // 节点超时时间
		GossipInterval time.Duration `yaml:"gossip_interval"` // Gossip协议消息间隔
	} `yaml:"cluster"`

	Logging struct {
		Level string `yaml:"level"` // 日志级别: debug, info, warn, error
	} `yaml:"logging"`
}

// NewDefaultConfig 创建一个带有默认值的配置实例
func NewDefaultConfig() *Config {
	cfg := &Config{}

	// 设置默认值
	cfg.Server.Address = ":6379"
	cfg.Server.ClusterMode = false
	cfg.Server.NodeID = "" // 留空表示自动生成
	cfg.Server.ClusterPassword = ""

	cfg.Replication.SlaveOf = ""

	cfg.Cluster.NodeTimeout = 15 * time.Second
	cfg.Cluster.GossipInterval = 1 * time.Second

	cfg.Logging.Level = "info"

	return cfg
}

// LoadOrCreate 加载配置文件，如果文件不存在则创建默认配置文件
func LoadOrCreate(filePath string) (*Config, error) {
	// 1. 先创建默认配置作为基础
	defaultCfg := NewDefaultConfig()

	// 2. 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 文件不存在，创建默认配置文件
		fmt.Printf("配置文件 '%s' 不存在，正在创建默认配置...\n", filePath)

		// 确保父目录存在
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建配置目录失败: %w", err)
		}

		// 保存默认配置到文件
		if err := defaultCfg.Save(filePath); err != nil {
			return nil, fmt.Errorf("保存默认配置文件失败: %w", err)
		}

		fmt.Printf("默认配置文件已创建: %s\n", filePath)
		return defaultCfg, nil
	}

	// 3. 文件存在，加载并合并配置
	fileCfg, err := loadFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("加载配置文件失败: %w", err)
	}

	// 合并配置：文件配置覆盖默认配置
	mergeConfig(defaultCfg, fileCfg)

	fmt.Printf("配置文件加载成功: %s\n", filePath)
	return defaultCfg, nil
}

// 从文件加载配置
func loadFromFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}

// MarshalYAML 为AppConfig实现yaml.Marshaler接口（指针接收器）
func (c *Config) MarshalYAML() (interface{}, error) {
	// 根节点为映射类型（键值对）
	root := &yaml.Node{Kind: yaml.MappingNode}
	// 递归处理当前结构体
	if err := marshalNode(reflect.ValueOf(c).Elem(), root); err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}
	return root, nil
}

// marshalNode 递归处理任意值，转换为yaml.Node
func marshalNode(val reflect.Value, node *yaml.Node) error {
	// 处理指针：如果是指针，取其指向的值
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil // 空指针直接返回
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		// 1. 处理结构体：节点类型为映射（MappingNode）
		node.Kind = yaml.MappingNode
		typ := val.Type()

		// 遍历结构体字段
		for i := 0; i < val.NumField(); i++ {
			fieldVal := val.Field(i)
			fieldTyp := typ.Field(i)

			// 跳过未导出字段（首字母小写）
			if fieldTyp.PkgPath != "" {
				continue
			}

			// 1.1 处理键节点（从yaml标签或字段名获取）
			key := fieldTyp.Tag.Get("yaml")
			if key == "" {
				key = fieldTyp.Name
			}
			keyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}

			// 1.2 处理值节点（递归）
			valueNode := &yaml.Node{}
			if err := marshalNode(fieldVal, valueNode); err != nil {
				return fmt.Errorf("处理字段 %s 失败: %w", fieldTyp.Name, err)
			}

			// 添加键值对到当前节点
			node.Content = append(node.Content, keyNode, valueNode)
		}

	case reflect.Slice, reflect.Array:
		// 2. 处理切片/数组：节点类型为序列（SequenceNode）
		node.Kind = yaml.SequenceNode

		// 遍历切片元素，递归处理每个元素
		for i := 0; i < val.Len(); i++ {
			elemNode := &yaml.Node{}
			if err := marshalNode(val.Index(i), elemNode); err != nil {
				return fmt.Errorf("处理切片元素 %d 失败: %w", i, err)
			}
			node.Content = append(node.Content, elemNode)
		}

	case reflect.Map:
		// 3. 处理映射：节点类型为映射（MappingNode）
		node.Kind = yaml.MappingNode

		// 遍历映射的键值对
		for _, keyVal := range val.MapKeys() {
			// 3.1 处理映射的键
			keyNode := &yaml.Node{}
			if err := marshalNode(keyVal, keyNode); err != nil {
				return fmt.Errorf("处理映射键失败: %w", err)
			}

			// 3.2 处理映射的值（递归）
			valueNode := &yaml.Node{}
			if err := marshalNode(val.MapIndex(keyVal), valueNode); err != nil {
				return fmt.Errorf("处理映射值失败: %w", err)
			}

			node.Content = append(node.Content, keyNode, valueNode)
		}

	case reflect.String:
		// 4. 处理字符串：强制双引号
		node.Kind = yaml.ScalarNode
		node.Value = val.String()
		node.Style = yaml.DoubleQuotedStyle // 核心：字符串带双引号

	default:
		// 5. 其他类型（int、bool等）：使用默认序列化
		if err := node.Encode(val.Interface()); err != nil {
			return fmt.Errorf("序列化基本类型失败: %w", err)
		}
	}

	return nil
}

// Save 将配置保存到文件
func (c *Config) Save(filePath string) error {
	// 使用标准的yaml.Marshal方法
	// 关键：传入指针，确保Marshal能识别到Marshaler接口实现
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件，权限0644表示所有者可读写，其他用户只读
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// 合并两个配置，src中的非零值会覆盖dest中的值
func mergeConfig(dest, src *Config) {
	// 合并Server配置
	if src.Server.Address != "" {
		dest.Server.Address = src.Server.Address
	}
	if src.Server.ClusterMode {
		dest.Server.ClusterMode = src.Server.ClusterMode
	}
	if src.Server.NodeID != "" {
		dest.Server.NodeID = src.Server.NodeID
	}
	if src.Server.ClusterPassword != "" {
		dest.Server.ClusterPassword = src.Server.ClusterPassword
	}

	// 合并Replication配置
	if src.Replication.SlaveOf != "" {
		dest.Replication.SlaveOf = src.Replication.SlaveOf
	}

	// 合并Cluster配置
	if src.Cluster.NodeTimeout != 0 {
		dest.Cluster.NodeTimeout = src.Cluster.NodeTimeout
	}
	if src.Cluster.GossipInterval != 0 {
		dest.Cluster.GossipInterval = src.Cluster.GossipInterval
	}

	// 合并Logging配置
	if src.Logging.Level != "" {
		dest.Logging.Level = src.Logging.Level
	}
}
