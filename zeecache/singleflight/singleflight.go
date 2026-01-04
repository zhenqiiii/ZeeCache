package singleflight

import "sync"

// singleflight用于避免大量的重复请求，从而避免缓存击穿/穿透，降低资源浪费

// call代表正在进行中，或者已经结束的请求，sync.WaitGroup锁避免重入
type call struct {
	wg  sync.WaitGroup
	val interface{} // 请求返回值
	err error       // 返回的错误
}

// Group是singleflight的主数据结构，存放call
type Group struct {
	mu sync.Mutex       // 给m加的锁
	m  map[string]*call //不同key的请求（call）
}

// Do方法保证瞬时情况下对于同一个key的无论多少请求，fn都只执行一次
//
// 请求的发出是通过Do方法的,而fn应该是一个发出请求的函数
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 每次对g.m进行操作时，都需要加锁，完成操作后解锁
	g.mu.Lock()
	if g.m == nil { //lazy initialization
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok { //key对应的call已经存在(第一次意外的其他请求都会走到这个分支中)
		// 已经拿到对应的call，对g.m的访问已经结束
		g.mu.Unlock()
		c.wg.Wait() // 等待请求结束
		return c.val, c.err
	}

	// 该key的请求不存在，创建一个
	c := new(call)
	c.wg.Add(1)  // 加锁，防止再次发出请求
	g.m[key] = c // 记录,表明key已经有请求在处理
	g.mu.Unlock()

	c.val, c.err = fn() //调用函数
	c.wg.Done()         // 执行完一次了，去锁

	// 从group中删去该key的请求，因为singleflight只负责瞬时并发的大量请求，如果不是同一时刻的请求还是要放行的
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err

}
