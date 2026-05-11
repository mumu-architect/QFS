package fileManager

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type FileSyncClient struct {
	LeaderURL  string // 缓存的旧Leader地址
	RemoteFile string
	LocalFile  string
	retryTimes int // 已重试次数
	maxRetry   int // 最大重试阈值 3次
}

// GetFileMD5 计算一个文件的MD5值
func GetFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	// 转为16进制字符串
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// DoRemoteHttpFileSync TODO：实时同步远程http文件
func (nf *NodeFile) DoRemoteHttpFileSync(task *SyncTask) {
	fsc := &FileSyncClient{
		LeaderURL:  task.LeaderURL,
		RemoteFile: task.SourceURL,
		LocalFile:  task.LocalPath,
	}
	nf.StartSync(fsc)
	return
}

// 获取本地文件当前偏移量
func (nf *NodeFile) getLocalOffset(fsc *FileSyncClient) int64 {
	info, err := os.Stat(fsc.LocalFile)
	if err != nil {
		return 0
	}
	return info.Size()
}

// 在这里模拟：调用你Dragonboat接口获取最新Leader地址
func (nf *NodeFile) GetCurrentLeader() (string, int) {
	// 这里替换成你实际读取Dragonboat暴露的Leader函数
	leaderIp := nf.ShardNodeInfo[nf.LeaderId].IP
	leaderPort := nf.ShardNodeInfo[nf.LeaderId].Port
	return leaderIp, leaderPort
}

func (nf *NodeFile) StartSync(c *FileSyncClient) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 20 * time.Second,
		}).DialContext,
		DisableKeepAlives:     false,
		ResponseHeaderTimeout: 8 * time.Second,
		IdleConnTimeout:       15 * time.Second,
		MaxIdleConns:          100,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   0,
	}
	// 设置最大重试3次
	c.maxRetry = 3

	for {
		offset := nf.getLocalOffset(c)
		// 先用当前缓存的MasterURL请求
		reqURL := fmt.Sprintf("%s/SlavePullFile?file=%s&offset=%d", c.LeaderURL, c.RemoteFile, offset)

		log.Printf("使用旧地址同步,LeaderURL:%s, file=%s ,retry:%d, offset:%d", c.LeaderURL, c.RemoteFile, c.retryTimes, offset)

		resp, err := client.Get(reqURL)
		if err != nil {
			// 请求失败，重试计数+1
			c.retryTimes++
			log.Printf("连接旧主失败，当前重试次数: %d", c.retryTimes)

			// 判断是否达到最大重试次数
			if c.retryTimes >= c.maxRetry {
				// 超过3次 -> 强制刷新为Dragonboat最新Leader
				leaderIp, leaderPort := nf.GetCurrentLeader()
				c.LeaderURL = fmt.Sprintf("http://%s:%d", leaderIp, leaderPort)
				c.retryTimes = 0 // 清空重试计数器
				log.Println("重试超限，已切换为Dragonboat新Leader地址")
			}

			time.Sleep(1 * time.Second)
			continue
		}

		// 成功连上，重置重试计数器
		c.retryTimes = 0
		//TODO:创建文件夹
		err1 := os.MkdirAll(filepath.Dir(c.LocalFile), 0755)
		if err1 != nil {
			fmt.Printf("文件目录创建失败：%s \n", err1)
			return
		}
		fmt.Printf("OpenFile文件：%s \n", 1111)
		f, err := os.OpenFile(c.LocalFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		// 流式拷贝，主节点断开会直接报错退出
		buf := make([]byte, 1024*1024)
		_, err = io.CopyBuffer(f, resp.Body, buf)
		_ = f.Close()
		resp.Body.Close()
		log.Println("当前流连接断开，准备下一轮同步")
		time.Sleep(500 * time.Millisecond)
	}
}
