package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash函数将字节切片映射到2^32的空间中
//
// 这里将Hash函数定义为一种类型，采取依赖注入的方式，
// 允许替换成自定义的函数
type Hash func(data []byte) uint32

// Map包含所有被哈希处理过后的真实节点key，Map是一致性哈希的主数据结构
//
// 可以理解为用节点hash值组成的导航，只存节点hash
type Map struct {
	hash     Hash           // hash:自定义hash函数
	replicas int            // replicas：虚拟节点的个数，即一个真实节点对应多少个虚拟节点
	keys     []int          // keys: 哈希环，存放了所有虚拟节点的hash，每次添加节点时都要排序一次
	hashMap  map[int]string // hashMap: 虚拟节点与真实节点的映射,键为虚拟节点哈希值，值为真实节点的名称
}

// New创建Map实例
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil { // hash函数默认为crc32.ChecksumIEEE
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add添加真实节点至Map中
//
// 接收的参数keys即为所有真实节点的名称，并根据倍数增生虚拟节点
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 增生
		for i := 0; i < m.replicas; i++ {
			// 得到虚拟节点hash值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			// 将虚拟节点加入哈希环并在hashMap中记录映射
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	// 对哈希环重新排序
	sort.Ints(m.keys)
}

// 理解：当接收到一个数据请求时，用其key计算hash，
// 然后用某种方法从hashMap中得到该hash顺时针碰到的第一个虚拟节点的hash，
// 从而用hashMap中的映射关系知道真实节点的名称（key），最后从对应节点获取数据
// 以上的这些理解，由下面的Get方法实现

// Get方法获取给定key在哈希环上最近的节点（顺时针）
//
// 也就是获取存储该数据的节点的名称，向该节点要数据
func (m *Map) Get(key string) string {
	// 没有节点
	if len(m.keys) == 0 {
		return ""
	}
	// 计算key的hash
	hash := int(m.hash([]byte(key)))
	// 二分法查找正确虚拟节点--Search方法会返回满足回调函数的最小数值，均不满足则返回len(m.keys)
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
	// Q:为什么要%？A:如果idx == len(m.keys),那么应该选择m.keys[0]
	// 也就是说该key处于第一个虚拟节点和最后一个虚拟节点之间，
	// 按顺时针的话应该存放在第一个虚拟节点
}
