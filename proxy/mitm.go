package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

type Mitm interface {
	Start() error
	Dial(host string) (net.Conn, error)
}

// 直接转发 https 流量
type MitmForward struct{}

func (m *MitmForward) Start() error {
	return nil
}

func (m *MitmForward) Dial(host string) (net.Conn, error) {
	return net.Dial("tcp", host)
}

// 内部解析 https 流量
// 每个连接都会消耗掉两个文件描述符，可能会达到打开文件上限
type MitmServer struct {
	Proxy    *Proxy
	CA       *cert.CA
	Listener net.Listener
	Server   *http.Server
}

func NewMitmServer(proxy *Proxy) (Mitm, error) {
	ca, err := cert.NewCA("")
	if err != nil {
		return nil, err
	}

	m := &MitmServer{
		Proxy: proxy,
		CA:    ca,
	}

	server := &http.Server{
		IdleTimeout:  time.Millisecond * 100, // 尽快关闭内部的连接，释放文件描述符
		Handler:      m,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // disable http2
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Debugf("MitmServer GetCertificate ServerName: %v\n", chi.ServerName)
				return ca.GetCert(chi.ServerName)
			},
		},
	}

	m.Server = server

	return m, nil
}

func (m *MitmServer) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:") // port number is automatically chosen
	if err != nil {
		return err
	}
	m.Listener = ln
	m.Server.Addr = ln.Addr().String()
	log.Infof("MitmServer Server Addr is %v\n", m.Server.Addr)
	defer ln.Close()

	return m.Server.ServeTLS(ln, "", "")
}

func (m *MitmServer) Dial(host string) (net.Conn, error) {
	return net.Dial("tcp", m.Server.Addr)
}

func (m *MitmServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = "https"
	}

	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	m.Proxy.ServeHTTP(res, req)
}
