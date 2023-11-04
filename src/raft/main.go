package main

import "sync"

// MasterNode  声明Leader对象ip,port
type MasterNode struct {
	IP   string
	Port uint32
}

type Raft struct {
	mutex           sync.Mutex //锁
	addrPort        string     //节点编号ip:port
	currentTerm     int        //当前任期
	voteFor         string     //为那个节点投票ip:port
	state           int        //3个状态，0:follower,1:candidate,2:leader
	lastMessageTime int64      //发送最后一条数据的时间
	currentLeader   int        //设置当前节点的领导
	message         chan bool  //节点间发信息的通道
	electCh         chan bool  //选举通道
	heartBeat       chan bool  //心跳信号通道
	heartbeatRe     chan bool  //返回心跳信号的通道
	timeout         int        //超时时间
}
