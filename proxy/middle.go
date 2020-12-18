package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
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

// mock net.Conn
type conn struct {
	mock_conn.End
	host string // remote host

	// 以下为实现 SetReadDeadline 所需字段：需要确保 Read 方法可以提前返回
	// connection: keep-alive 相关
	readCanCancel bool        // 是否可取消 Read
	firstRead     bool        // 首次调用 Read 初始化
	pendingRead   bool        // 当前是否有 Read 操作在阻塞中
	readErrChan   chan error  // Read 方法提前返回时的错误，总是 os.ErrDeadlineExceeded
	readErr       error       // 底层 End 返回的错误
	readDeadline  time.Time   // SetReadDeadline 设置的时间
	chunk         chan []byte // Read 和 beginRead 的交互 channel
}

var connUnexpected = errors.New("unexpected read error")

// 建立客户端和服务端通信的通道
func newPipes(host string) (client *conn, server *connBuf) {
	pipes := mock_conn.NewConn()
	client = &conn{
		End:  *pipes.Client,
		host: host,
	}
	serverConn := &conn{
		End:           *pipes.Server,
		host:          host,
		readCanCancel: true,
		readErrChan:   make(chan error),
		chunk:         make(chan []byte),
	}
	server = newConnBuf(serverConn)
	return client, server
}

func (c *conn) beginRead(size int) {
	buf := make([]byte, size)
	for {
		n, err := c.End.Read(buf)
		if err != nil {
			c.readErr = err
			close(c.chunk)
			return
		}
		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		c.chunk <- chunk
	}
}

func (c *conn) Read(data []byte) (int, error) {
	if !c.readCanCancel {
		return c.End.Read(data)
	}

	if !c.firstRead {
		go c.beginRead(len(data))
	}
	c.firstRead = true

	if !c.readDeadline.Equal(time.Time{}) {
		if !c.readDeadline.After(time.Now()) {
			return 0, os.ErrDeadlineExceeded
		} else {
			log.WithField("host", c.host).Warnf("c.readDeadline is future %v\n", c.readDeadline)
			return 0, connUnexpected
		}
	}

	c.pendingRead = true
	defer func() {
		c.pendingRead = false
	}()

	select {
	case err := <-c.readErrChan:
		return 0, err
	case chunk, ok := <-c.chunk:
		if !ok {
			return 0, c.readErr
		}
		copy(data, chunk)
		return len(chunk), nil
	}
}

func (c *conn) SetDeadline(t time.Time) error {
	log.WithField("host", c.host).Warnf("SetDeadline %v\n", t)
	return connUnexpected
}

// http server 标准库实现时，当多个 http 复用底层 socke 时，会调用此方法
func (c *conn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	if c.pendingRead && !t.Equal(time.Time{}) && !t.After(time.Now()) {
		c.readErrChan <- os.ErrDeadlineExceeded
	}
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	log.WithField("host", c.host).Warnf("SetWriteDeadline %v\n", t)
	return connUnexpected
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
