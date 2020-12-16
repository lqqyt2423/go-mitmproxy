package proxy

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	mock_conn "github.com/jordwest/mock-conn"
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

type ioRes struct {
	n   int
	err error
}

// mock net.Conn
type conn struct {
	mock_conn.End
	host        string     // remote host
	readErrChan chan error // Read 方法提前返回时的错误
}

// 建立客户端和服务端通信的通道
func newPipes(host string) (client *conn, server *connBuf) {
	pipes := mock_conn.NewConn()
	client = &conn{*pipes.Client, host, nil}
	serverConn := &conn{*pipes.Server, host, make(chan error)}
	server = newConnBuf(serverConn)
	return client, server
}

// 当接收到 readErrChan 时，可提前返回
func (c *conn) Read(data []byte) (int, error) {
	select {
	case err := <-c.readErrChan:
		return 0, err
	default:
	}

	resChan := make(chan *ioRes)
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done:
			return
		default:
		}

		n, err := c.End.Read(data)
		select {
		case resChan <- &ioRes{n, err}:
			return
		case <-done:
			close(resChan)
		}
	}()

	select {
	case res := <-resChan:
		return res.n, res.err
	case err := <-c.readErrChan:
		return 0, err
	}
}

func (c *conn) SetDeadline(t time.Time) error {
	if !t.Equal(time.Time{}) {
		log.WithField("host", c.host).Warnf("SetDeadline %v\n", t)
	}
	return nil
}

// http server 会在连接快结束时调用此方法
func (c *conn) SetReadDeadline(t time.Time) error {
	if !t.Equal(time.Time{}) {
		if !t.After(time.Now()) {
			// 使当前 Read 尽快返回
			c.readErrChan <- os.ErrDeadlineExceeded
		} else {
			log.WithField("host", c.host).Warnf("SetReadDeadline %v\n", t)
		}
	}

	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	log.WithField("host", c.host).Warnf("SetWriteDeadline %v\n", t)
	return nil
}

// add Peek method for conn
type connBuf struct {
	*conn
	r *bufio.Reader
}

func newConnBuf(c *conn) *connBuf {
	return &connBuf{c, bufio.NewReader(c)}
}

func (b *connBuf) Peek(n int) ([]byte, error) {
	return b.r.Peek(n)
}

func (b *connBuf) Read(data []byte) (int, error) {
	return b.r.Read(data)
}

// Middle: man-in-the-middle
type Middle struct {
	Proxy    *Proxy
	CA       *cert.CA
	Listener net.Listener
	Server   *http.Server
}

func NewMiddle(proxy *Proxy) (Interceptor, error) {
	ca, err := cert.NewCA("")
	if err != nil {
		return nil, err
	}

	m := &Middle{
		Proxy: proxy,
		CA:    ca,
	}

	server := &http.Server{
		Handler:      m,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // disable http2
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Debugf("Middle GetCertificate ServerName: %v\n", chi.ServerName)
				return ca.GetCert(chi.ServerName)
			},
		},
	}

	// 每次连接尽快结束，因为连接并无开销
	server.SetKeepAlivesEnabled(false)

	m.Server = server

	return m, nil
}

func (m *Middle) Start() error {
	m.Listener = &listener{make(chan net.Conn)}
	return m.Server.ServeTLS(m.Listener, "", "")
}

func (m *Middle) Dial(host string) (net.Conn, error) {
	clientConn, serverConn := newPipes(host)
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
