
#### 1.集群的文件目录
```go
cluster/
├── main.go          # 入口文件（启动集群、故障模拟）
├── cluster.go       # 集群核心结构体与初始化
├── slot.go          # 哈希槽计算与初始化分配
├── storage.go       # 数据存储与持久化（落盘）
├── sync.go          # 主从同步（全量+增量+断点续传）
├── raft.go          # Raft简化选举与故障转移
├── gossip.go        # Gossip协议（节点状态同步）
├── api.go           # HTTP API接口（客户端交互）
├── utils.go         # 工具函数（curl、ID生成等）
├── shrink.go        # 集群缩容（槽位迁移+节点下线）
└── scale.go         # 集群扩容（槽位均分+并行迁移）
```
#### 三、使用示例
##### 1. String 操作（兼容原有逻辑）
```go
# 写入 String
curl http://localhost:6379/6379/Set?key=name&val=redis-cluster
# 读取 String
curl http://localhost:6379/6379/Get?key=name
```
##### 2. Hash 操作（新增）
```go
# 写入 Hash
curl http://localhost:6379/6379/HSet?key=user:1&field=name&val=zhangsan
curl http://localhost:6379/6379/HSet?key=user:1&field=age&val=20
# 读取 Hash
curl http://localhost:6379/6379/HGet?key=user:1&field=name
```
##### 3. 验证数据一致性
```go
# 从从节点读取 Hash（验证同步）
curl http://localhost:6382/6382/HGet?key=user:1&field=age
# 查看落盘文件（redis-cluster-data 目录下）
cat redis-cluster-data/6379.data
```
##### 4. HMSet 批量设置 Hash 字段
```go
# 批量设置user:1的3个字段（name/age/city）
curl "http://localhost:6379/6379/HMSet?key=user:1&name=zhangsan&age=20&city=beijing"
```
##### 5. HMGet 批量获取 Hash 字段
```go
# 批量获取user:1的name和city字段
curl http://localhost:6379/6379/HMGet?key=user:1&fields=name,city
# 响应结果
{"key":"user:1","values":{"name":"zhangsan","city":"beijing"}}
```
