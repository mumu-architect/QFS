package config

import (
	"gopkg.in/yaml.v3"
	"mumu.com/common/logger"
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
	monitorHostAddr, err2 := monitorMap["HostAddr"].(string)
	if err != nil {
		// 键"monitorHostAddr"不存在
		logger.Error.Printf("键HostAddr不存在：%s", err2)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	hostList = strings.Split(monitorHostAddr, ",")
	return hostList, nil
}

// ParamConfig 定义一个接口
type ParamConfig interface {
	GetConfigParam() (string, error)
}
type OneConfigParam struct {
	fileUrl string
	oneKey  string
}

func (c OneConfigParam) GetConfigParam(fileUrl string, oneKey string) (string, error) {
	hostMap, err := GetConfig(fileUrl)
	var hostList string
	paramValue, err1 := hostMap[oneKey].(string)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	hostList = paramValue
	return hostList, nil
}

type TwoConfigParam struct {
	fileUrl string
	oneKey  string
	twoKey  string
}

func (c TwoConfigParam) GetConfigParam(fileUrl string, oneKey string, twoKey string) (string, error) {
	hostMap, err := GetConfig(fileUrl)
	var hostList string

	monitorMap, err1 := hostMap[oneKey].(map[string]any)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	paramValue, err2 := monitorMap[twoKey].(string)
	if err != nil {
		// 键"monitorHostAddr"不存在
		logger.Error.Printf("键%s不存在：%s", twoKey, err2)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	hostList = paramValue
	return hostList, nil
}

type ThreeConfigParam struct {
	fileUrl  string
	oneKey   string
	twoKey   string
	threeKey string
}

func (c ThreeConfigParam) GetConfigParam(fileUrl string, oneKey string, twoKey string, threeKey string) (string, error) {
	hostMap, err := GetConfig(fileUrl)
	var hostList string

	monitorMap, err1 := hostMap[oneKey].(map[string]any)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	dataMap, err2 := monitorMap[twoKey].(map[string]any)
	if err != nil {
		// 键"monitorHostAddr"不存在
		logger.Error.Printf("键%s不存在：%s", twoKey, err2)
		return hostList, err
	}
	paramValue, err2 := dataMap[threeKey].(string)
	if err != nil {
		// 键"monitorHostAddr"不存在
		logger.Error.Printf("键%s不存在：%s", threeKey, err2)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	hostList = paramValue
	return hostList, nil
}
