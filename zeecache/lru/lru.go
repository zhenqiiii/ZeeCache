package lru

import (
	"container/list"
	"time"
)

// LRU缓存，非并发安全
//
// 第一层
type Cache struct {
	maxBytes int64
	nbytes   int64                    // 只记录key和value的总byte，看作所缓存内容的总字节长度
	ll       *list.List               // 双向链表
	cache    map[string]*list.Element // 字典,value存的就是双向链表的节点指针
	// 可选，元素被清除时调用，当为nil的时候，不执行操作
	OnEvicted func(key string, value Value)

	// 新增：统计回调
	OnLRUEvicted func(key string, value Value) // LRU淘汰回调（在Cache层封装Stats的RecordEviction方法）
	OnExpired    func(key string, value Value) // 过期删除回调，同上
}

// 这里某个记录的key实际在字典和双向链表中都有存一份
// 好处在于当双向链表中移出队首节点时，
// 同样能够得到该节点的key，从而在字典中执行删除

// 输入值，双向链表节点的数据类型:list.Element.Value属性
type entry struct {
	key       string
	value     Value
	expiresAt int64 // 过期时间戳(nano,绝对)
}

// 值接口，使用Len方法计算大小（bytes）
//
// 实现该接口的类型均可以被缓存，从而不局限于单一类型
type Value interface {
	Len() int
}

// 实例化一个Cache
func New(maxBytes int64, OnEvicted, OnLRUEvicted, OnExpired func(string, Value)) *Cache {
	return &Cache{
		maxBytes:     maxBytes,
		ll:           list.New(),
		cache:        make(map[string]*list.Element),
		OnEvicted:    OnEvicted,
		OnLRUEvicted: OnLRUEvicted,
		OnExpired:    OnExpired,
	}
}

// 查找
//
// 两个步骤：1. 在字典中找到对应双向链表的节点  2. 将节点移动到队尾
//
// tips：双向链表队首队尾是相对的，这里约定front为队尾，back为队首(从队首移出)
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		// 获取节点值
		kv := ele.Value.(*entry)
		// 过期检查逻辑:惰性过期
		// 有设置过期时间且已经过期
		if kv.expiresAt > 0 && time.Now().UnixNano() > kv.expiresAt {
			c.Remove(kv.key) //删除过期数据
			return nil, false
		}
		// 没过期
		// 移动到队尾
		c.ll.MoveToFront(ele)
		return kv.value, true
	}
	return
}

// 根据给定key执行删除操作
//
// 为了复用性就把入参写成了key而非element
func (c *Cache) Remove(key string) {
	// 找到了就删除,没找到也就不存在,也就不用删除了
	if ele, ok := c.cache[key]; ok {
		// 从dl和字典中移除
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		// 更新cache长度
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// OnEvicted
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}

}

// 删除(lru策略)
//
// 移出最近最少访问的节点（队首）
func (c *Cache) RemoveOldest() {
	// 获取队首节点
	ele := c.ll.Back()
	if ele != nil {
		// 从dl和字典中移除
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		// 更新cache大小
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 通用回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
		// LRU淘汰统计回调
		if c.OnLRUEvicted != nil {
			c.OnLRUEvicted(kv.key, kv.value)
		}
	}
}

// RemoveExpired方法遍历双向链表,删除已经过期的元素
//
// 这层写好后,给上层调用
func (c *Cache) RemoveExpired() {
	// 获取当前时间戳
	now := time.Now().UnixNano()
	// 从队首开始遍历,也就是先删除那些即将被LRU淘汰的
	for ele := c.ll.Back(); ele != nil; {
		next := ele.Prev() // 先拿到下一个节点
		// 判断过期与否
		kv := ele.Value.(*entry)
		if kv.expiresAt > 0 && now > kv.expiresAt {
			c.Remove(kv.key)
			// 过期统计回调
			if c.OnExpired != nil {
				c.OnExpired(kv.key, kv.value)
			}
		}
		ele = next
	}

}

// 新增/修改
func (c *Cache) AddWithTTL(key string, value Value, ttl int64) {
	// 已存在，修改并移动至队尾
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else { // 不存在，在队尾新增节点并存入字典,设置过期时间
		ele := c.ll.PushFront(&entry{key, value, ttl})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	// 超出最大限制，移出队首节点（最近最少访问）直到符合大小限制
	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}

}

// 用于获取添加了多少条数据(链表中节点个数)
func (c *Cache) Len() int {
	return c.ll.Len()
}

// GetStatsInfo 返回
//
// keysCount（缓存项数量）和bytesCount（缓存字节数）两个统计信息
func (c *Cache) GetStatsInfo() (keys int64, bytes int64) {
	return int64(c.ll.Len()), c.nbytes
}
