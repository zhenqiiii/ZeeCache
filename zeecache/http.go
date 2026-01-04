package zeecache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"zeecache/consistenthash"
	pb "zeecache/zeecachepb"

	"google.golang.org/protobuf/proto"
)

// 默认地址前缀
const (
	defaultBasePath = "/_zeecache/"
	defaultReplicas = 50
)

// HTTPPool实现了PeerPicker接口，充当节点池的角色，也就是单个服务端/节点
//
// 实际地址：self + basePath + ...
type HTTPPool struct {
	self     string              // 节点自己的地址:IP+端口
	basePath string              // 节点池路径前缀，格式：/_zeecache/
	mu       sync.Mutex          // 给peers和httpGetters加锁
	peers    *consistenthash.Map // 节点列表，用于根据具体key选择节点
	// 每个远程节点对应的httpGetter，因为每个远程节点的baseURL都不同，
	// 所以要使用对应的httpGetter，应该同时也会方便加锁
	httpGetters map[string]*httpGetter
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
	// 封装进protobuf的Response中,将Response作为数据体返回
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)

}

// Set方法更新单一服务端（HTTPPool）的远程节点列表peers
//
// 每次都是更新全体节点
func (p *HTTPPool) Set(peers ...string) {
	// 加锁
	p.mu.Lock()
	defer p.mu.Unlock()
	// 创建一个新的Map，并将peers...加入进去
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	// 同步更新每个节点对应的httpGetter（可以理解为客户端）
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	// 这里的peer（节点名称）就是IP+端口，也就是HTTPPool的self
	// 符合地址形式：IP&port + basePath + groupname + key
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PeerPicker根据key选择节点
//
// 实际就是对一致性哈希结构体Map的Get方法的封装
//
// 得到节点名称后返回对应节点的PeerGetter
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 获取节点名称，返回对应Getter（不能是本机节点）
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

//=========================客户端===========================//
// 这里的客户端不是说用户的交互端，而是一个节点访问另一节点时用的客户端

type httpGetter struct {
	baseURL string // 将要访问的远程节点的地址
}

// 重写PeerGetter的Get方法，用于向另一节点发送请求获取数据
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	// 拼接路径
	u := fmt.Sprintf("%v%v/%v", h.baseURL, url.QueryEscape(in.GetGroup()), url.QueryEscape(in.GetKey()))
	// 发送Get请求到服务端
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 服务端出现异常
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	// 从response读取数据
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	// 解码protobuf
	// 注意这个out是传址调用，是一个指针，会影响本身的值，所以不用返回
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

var _PeerGetter = (*httpGetter)(nil)
