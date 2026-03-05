package snowflake

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// -------------------------- 1. 定义包级常量（保留dataCenterID的位分配）--------------------------
const (
	timeBits   int64 = 30 // 时间戳位数（30位：毫秒级，可用约34.05年）
	dataBits   int64 = 3  // 数据中心ID位数（3位：0~7）
	workerBits int64 = 18 // 机器ID位数（保留18位（0~262143），满足999999需求）
	seqBits    int64 = 12 // 序列号位数（12位：0~4095）
)

// 最大值计算（基于新位分配）
const (
	maxDataCenterID int64 = (1 << dataBits) - 1   // 数据中心ID最大值：31
	maxWorkerID     int64 = (1 << workerBits) - 1 // 机器ID最大值：1048575
	maxSequence     int64 = (1 << seqBits) - 1    // 序列号最大值：4095
	maxTimestamp    int64 = (1 << timeBits) - 1   // 时间戳最大值：67108863（毫秒）
)

// 位移偏移量（组装ID时的位运算偏移）
const (
	timeShift   int64 = dataBits + workerBits + seqBits // 时间戳左移：5+20+12=37位
	dataShift   int64 = workerBits + seqBits            // 数据中心ID左移：20+12=32位
	workerShift int64 = seqBits                         // 机器ID左移：12位
)

// -------------------------- 2. 定义Snowflake结构体（恢复dataCenterID字段）--------------------------
type Snowflake struct {
	epoch        int64        // 起始时间戳（毫秒，每个实例可自定义）
	dataCenterID int64        // 数据中心ID（0~31）
	workerID     int64        // 机器ID（0~1048575）
	lastTime     atomic.Int64 // 上一次生成ID的时间戳（原子操作）
	sequence     atomic.Int64 // 当前毫秒内序列号（原子操作）
}

// -------------------------- 3. 构造函数（恢复dataCenterID参数校验）--------------------------
func NewSnowflake(dataCenterID, workerID int64, epoch int64) (*Snowflake, error) {
	// 校验数据中心ID合法性
	if dataCenterID < 0 || dataCenterID > maxDataCenterID {
		return nil, fmt.Errorf("dataCenterID 必须在 0-%d 之间（当前传入：%d）", maxDataCenterID, dataCenterID)
	}
	// 校验机器ID合法性（支持999999）
	if workerID < 0 || workerID > maxWorkerID {
		return nil, fmt.Errorf("workerID 必须在 0-%d 之间（当前传入：%d）", maxWorkerID, workerID)
	}
	// 校验起始时间戳合法性
	if epoch <= 0 {
		return nil, errors.New("epoch 必须是有效的正时间戳（毫秒）")
	}

	// 初始化实例
	sf := &Snowflake{
		epoch:        epoch,
		dataCenterID: dataCenterID,
		workerID:     workerID,
	}
	sf.lastTime.Store(-1) // 初始化为-1，确保首次生成正常
	sf.sequence.Store(0)  // 初始序列号为0
	return sf, nil
}

// -------------------------- 4. 核心方法：无锁生成ID（同步调整位运算逻辑）--------------------------
func (s *Snowflake) NextID() (int64, error) {
	for {
		// 原子读取当前状态（无锁）
		lastTime := s.lastTime.Load()
		currentSeq := s.sequence.Load()
		currentTime := time.Now().Unix()

		// 1. 计算相对起始时间的时间戳
		relativeTime := currentTime - s.epoch
		if relativeTime < 0 {
			return 0, errors.New("当前时间早于起始时间（epoch）")
		}
		// 校验时间戳是否超出算法上限
		if relativeTime > maxTimestamp {
			return 0, fmt.Errorf("时间戳超出算法上限：最大支持相对时间 %d 毫秒（约1.9年）", maxTimestamp)
		}

		// 2. 处理时间回退（避免ID重复）
		if currentTime < lastTime {
			return 0, fmt.Errorf("系统时间回退：当前时间(%d) < 上一次生成时间(%d)", currentTime, lastTime)
		}

		// 3. 计算新序列号
		var newSeq int64
		if currentTime == lastTime {
			// 同一毫秒：序列号自增
			newSeq = currentSeq + 1
			if newSeq > maxSequence {
				// 序列号用尽，自旋等待下一毫秒
				continue
			}
		} else {
			// 新毫秒：重置序列号为0
			newSeq = 0
		}

		// 4. CAS原子更新状态（无锁核心：确保并发安全）
		if s.lastTime.CompareAndSwap(lastTime, currentTime) &&
			s.sequence.CompareAndSwap(currentSeq, newSeq) {

			// 5. 组装64位ID（基于新的位分配和偏移量）
			id := relativeTime<<timeShift | // 时间戳部分（左移37位）
				s.dataCenterID<<dataShift | // 数据中心ID部分（左移32位）
				s.workerID<<workerShift | // 机器ID部分（左移12位）
				newSeq // 序列号部分（占12位）
			return id, nil
		}

		// CAS失败：自旋重试（无锁竞争，性能优于互斥锁）
	}
}

// -------------------------- 5. ID解析方法（还原所有字段，方便排查）--------------------------
func (s *Snowflake) ParseID(id int64) (map[string]interface{}, error) {
	// 位运算提取各字段（与组装逻辑反向）
	relativeTime := (id >> timeShift) & maxTimestamp
	dataCenterID := (id >> dataShift) & maxDataCenterID
	workerID := (id >> workerShift) & maxWorkerID
	sequence := id & maxSequence

	// 还原原始时间戳和具体时间
	rawTimestamp := s.epoch + relativeTime
	generateTime := time.UnixMilli(rawTimestamp).Format("2006-01-02 15:04:05.999")
	epochTime := time.UnixMilli(s.epoch).Format("2006-01-02 15:04:05")

	return map[string]interface{}{
		"id":             id,
		"data_center_id": dataCenterID,
		"worker_id":      workerID,
		"sequence":       sequence,
		"relative_time":  relativeTime, // 相对起始时间的毫秒数
		"raw_timestamp":  rawTimestamp, // 原始时间戳（毫秒）
		"generate_time":  generateTime,
		"epoch":          epochTime,
		"max_life":       "约1.9年", // 算法时间上限提示
	}, nil
}
