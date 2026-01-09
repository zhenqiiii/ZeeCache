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
	lru         *lru.Cache    // 封装的lru缓存
	cacheBytes  int64         // 缓存大小
	defaultTTL  time.Duration // 默认TTL
	stopJanitor chan struct{} // 停止清理任务的通道
}

// 添加/修改
func (c *cache) add(key string, value ByteView, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { // Lazy Initialization: 提高性能，减少程序内存使用
		c.lru = lru.New(c.cacheBytes, nil)
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
}

// 获取
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
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
				}
				c.mu.Unlock()
				// 接收到停止信号则停止
			case <-c.stopJanitor:
				return
			}
		}
	}()
}
