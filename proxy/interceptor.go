package proxy

import (
	"net"
	"net/http"
)

// 拦截 https 流量通用接口
type interceptor interface {
	// 初始化
	Start() error
	// 传入当前客户端 req
	Dial(req *http.Request) (net.Conn, error)
}

// 直接转发 https 流量
type forward struct{}

func (i *forward) Start() error {
	return nil
}

func (i *forward) Dial(req *http.Request) (net.Conn, error) {
	return net.Dial("tcp", req.Host)
}
