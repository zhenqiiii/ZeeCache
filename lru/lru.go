package lru

import "container/list"

// LRU缓存，非并发安全
type Cache struct {
	maxBytes int64
	nbytes   int64
	ll       *list.List               // 双向链表
	cache    map[string]*list.Element // 字典,value存的就是双向链表的节点指针
	// 可选，元素被清除时调用，当为nil的时候，不执行操作
	OnEvicted func(key string, value Value)
}

// 这里某个记录的key实际在字典和双向链表中都有存一份
// 好处在于当双向链表中移出队首节点时，
// 同样能够得到该节点的key，从而在字典中执行删除

// 输入值，双向链表节点的数据类型:list.Element.Value属性
type entry struct {
	key   string
	value Value
}

// 值接口，使用Len方法计算大小（bytes）
//
// 实现该接口的类型均可以被缓存，从而不局限于单一类型
type Value interface {
	Len() int
}

// 实例化一个Cache
func New(maxBytes int64, OnEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: OnEvicted,
	}
}

// 查找
//
//两个步骤：1. 在字典中找到对应双向链表的节点  2. 将节点移动到队尾
//
// tips：双向链表队首队尾是相对的，这里约定front为队尾，back为队首(从队首移出)
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		// 移动到队尾
		c.ll.MoveToFront(ele)
		// 获取节点值
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// 删除
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
		// 回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 新增/修改
func (c *Cache) Add(key string, value Value) {
	// 已存在，修改并移动至队尾
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else { // 不存在，在队尾新增节点并存入字典
		ele := c.ll.PushFront(&entry{key, value})
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
