package config

import (
	"common/logger"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

// Config 配置类
type Config struct {
	MonitorHostAddr []MonitorHostAddr
}

type MonitorHostAddr struct {
	Ip   string
	Port int
}

// GetConfig 获取配置文件
func GetConfig(fileUrl string) (map[string]any, error) {
	var err error
	dataBytes, err := os.ReadFile(fileUrl)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err)
		return nil, err
	}
	mp := make(map[string]any)
	err = yaml.Unmarshal(dataBytes, mp)
	if err != nil {
		logger.Error.Printf("解析 yaml 文件失败：%s", err)
		return nil, err
	}
	return mp, err
}

// GetMonitorHostAddr 获取监控主机地址
func GetMonitorHostAddr(fileUrl string) ([]string, error) {
	hostMap, err := GetConfig(fileUrl)
	var hostList []string
	monitorMap, err1 := hostMap["monitor"].(map[string]any)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	monitorHostAddr, err2 := monitorMap["monitorHostAddr"].(string)
	if err != nil {
		// 键"monitorHostAddr"不存在
		logger.Error.Printf("键monitorHostAddr不存在：%s", err2)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	hostList = strings.Split(monitorHostAddr, ",")
	return hostList, nil
}
