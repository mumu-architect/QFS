package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cornelk/hashmap"
	"github.com/hashicorp/raft"
	draftboard "github.com/hashicorp/raft-boltdb"
	"github.com/vision9527/raft-demo/fsm"
	"mumu.com/common/logger"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type P struct {
	nodeMap hashmap.Map[string, P1]
}
type P1 struct {
	name string
	age  int
}

var instance *P
var once sync.Once

func GetInstance() *P {
	once.Do(func() {
		instance = &P{}
	})
	return instance
}

type Person struct {
	Name string
	Age  int
}

var (
	httpAddr    string
	raftAddr    string
	raftId      string
	raftCluster string
	raftDir     string
)

var (
	isLeader     bool
	leaderIpAddr string
)

func init() {
	flag.StringVar(&httpAddr, "http_addr", "127.0.0.1:7001", "http listen addr")
	flag.StringVar(&raftAddr, "raft_addr", "127.0.0.1:7000", "raft listen addr")
	flag.StringVar(&raftId, "raft_id", "1", "raft id")
	flag.StringVar(&raftCluster, "raft_cluster", "1/127.0.0.1:7000,2/127.0.0.1:8000,3/127.0.0.1:9000", "cluster info")
}

func NewMyRaft(raftAddr, raftId, raftDir string) (*raft.Raft, *fsm.Fsm, error) {
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
	logStore, err := draftboard.NewBoltStore(filepath.Join(raftDir, "raft-log.db"))
	if err != nil {
		return nil, nil, err
	}
	stableStore, err := draftboard.NewBoltStore(filepath.Join(raftDir, "raft-stable.db"))
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

func Bootstrap(rf *raft.Raft, raftId, raftAddr, raftCluster string) {
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

type HttpServer struct {
	ctx *raft.Raft
	fsm *fsm.Fsm
}

// Result json返回数据格式
type Result struct {
	Res     bool   `json:"res"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// 对象转json
func jsonEncode(w http.ResponseWriter, data any) {
	jsonData, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func Success(w http.ResponseWriter, data any) {
	result := Result{
		Res:     true,
		Code:    200,
		Message: "ok",
		Data:    data,
	}
	jsonEncode(w, result)
}
func (h HttpServer) Set(w http.ResponseWriter, r *http.Request) {
	if !isLeader {
		fmt.Fprintf(w, "not leader")
		return
	}
	vars := r.URL.Query()
	key := vars.Get("key")
	value := vars.Get("value")
	if key == "" || value == "" {
		fmt.Fprintf(w, "error key or value")
		return
	}

	data := "set" + "," + key + "," + value
	future := h.ctx.Apply([]byte(data), 5*time.Second)
	if err := future.Error(); err != nil {
		fmt.Fprintf(w, "error:"+err.Error())
		return
	}
	//fmt.Fprintf(w, "ok")
	var data2 map[string]string
	if data2 == nil {
		data2 = make(map[string]string)
	}
	Success(w, data2)
	return
}

func (h HttpServer) Get(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()
	key := vars.Get("key")
	if key == "" {
		fmt.Fprintf(w, "error key")
		return
	}
	value := h.fsm.DataBase.Data[key]
	logger.Info.Printf("key: %s", key)
	logger.Info.Printf("value: %s", value)
	//fmt.Fprintf(w, "%s", value)
	var data map[string]string
	if data == nil {
		data = make(map[string]string)
	}
	//strKey := fmt.Sprintf("%s", key)
	data[key] = value
	Success(w, data)
	return
}
func main() {
	flag.Parse()
	// 初始化配置
	if httpAddr == "" || raftAddr == "" || raftId == "" || raftCluster == "" {
		fmt.Println("config error")
		os.Exit(1)
		return
	}
	raftDir := "data/raft_" + raftId
	os.MkdirAll(raftDir, 0700)

	// 初始化raft
	myRaft, fm, err := NewMyRaft(raftAddr, raftId, raftDir)
	if err != nil {
		fmt.Println("NewMyRaft error ", err)
		os.Exit(1)
		return
	}

	// 启动raft
	Bootstrap(myRaft, raftId, raftAddr, raftCluster)

	// 监听leader变化
	go func() {
		for leader := range myRaft.LeaderCh() {
			isLeader = leader
			//logger.Info.Printf("Leader: %s", isLeader) //bool判断是否有领导
			if isLeader {
				leaderIpAddr, _ := myRaft.LeaderWithID() //获取leader,地址和端口
				leaderAddr := myRaft.Leader()
				logger.Info.Printf("Leader:%s=>%s", leaderIpAddr, leaderAddr) //bool判断是否有领导
			}
		}
	}()

	// 启动http server
	httpServer := HttpServer{
		ctx: myRaft,
		fsm: fm,
	}

	http.HandleFunc("/set", httpServer.Set)
	http.HandleFunc("/get", httpServer.Get)
	http.ListenAndServe(httpAddr, nil)

	// 关闭raft
	shutdownFuture := myRaft.Shutdown()
	if err := shutdownFuture.Error(); err != nil {
		fmt.Printf("shutdown raft error:%v \n", err)
	}

	// 退出http server
	fmt.Println("shutdown kv http server")

	/*

		//读取yml
		config.GetConfig("../Monitor/config/MonitorConfig.yaml")

		// 创建一个Person对象
		person := Person{
			Name: "John",
			Age:  30,
		}

		// 将Person对象转换为JSON字符串
		jsonData, err := json.Marshal(person)
		if err != nil {
			fmt.Println("转换为JSON时发生错误:", err)
			return
		}

		// 打印JSON字符串
		fmt.Println(string(jsonData))

		// 获取当前时间
		now := time.Now()

		// 输出时间戳（秒）
		fmt.Println("时间戳（秒）:", now.Unix())
		now = time.Now()
		// 输出时间戳（纳秒）
		fmt.Println("时间戳（毫秒）:", now.UnixMilli())
		now = time.Now()
		// 输出时间戳（微秒）
		fmt.Println("时间戳（微秒）:", now.UnixMicro())
		now = time.Now()
		// 输出时间戳（纳秒）
		fmt.Println("时间戳（纳秒）:", now.UnixNano())
		now = time.Now()
		// 输出时间戳（纳秒）
		fmt.Println("时间戳（纳秒）:", now.UnixNano())
		logger.Trace.Println("Trace31231231")
		logger.Info.Println("Info31231231")
		logger.Warning.Printf("debug %s", "secret")
		logger.Error.Println("Error31231231")
		logger.Info.Printf("debug %s", "Info31231231")
		m := hashmap.New[string, int]()
		m.Set("amount", 123)
		m.Set("amount", 234)
		//a := GetInstance()

		p1 := P1{}
		var b *hashmap.Map[string, P1] = hashmap.New[string, P1]()
		b.Set("amount", p1)
		value, _ := b.Get("amount")

		logger.Info.Println(value)

		logger.Info.Println(m.Get("Aamount"))
		for i := 0; i < 10000; i++ {
			m.Set("A"+strconv.Itoa(i), i)
		}

		t := time.Now().UnixNano()
		logger.Info.Println(t)
		logger.Info.Println(m.Get("A5000"))
		e := time.Now().UnixNano()
		logger.Info.Println(e)
		tt := e - t
		logger.Info.Println(tt)
	*/
}
