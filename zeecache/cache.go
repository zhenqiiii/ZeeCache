package zeecache

import (
	"sync"
	"zeecache/lru"
)

// 添加并发特性的cache完全体,实例化并封装了一开始实现的lru缓存
//
// 整个cache共用一把锁,保证每次只有一个对象访问
type cache struct {
	mu         sync.Mutex // 锁
	lru        *lru.Cache // 封装的lru缓存
	cacheBytes int64      // 缓存大小
}

// 添加/修改
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { // Lazy Initialization: 提高性能，减少程序内存使用
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
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
