package main

import (
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

//
// When the proxy forwards HTTPS requests, go-mitmproxy by default initiates an SSL connection with the target server first.
// 1. If you do not want to establish a connection with the target server, such as:
//    Generating a response directly in the RequestHeaders or Request Hook and returning it to the client.
// 2. Or if you want the proxy to establish a connect connection with the client first, delaying the connection with the target service until a real HTTPS request is made.
//
// => Then you can refer to the following code
//    set client.UpstreamCert = false in the ClientConnected Hook.

//
// 当代理 https 请求时，go-mitmproxy 默认会先和目标服务器建立 SSL 连接
// 1. 如果你并不和目标服务器建立连接，如：
//    在 Requestheaders 或 Request Hook 中直接生成 Response，返回给客户端
// 2. 或者想让中间人先和客户端成功建立 connect 连接，推迟至真正发起 https 请求时中间人和目标服务再建立连接
//
// => 那么你可以参考下面代码
//    在 ClientConnected Hook 中设置 client.UpstreamCert = false

type CloseConn struct {
	proxy.BaseAddon
}

func (a *CloseConn) ClientConnected(client *proxy.ClientConn) {
	// necessary
	client.UpstreamCert = false
}

func (a *CloseConn) Requestheaders(f *proxy.Flow) {
	// give some response to client
	// then will not request remote server
	f.Response = &proxy.Response{
		StatusCode: 502,
	}
}

func main() {
	opts := &proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	p.AddAddon(&CloseConn{})
	p.AddAddon(&proxy.LogAddon{})

	log.Fatal(p.Start())
}
