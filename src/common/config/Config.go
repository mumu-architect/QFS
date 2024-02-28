package config

import (
	"gopkg.in/yaml.v3"
	"mumu.com/common/logger"
	"os"
	"strings"
)

// Config 配置类
// Config is a struct that contains a slice of MonitorHostAddr
type Config struct {
	MonitorHostAddr []MonitorHostAddr
}

// MonitorHostAddr is a struct that contains an IP string and a Port int
type MonitorHostAddr struct {
	Ip   string
	Port int
}

// GetConfig 获取配置文件
// This function reads a file from a given URL and unmarshals the YAML content into a map
func GetConfig(fileUrl string) (map[string]any, error) {
	// Declare variables
	var err error
	// Read the file from the given URL
	dataBytes, err := os.ReadFile(fileUrl)
	if err != nil {
		// Log an error if the file cannot be read
		logger.Error.Printf("读取文件失败：%s", err)
		return nil, err
	}
	// Create a new map
	mp := make(map[string]any)
	// Unmarshal the YAML content into the map
	err = yaml.Unmarshal(dataBytes, mp)
	if err != nil {
		// Log an error if the YAML cannot be unmarshalled
		logger.Error.Printf("解析 yaml 文件失败：%s", err)
		return nil, err
	}
	// Return the map and any errors
	return mp, err
}

// GetMonitorHostAddr 获取监控主机地址
// GetMonitorHostAddr takes a fileUrl as an argument and returns a list of strings and an error
func GetMonitorHostAddr(fileUrl string) ([]string, error) {
	// GetConfig takes a fileUrl as an argument and returns a map of strings to any type and an error
	hostMap, err := GetConfig(fileUrl)
	// Initialize an empty list of strings
	var hostList []string
	// Get the map of strings to any type associated with the key "monitor"
	monitorMap, err1 := hostMap["monitor"].(map[string]any)
	if err != nil {
		// Log an error if the file could not be read
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	// Get the string associated with the key "HostAddr"
	monitorHostAddr, err2 := monitorMap["HostAddr"].(string)
	if err != nil {
		// Log an error if the key "HostAddr" does not exist
		logger.Error.Printf("键HostAddr不存在：%s", err2)
		return hostList, err
	}
	// Split the string by commas and add to the list of strings
	//fmt.Printf("%+v", mp)
	hostList = strings.Split(monitorHostAddr, ",")
	// Return the list of strings and no error
	return hostList, nil
}

// ParamConfig 定义一个接口
type ParamConfig interface {
	// GetConfigParam returns a string and an error
	GetConfigParam() (string, error)
}
type OneConfigParam struct {
	// fileUrl is a string field
	fileUrl string
	// oneKey is a string field
	oneKey string
}

// GetConfigParam takes two strings as parameters and returns a string and an error
func (c OneConfigParam) GetConfigParam(fileUrl string, oneKey string) (string, error) {
	// GetConfig takes a string as a parameter and returns a map and an error
	hostMap, err := GetConfig(fileUrl)
	// hostList is a string variable
	var hostList string
	// hostMap is a map variable
	paramValue, err1 := hostMap[oneKey].(string)
	if err != nil {
		logger.Error.Printf("读取文件失败：%s", err1)
		return hostList, err
	}
	//fmt.Printf("%+v", mp)
	// paramValue is a string variable
	hostList = paramValue
	return hostList, nil
}

// TwoConfigParam is a struct that contains fileUrl, oneKey, and twoKey strings
type TwoConfigParam struct {
	fileUrl string
	oneKey  string
	twoKey  string
}

// GetConfigParam returns a string and an error based on the given fileUrl, oneKey, and twoKey parameters
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

// ThreeConfigParam is a struct that contains fileUrl, oneKey, twoKey, and threeKey strings
type ThreeConfigParam struct {
	fileUrl  string
	oneKey   string
	twoKey   string
	threeKey string
}

// GetConfigParam returns a string and an error based on the given fileUrl, oneKey, twoKey, and threeKey parameters
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
