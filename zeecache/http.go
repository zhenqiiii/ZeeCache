package zeecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

// 默认地址前缀
const defaultBasePath = "/_zeecache/"

// HTTPPool实现了PeerPicker接口，充当节点池的角色
//
// 实际地址：self + basePath + ...
type HTTPPool struct {
	self     string // 节点自己的地址:IP+端口
	basePath string // 节点池路径前缀，格式：/_zeecache/
}

// NewHTTPPool创建一个HTTPPool实例，即HTTP节点池
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log打出服务端信息: 某HTTPPool在某路径下接收到某类http请求
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP是对Handler的重写，用于处理所有请求
//
// 处理流程：先拿到Group缓存，再从缓存中读取数据，最后返回
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 判断请求是否以正确路径前缀开始
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	// 打日志
	p.Log("%s %s", r.Method, r.URL.Path)

	// 提取信息
	// 格式：/<basepath>/<groupname>/<key>
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 { // 格式有误：比如说不含key路径参数，那么len(parts)为1
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName := parts[0]
	key := parts[1]

	// 获取Group缓存
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	// 读取值
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回
	log.Print(view.ByteSlice())
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice())

}
