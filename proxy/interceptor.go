package proxy

import (
	"net"
)

// 拦截 https 流量通用接口
type Interceptor interface {
	// 初始化
	Start() error
	// 针对每个 host 连接
	Dial(host string) (net.Conn, error)
}

// 直接转发 https 流量
type Forward struct{}

func (i *Forward) Start() error {
	return nil
}

func (i *Forward) Dial(host string) (net.Conn, error) {
	return net.Dial("tcp", host)
}
