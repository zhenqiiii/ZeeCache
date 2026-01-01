package zeecache

// ByteView保存只读的真实缓存值,使用byte类型以支持任意数据类型的存储
//
// 这个其实就是有并发特性的cache中的键值对的值
//
// 实现了lru中的value接口,可以存入lru缓存
type ByteView struct {
	b []byte
}

// Len方法返回缓存值长度
//
// 实现Value接口
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice以byte切片形式返回数据拷贝
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String以string形式返回数据
func (v ByteView) String() string {
	return string(v.b)
}

// 负责生成拷贝
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
