package proxy

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

// 模拟了标准库中 server 运行，目的是仅通过当前进程内存转发 socket 数据，不需要经过 tcp 或 unix socket

// mock net.Listener
type listener struct {
	connChan chan net.Conn
}

func (l *listener) Accept() (net.Conn, error) { return <-l.connChan, nil }
func (l *listener) Close() error              { return nil }
func (l *listener) Addr() net.Addr            { return nil }

type pipeAddr struct {
	remoteAddr string
}

func (pipeAddr) Network() string   { return "pipe" }
func (a *pipeAddr) String() string { return a.remoteAddr }

// 建立客户端和服务端通信的通道
func newPipes(req *http.Request) (net.Conn, *connBuf) {
	client, srv := net.Pipe()
	server := newConnBuf(srv, req)
	return client, server
}

// add Peek method for conn
type connBuf struct {
	net.Conn
	r          *bufio.Reader
	host       string
	remoteAddr string
}

func newConnBuf(c net.Conn, req *http.Request) *connBuf {
	return &connBuf{
		Conn:       c,
		r:          bufio.NewReader(c),
		host:       req.Host,
		remoteAddr: req.RemoteAddr,
	}
}

func (b *connBuf) Peek(n int) ([]byte, error) {
	return b.r.Peek(n)
}

func (b *connBuf) Read(data []byte) (int, error) {
	return b.r.Read(data)
}

func (b *connBuf) RemoteAddr() net.Addr {
	return &pipeAddr{remoteAddr: b.remoteAddr}
}

// Middle: man-in-the-middle
type Middle struct {
	Proxy    *Proxy
	CA       *cert.CA
	Listener net.Listener
	Server   *http.Server
}

func NewMiddle(proxy *Proxy,caPath string) (Interceptor, error) {
	ca, err := cert.NewCA(caPath)
	if err != nil {
		return nil, err
	}

	m := &Middle{
		Proxy: proxy,
		CA:    ca,
	}

	server := &http.Server{
		Handler:      m,
		IdleTimeout:  5 * time.Second,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // disable http2
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Debugf("Middle GetCertificate ServerName: %v\n", chi.ServerName)
				return ca.GetCert(chi.ServerName)
			},
		},
	}

	m.Server = server
	m.Listener = &listener{make(chan net.Conn)}

	return m, nil
}

func (m *Middle) Start() error {
	return m.Server.ServeTLS(m.Listener, "", "")
}

func (m *Middle) Dial(req *http.Request) (net.Conn, error) {
	clientConn, serverConn := newPipes(req)
	go m.intercept(serverConn)
	return clientConn, nil
}

func (m *Middle) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if strings.EqualFold(req.Header.Get("Connection"), "Upgrade") && strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		// wss
		DefaultWebSocket.WSS(res, req)
		return
	}

	if req.URL.Scheme == "" {
		req.URL.Scheme = "https"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	m.Proxy.ServeHTTP(res, req)
}

// 解析 connect 流量
// 如果是 tls 流量，则进入 listener.Accept => Middle.ServeHTTP
// 否则很可能是 ws 流量
func (m *Middle) intercept(serverConn *connBuf) {
	log := log.WithField("in", "Middle.intercept").WithField("host", serverConn.host)

	buf, err := serverConn.Peek(3)
	if err != nil {
		log.Errorf("Peek error: %v\n", err)
		serverConn.Close()
		return
	}

	if buf[0] == 0x16 && buf[1] == 0x03 && (buf[2] >= 0x0 || buf[2] <= 0x03) {
		// tls
		m.Listener.(*listener).connChan <- serverConn
	} else {
		// ws
		DefaultWebSocket.WS(serverConn, serverConn.host)
	}
}
