package server

import (
	"errors"
	"flag"
	"fmt"
	"github.com/hashicorp/raft"
	raftBoltdb "github.com/hashicorp/raft-boltdb"
	"github.com/vision9527/raft-demo/fsm"
	"mumu.com/common/config"
	"mumu.com/common/logger"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// httpAddr    string
	raftAddr string
	raftId   string
	// raftCluster string
	// raftDir string
)

type MonitorRaftSelection struct {
	isExistLeader        bool   //是否存在leader
	leaderIpAddr         string //领导主机地址
	dataStorageDirectory string //数据存储目录
	monitorHostAddr      string //监控的所有节点地址
	raftAddr             string
	raftId               string
}

func init() {
	//flag.StringVar(&httpAddr, "http_addr", "127.0.0.1:7001", "http listen addr")
	flag.StringVar(&raftAddr, "raft_addr", "127.0.0.1:7000", "raft listen addr")
	flag.StringVar(&raftId, "raft_id", "1", "raft id")
	//flag.StringVar(&raftCluster, "raft_cluster", "1/127.0.0.1:7000,2/127.0.0.1:8000,3/127.0.0.1:9000", "cluster info")
}

// GetConfigInfo 获取配置文件信息
func (mrs *MonitorRaftSelection) GetConfigInfo() (bool, error) {
	flag.Parse()
	// 初始化配置
	if raftAddr == "" || raftId == "" {
		os.Exit(1)
		return false, errors.New("config error: raftAddr|raftId is a null character")
	}
	mrs.raftAddr = raftAddr
	mrs.raftId = raftId
	//读取yml
	hostAddr, err := config.TwoConfigParam{}.GetConfigParam("../monitor/config/MonitorConfig.yaml", "monitor", "HostAddr")
	if err != nil {
		logger.Error.Printf("%s", err)
		return false, err
	}
	hostAddrArr := strings.Split(hostAddr, ",")
	i := 0
	var newLeaderIpAddr []string
	for _, v := range hostAddrArr {
		newLeaderIpAddr[i] = fmt.Sprintf("%d/%s", i+1, v)
		i++
	}
	mrs.monitorHostAddr = strings.Join(newLeaderIpAddr, ",")
	dataStorageDirectory, err := config.TwoConfigParam{}.GetConfigParam("../monitor/config/MonitorConfig.yaml", "monitor", "DataStorageDirectory")
	if err != nil {
		logger.Error.Printf("%s", err)
		return false, err
	}

	mrs.dataStorageDirectory = dataStorageDirectory + "/raft_" + raftId
	os.MkdirAll(mrs.dataStorageDirectory, 0700)
	return true, nil
}

// NewRaftSelection 实例化raft选举信息
func (mrs *MonitorRaftSelection) NewRaftSelection() (*raft.Raft, *fsm.Fsm, error) {
	raftAddr := mrs.raftAddr
	raftId := mrs.raftId
	raftDir := mrs.dataStorageDirectory
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(raftId)
	// config.HeartbeatTimeout = 1000 * time.Millisecond
	// config.ElectionTimeout = 1000 * time.Millisecond
	// config.CommitTimeout = 1000 * time.Millisecond

	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return nil, nil, err
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 2, 5*time.Second, os.Stderr)
	if err != nil {
		return nil, nil, err
	}
	snapshots, err := raft.NewFileSnapshotStore(raftDir, 2, os.Stderr)
	if err != nil {
		return nil, nil, err
	}
	logStore, err := raftBoltdb.NewBoltStore(filepath.Join(raftDir, "raft-log.db"))
	if err != nil {
		return nil, nil, err
	}
	stableStore, err := raftBoltdb.NewBoltStore(filepath.Join(raftDir, "raft-stable.db"))
	if err != nil {
		return nil, nil, err
	}
	fm := new(fsm.Fsm)
	fm.DataBase.Data = make(map[string]string)

	rf, err := raft.NewRaft(config, fm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, nil, err
	}

	return rf, fm, nil
}

func (mrs *MonitorRaftSelection) Bootstrap(rf *raft.Raft) {
	raftCluster := mrs.monitorHostAddr
	servers := rf.GetConfiguration().Configuration().Servers
	if len(servers) > 0 {
		return
	}
	peerArray := strings.Split(raftCluster, ",")
	if len(peerArray) == 0 {
		return
	}

	var configuration raft.Configuration
	for _, peerInfo := range peerArray {
		peer := strings.Split(peerInfo, "/")
		id := peer[0]
		addr := peer[1]
		server := raft.Server{
			ID:      raft.ServerID(id),
			Address: raft.ServerAddress(addr),
		}
		configuration.Servers = append(configuration.Servers, server)
	}
	rf.BootstrapCluster(configuration)
	return
}

// GetGoLeaderIpAddr 获取领导的IP和端口
func (mrs *MonitorRaftSelection) GetGoLeaderIpAddr(myRaft *raft.Raft) {
	// 监听leader变化
	go func() {
		for leader := range myRaft.LeaderCh() {
			mrs.isExistLeader = leader
			//logger.Info.Printf("Leader: %s", isLeader) //bool判断是否有领导
			if mrs.isExistLeader {
				//获取leader,地址和端口
				leaderIpAddr, _ := myRaft.LeaderWithID()
				mrs.leaderIpAddr = fmt.Sprintf("%s", leaderIpAddr)
				//leaderAddr := myRaft.Leader()
				//logger.Info.Printf("Leader:%s=>%s", mrs.leaderIpAddr, leaderAddr) //bool判断是否有领导
			}
		}
	}()
}

// GetLeaderIpAddr 获取主IP端口
func (mrs *MonitorRaftSelection) GetLeaderIpAddr() string {
	return mrs.leaderIpAddr
}

// MonitorRaftSelectionRun 选主
func MonitorRaftSelectionRun() (MonitorRaftSelection, error) {
	monitorRaftSelection := MonitorRaftSelection{}
	_, err := monitorRaftSelection.GetConfigInfo()
	if err == nil {
		logger.Error.Println(err)
		return monitorRaftSelection, err
	}
	monitorRaftSelection.NewRaftSelection()
	// 初始化raft
	myRaft, _, err := monitorRaftSelection.NewRaftSelection()
	if err != nil {
		os.Exit(1)
		return monitorRaftSelection, err
	}

	// 启动raft
	monitorRaftSelection.Bootstrap(myRaft)
	//获取领导的IP和端口
	monitorRaftSelection.GetGoLeaderIpAddr(myRaft)
	return monitorRaftSelection, err
}
