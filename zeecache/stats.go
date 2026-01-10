package zeecache

import (
	"sync"
	"sync/atomic"
	"time"
)

// 实现统计监控机制的核心数据结构和方法

// Stats核心缓存统计数据结构
type Stats struct {
	// 基础指标计数器
	// 使用原子操作，因为只是计数从而无需加锁，速度快
	hits        atomic.Int64 // 缓存命中次数（指直接从本地查询到的）
	misses      atomic.Int64 // 未命中次数
	evictions   atomic.Int64 // LRU策略缓存淘汰次数
	expired     atomic.Int64 // 过期删除次数
	getterCalls atomic.Int64 // 调用本地数据源次数
	peerHits    atomic.Int64 // 从远程节点获取成功次数
	peerErrors  atomic.Int64 // 从远程节点获取失败次数

	// 状态指标
	// 这里就用不了原子操作了，因为有一定过程在里面，用锁
	keysMu     sync.RWMutex
	keysCount  int64 //当前缓存项数
	bytesCount int64 // 当前缓存字节数（只记key-value）

	// 时间统计
	startTime time.Time // 统计开始时间
}

// NewStats 创建统计对象
func NewStats() *Stats {
	return &Stats{
		startTime: time.Now(),
	}
}

// RecordHit 记录缓存命中
func (s *Stats) RecordHit() {
	s.hits.Add(1)
}

// RecordHit 记录缓存未命中
func (s *Stats) RecordMiss() {
	s.misses.Add(1)
}

// RecordLRUEviction 记录LRU淘汰
func (s *Stats) RecordLRUEviction() {
	s.evictions.Add(1)
}

// RecordExpired 记录缓存过期删除次数
func (s *Stats) RecordExpired() {
	s.hits.Add(1)
}

// RecordGetterCall 记录本地数据源调用
func (s *Stats) RecordGetterCall() {
	s.getterCalls.Add(1)
}

// RecordPeerHits 记录远程节点命中
func (s *Stats) RecordPeerHit() {
	s.peerHits.Add(1)
}

// RecordPeerError 记录远程节点访问错误
func (s *Stats) RecordPeerError() {
	s.peerErrors.Add(1)
}

// UpdateKeysCount 更新缓存项数量
//
// 每次add key时调用
func (s *Stats) UpdateKeysCount(count int64) {
	// 先上锁
	s.keysMu.Lock()
	s.keysCount = count
	s.keysMu.Unlock()
}

// Q:count和bytes哪里获取？

// UpdateBytesCount 更新缓存字节数
func (s *Stats) UpdateBytesCount(bytes int64) {
	// 先上锁
	s.keysMu.Lock()
	s.bytesCount = bytes
	s.keysMu.Unlock()
}

// Snapshot统计快照，
type StatsSnapshot struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	Expired     int64
	GetterCalls int64
	PeerHits    int64
	PeerErrors  int64
	KeysCount   int64
	BytesCount  int64
	// 派生指标
	HitRate float64       // 命中率
	QPS     float64       // 每秒查询率
	Uptime  time.Duration // 统计时长(Duration:纳秒)
}

// GetSnapshot获取统计数据快照
func (s *Stats) GetSnapshot() StatsSnapshot {
	// 总有效查询次数
	total := s.hits.Load() + s.misses.Load()
	hitRate := 0.0
	if total > 0 { // 已有访问则进行计算，否则返回0.0
		hitRate = float64(s.hits.Load()) / float64(total) * 100
	}
	// 统计时长/秒
	uptime := time.Since(s.startTime).Seconds()
	qps := 0.0
	if uptime > 0 { // 开始统计了才计算qps
		qps = float64(total) / uptime
	}

	// 要读取keysCount和bytesCount,所以加读锁
	s.keysMu.RLock()
	defer s.keysMu.RUnlock()

	// 返回snapshot
	return StatsSnapshot{
		Hits:        s.hits.Load(),
		Misses:      s.misses.Load(),
		Evictions:   s.evictions.Load(),
		Expired:     s.expired.Load(),
		GetterCalls: s.getterCalls.Load(),
		PeerHits:    s.peerHits.Load(),
		PeerErrors:  s.peerErrors.Load(),
		KeysCount:   s.keysCount,
		BytesCount:  s.bytesCount,
		HitRate:     hitRate,
		QPS:         qps,
		Uptime:      time.Duration(uptime * float64(time.Second)),
	}
}

// Reset 重置统计数据
//
// 供上层调用，实现重置功能
func (s *Stats) Reset() {
	s.hits.Store(0)
	s.misses.Store(0)
	s.evictions.Store(0)
	s.expired.Store(0)
	s.getterCalls.Store(0)
	s.peerHits.Store(0)
	s.peerErrors.Store(0)
	s.keysMu.Lock()
	s.startTime = time.Now()
	s.keysMu.Unlock()
	// keysCount和bytesCount不重置，否则不符合实际意义
}
