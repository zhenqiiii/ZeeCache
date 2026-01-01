package zeecache

import (
	"fmt"
	"log"
	"sync"
)

// 该部分代码负责与外部交互，控制缓存存储和获取的主流程
// 缓存未击中时，从数据源获取数据

// 下面回调Getter的实现是用一个叫接口型函数的东西做的，
// 让函数实现某个接口，这样在传入回调函数的参数时，参数既可以是函数也可以是同样实现该接口的结构体
// 以对象名.Get()的形式调用即可

// Getter 加载某个key对应的值（数据）到缓存中
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现Getter接口
type GetterFunc func(key string) ([]byte, error)

// Get实际上就是GetterFunc，调用Get也就是调用GetterFunc本身
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group：最核心数据结构，Group是一个缓存的命名空间
//
// 每个Group都是一个缓存,也即Group才是cache的完全体
type Group struct {
	name      string // Group的唯一名称
	getter    Getter // 缓存未命中时获取源数据的callback
	mainCache cache  // 并发缓存
}

var (
	mu     sync.RWMutex // 读写互斥锁
	groups = make(map[string]*Group)
)

// NewGroup创建Group实例
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	// 回调函数为nil
	if getter == nil {
		panic("nil Getter")
	}

	// 处于创建过程中时，加锁
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g

}

// GetGroup 返回由NewGroup创建的对应名称的group，不存在时返回nil
func GetGroup(name string) *Group {
	// 加读锁
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get从缓存中获取某个key的value
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 流程1：缓存命中，直接从缓存中读取
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[ZeeCache] hit")
		return v, nil
	}

	// 流程2/3：未命中，从远程节点读取/本地读取（2是远程，后面实现）
	return g.load(key)
}

// load处理缓存未命中时的情况, 分为两种: 1. 本地读取  2. 从远程节点获取
func (g *Group) load(key string) (value ByteView, err error) {
	// 3: 本地读取
	return g.getLocally(key)
	// 2: 远程节点获取
}

// getLocally从本地读取数据
func (g *Group) getLocally(key string) (ByteView, error) {
	// 回调Getter
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	// 拿到值(注意此时的bytes是byte切片类型)
	value := ByteView{b: cloneBytes(bytes)}
	// 写入cache
	g.populateCache(key, value)
	return value, nil
}

// 差一个远程获取函数：getFromPeer

// populateCache将读取到的数据写入缓存
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
