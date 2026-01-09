package main

// var db = map[string]string{
// 	"Tom":  "630",
// 	"Jack": "589",
// 	"Sam":  "567",
// }

// // 启动缓存服务端节点
// func startCacheServer(addr string, addrs []string, zee *zeecache.Group) {
// 	// 创建服务端
// 	peers := zeecache.NewHTTPPool(addr)
// 	// 更新节点列表
// 	peers.Set(addrs...)
// 	// 将group和httppool连接（注册）
// 	zee.RegisterPeers(peers)
// 	log.Println("geecache is running at", addr)
// 	// 运行
// 	log.Fatal(http.ListenAndServe(addr[7:], peers))
// }

// // 启动用户API服务
// func startAPIServer(apiAddr string, zee *zeecache.Group) {
// 	// 使用http标准库注册处理函数
// 	// 在/api路径上注册该处理函数，处理逻辑为先获取用户请求中的key
// 	// 将key传给本机的group，调用Get方法获取缓存值并返回
// 	http.Handle("/api", http.HandlerFunc(
// 		func(w http.ResponseWriter, r *http.Request) {
// 			key := r.URL.Query().Get("key")
// 			view, err := zee.Get(key)
// 			if err != nil {
// 				http.Error(w, err.Error(), http.StatusInternalServerError)
// 				return
// 			}
// 			w.Header().Set("Content-Type", "application/octet-stream")
// 			w.Write(view.ByteSlice())
// 		}))
// 	log.Println("fontend server is running at", apiAddr)
// 	// 运行
// 	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))

// }

// // 封装创建新Group缓存过程
// func createGroup() *zeecache.Group {
// 	return zeecache.NewGroup("scores", 2<<10, zeecache.GetterFunc(
// 		func(key string) ([]byte, error) {
// 			log.Println("[SlowDB] search key", key)
// 			if v, ok := db[key]; ok {
// 				return []byte(v), nil
// 			}
// 			return nil, fmt.Errorf("%s not exist", key)
// 		}))
// }

func main() {

}
