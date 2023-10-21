package function

import (
	"fmt"
	"net"
	"strings"
)

// GetNativeIP 获取本机Ip
func GetNativeIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:8")
	if err != nil {
		fmt.Println("无法获取主机IP地址")
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	hostArr := strings.Split(localAddr.String(), ":")
	return hostArr[0], nil
}
