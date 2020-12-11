package proxy

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	mock_conn "github.com/jordwest/mock-conn"
	"github.com/lqqyt2423/go-mitmproxy/cert"
)

// 模拟实现 net

type listener struct {
	connChan chan net.Conn
}

func (l *listener) Accept() (net.Conn, error) {
	return <-l.connChan, nil
}

func (l *listener) Close() error {
	return nil
}

func (l *listener) Addr() net.Addr {
	return nil
}

type ioRes struct {
	n   int
	err error
}

type conn struct {
	*mock_conn.End

	Host        string     // remote host
	readErrChan chan error // Read 方法提前返回时的错误
}

func newConn(end *mock_conn.End, host string) *conn {
	return &conn{
		End:         end,
		Host:        host,
		readErrChan: make(chan error),
	}
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
		log.WithField("host", c.Host).Warnf("SetDeadline %v\n", t)
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
			log.Warnf("SetReadDeadline %v\n", t)
		}
	}

	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	log.WithField("host", c.Host).Warnf("SetWriteDeadline %v\n", t)
	return nil
}

// wrap conn for peek
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

type MitmMemory struct {
	Proxy    *Proxy
	CA       *cert.CA
	Listener net.Listener
	Server   *http.Server
}

func NewMitmMemory(proxy *Proxy) (Mitm, error) {
	ca, err := cert.NewCA("")
	if err != nil {
		return nil, err
	}

	m := &MitmMemory{
		Proxy: proxy,
		CA:    ca,
	}

	server := &http.Server{
		Handler:      m,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // disable http2
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Debugf("MitmMemory GetCertificate ServerName: %v\n", chi.ServerName)
				return ca.GetCert(chi.ServerName)
			},
		},
	}

	// 每次连接尽快结束，因为连接并无开销
	server.SetKeepAlivesEnabled(false)

	m.Server = server

	return m, nil
}

func (m *MitmMemory) Start() error {
	ln := &listener{
		connChan: make(chan net.Conn),
	}
	m.Listener = ln
	return m.Server.ServeTLS(ln, "", "")
}

func (m *MitmMemory) Dial(host string) (net.Conn, error) {
	log := log.WithField("in", "MitmMemory.Dial").WithField("host", host)
	pipes := mock_conn.NewConn()

	// 如果是 tls 流量，则进入 listener.Accept => MitmMemory.ServeHTTP
	// 否则很可能是 ws 流量，直接转发流量
	go func() {
		conn := newConn(pipes.Server, host)
		connb := newConnBuf(conn)
		buf, err := connb.Peek(3)
		if err != nil {
			log.Errorf("Peek error: %v\n", err)
			connb.Close()
			return
		}

		// tls
		if buf[0] == 0x16 && buf[1] == 0x03 && (buf[2] >= 0x0 || buf[2] <= 0x03) {
			m.Listener.(*listener).connChan <- connb
		} else {
			// websocket ws://
			log.Debug("begin websocket ws://")
			defer connb.Close()
			remoteConn, err := net.Dial("tcp", host)
			if err != nil {
				if !ignoreErr(log, err) {
					log.Error(err)
				}
				return
			}
			defer remoteConn.Close()
			transfer(log, connb, remoteConn)
		}
	}()

	return newConn(pipes.Client, host), nil
}

func (m *MitmMemory) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	log := log.WithField("in", "MitmMemory.ServeHTTP").WithField("host", req.Host)

	// websocket wss://
	if strings.EqualFold(req.Header.Get("Connection"), "Upgrade") && strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		log.Debug("begin websocket wss://")

		upgradeBuf, err := httputil.DumpRequest(req, false)
		if err != nil {
			log.Errorf("DumpRequest: %v\n", err)
			res.WriteHeader(502)
			return
		}

		cconn, _, err := res.(http.Hijacker).Hijack()
		if err != nil {
			log.Errorf("Hijack: %v\n", err)
			res.WriteHeader(502)
			return
		}
		defer cconn.Close()

		host := req.Host
		if !strings.Contains(host, ":") {
			host = host + ":443"
		}
		conn, err := tls.Dial("tcp", host, nil)
		if err != nil {
			log.Errorf("tls.Dial: %v\n", err)
			return
		}
		defer conn.Close()

		_, err = conn.Write(upgradeBuf)
		if err != nil {
			log.Errorf("wss upgrade: %v\n", err)
			return
		}
		transfer(log, conn, cconn)
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
