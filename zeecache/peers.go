package zeecache

import pb "zeecache/zeecachepb"

// PeerPicker接口用于确定某个key所在的节点
type PeerPicker interface {
	// PickPeer方法用于根据传入的key选择相应节点的PeerGetter
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter由节点peer实现,对应于客户端
type PeerGetter interface {
	// Get方法用于从对应的group查找缓存值
	Get(in *pb.Request, out *pb.Response) error
}
