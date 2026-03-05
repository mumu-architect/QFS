package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"mumu.com/config" // 导入新的 config 包

	"github.com/google/uuid" // 使用 uuid 库生成唯一 NodeID
	"gopkg.in/yaml.v3"
)

func main() {
	// 1. 设置命令行参数,定义配置文件路径
	configFile := flag.String("config", "./configs/my_config.yaml", "Path to a YAML configuration file. (e.g., -config ./configs/my_config.yaml)")
	//是否格式化打印配置
	printExampleConfig := flag.Bool("print-example-config", false, "Print an example YAML configuration to stdout.")

	// 为了向后兼容，保留部分老的命令行参数，但优先级低于配置文件
	centerId := flag.Int64("centerid", 31, "Center ID (overridden by config file if provided)")
	workerId := flag.Int64("workerid", 1023, "Worker ID (overridden by config file if provided)")
	addr := flag.String("addr", "", "Server address (overridden by config file if provided)")
	clusterMode := flag.Bool("cluster", false, "Enable cluster mode (overridden by config file if provided)")
	nodeID := flag.String("nodeid", "", "Node ID (overridden by config file if provided)")
	slaveOf := flag.String("slaveof", "", "Address of master node.go (overridden by config file if provided)")
	clusterAuth := flag.String("cluster-auth", "", "Password for cluster management commands (overridden by config file if provided)")

	flag.Parse()

	// 2. 加载配置
	var cfg *config.Config
	var err error
	cfg, err = config.LoadOrCreate(*configFile)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// 3. 处理特殊命令：打印示例配置
	if *printExampleConfig {
		exampleCfg := cfg
		yamlData, err := yaml.Marshal(exampleCfg)
		if err != nil {
			log.Fatalf("Failed to generate example config: %v", err)
		}
		fmt.Println(string(yamlData))
		//退出
		os.Exit(0)
	}

	//4.如果命令行有值，直接覆盖
	if *centerId > 0 {
		cfg.Server.CenterId = *centerId
	}
	if *workerId > 0 {
		cfg.Server.WorkerId = *workerId
	}
	if *addr != "" {
		cfg.Server.Address = *addr
	}
	if *clusterMode {
		cfg.Server.ClusterMode = *clusterMode
	}
	if *nodeID != "" {
		cfg.Server.NodeID = *nodeID
	}
	if *slaveOf != "" {
		cfg.Replication.SlaveOf = *slaveOf
	}
	if *clusterAuth != "" {
		cfg.Server.ClusterPassword = *clusterAuth
	}

	// 5. 应用配置
	// 如果NodeID未在配置中设置，则自动生成一个
	if cfg.Server.NodeID == "" {
		cfg.Server.NodeID = uuid.New().String()[:8] // 使用UUID的前8位作为ID
	}
	fmt.Printf("Starting Redis-Go node.go '%s' on %s...\n", cfg.Server.NodeID, cfg.Server.Address)
	if cfg.Server.ClusterMode {
		fmt.Println("Cluster mode is ENABLED.")
	} else {
		fmt.Println("Cluster mode is DISABLED.")
	}

}
