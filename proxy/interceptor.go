package proxy

import (
	"net"
	"net/http"
)

// 拦截 https 流量通用接口
type Interceptor interface {
	// 初始化
	Start() error
	// 传入当前客户端 req
	Dial(req *http.Request) (net.Conn, error)
}

// 直接转发 https 流量
type Forward struct{}

func (i *Forward) Start() error {
	return nil
}

func (i *Forward) Dial(req *http.Request) (net.Conn, error) {
	return net.Dial("tcp", req.Host)
}
