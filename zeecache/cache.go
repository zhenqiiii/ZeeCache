package zeecache

import (
	"sync"
	"time"
	"zeecache/lru"
)

// 添加并发特性的cache完全体,实例化并封装了一开始实现的lru缓存
//
// 整个cache共用一把锁,保证每次只有一个对象访问
type cache struct {
	mu          sync.Mutex    // 锁
	lru         *lru.Cache    // 封装的lru缓存,懒加载
	cacheBytes  int64         // 缓存大小
	defaultTTL  time.Duration // 默认TTL
	stopJanitor chan struct{} // 停止清理任务的通道，懒加载
	stats       *Stats        // 数据统计对象
}

// Stats 获取统计对象,包外访问需要，但是现在似乎多余
func (c *cache) Stats() *Stats {
	return c.stats
}

// NewCache 创建带统计的cache
func NewCache(cacheBytes int64, defaultTTL time.Duration) *cache {
	// 统计对象
	stats := NewStats()
	c := &cache{
		cacheBytes: cacheBytes,
		defaultTTL: defaultTTL,
		stats:      stats,
	}
	return c
}

// 添加/修改
func (c *cache) add(key string, value ByteView, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { // Lazy Initialization: 提高性能，减少程序内存使用
		c.lru = lru.New(c.cacheBytes,
			nil,
			func(s string, v lru.Value) {
				// 感觉都没必要打日志，不然数据量大刷屏了
				// 重要信息都找不到，先保留吧，有用再开，反正是回调
				// log.Printf("[LRU Eviction]: %s", s)
				// 统计
				// lru淘汰数量
				c.stats.RecordLRUEviction()
				// keysCount,bytesCount
				if keys, bytes := c.lru.GetStatsInfo(); c.lru != nil {
					c.stats.UpdateKeysCount(keys)
					c.stats.UpdateBytesCount(bytes)
				}
			},
			func(s string, v lru.Value) {
				// log.Printf("[Expired]: %s", s)
				c.stats.RecordExpired()
			},
		)
	}
	// 设置了ttl或者初始化时设定了默认ttl，则启动定时清理机制
	if ttl > 0 || c.defaultTTL > 0 {
		c.startJanitor()
	}

	// 计算过期时间,优先级：ttl > defaultTTL
	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).UnixNano()
	} else if c.defaultTTL > 0 {
		expiresAt = time.Now().Add(c.defaultTTL).UnixNano()
	}
	// 添加
	c.lru.AddWithTTL(key, value, expiresAt)

	// 更新状态统计:keysCount,bytesCount
	if keys, bytes := c.lru.GetStatsInfo(); c.lru != nil {
		c.stats.UpdateKeysCount(keys)
		c.stats.UpdateBytesCount(bytes)
	}
}

// 获取（只从缓存中）
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 在此处计算hit和miss数，
	// 那么后续得到的HitRate为本地缓存命中率
	// 但QPS正确，请求是全部计算到了的
	// 因为所有请求都会先从本地缓存走一遍
	if c.lru == nil {
		// 出于某种原因没有lru，也算miss
		c.stats.RecordMiss()
		return
	}
	if v, ok := c.lru.Get(key); ok {

		c.stats.RecordHit()
		return v.(ByteView), ok
	}
	// 没找到
	c.stats.RecordMiss()
	return
}

// startJantior方法在后台运行一个定时清理协程，
//
// 用于实现定时清理机制
//
// 当ttl/defaultTTL不为0时，自动调用方法启动Janitor
func (c *cache) startJanitor() {
	if c.stopJanitor != nil {
		return // 已经启动
	}
	// 创建停止通道
	c.stopJanitor = make(chan struct{})
	// 运行
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
		defer ticker.Stop()
		for {
			select {
			// 计时器触发清理操作
			case <-ticker.C:
				c.mu.Lock()
				if c.lru != nil {
					c.lru.RemoveExpired()
					// 更新统计：keysCount、bytesCount
					// 负责过期清理时的数据更新
					// LRU淘汰后的数据更新在回调中实现：OnLRUEviction
					keys, bytes := c.lru.GetStatsInfo()
					c.stats.UpdateKeysCount(keys)
					c.stats.UpdateBytesCount(bytes)
				}
				c.mu.Unlock()
				// 接收到停止信号则停止
			case <-c.stopJanitor:
				return
			}
		}
	}()
}
