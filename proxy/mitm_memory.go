package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
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

	readErrChan chan error // Read 方法提前返回时的错误
}

func newConn(end *mock_conn.End) *conn {
	return &conn{
		End:         end,
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
	done := make(chan bool)
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
	log.Warnf("SetDeadline %v\n", t)
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
	log.Warnf("SetWriteDeadline %v\n", t)
	return nil
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
	pipes := mock_conn.NewConn()
	m.Listener.(*listener).connChan <- newConn(pipes.Server)
	return newConn(pipes.Client), nil
}

func (m *MitmMemory) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = "https"
	}

	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	m.Proxy.ServeHTTP(res, req)
}
