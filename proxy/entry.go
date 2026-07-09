package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	log "github.com/sirupsen/logrus"
)

// wrap tcpListener for remote client
type wrapListener struct {
	net.Listener
	proxy *Proxy
}

func (l *wrapListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	proxy := l.proxy
	wc := newWrapClientConn(c, proxy)
	connCtx := newConnContext(wc, proxy)
	wc.connCtx = connCtx

	for _, addon := range proxy.Addons {
		addon.ClientConnected(connCtx.ClientConn)
	}

	return wc, nil
}

// wrap tcpConn for remote client
type wrapClientConn struct {
	net.Conn
	r       *bufio.Reader
	proxy   *Proxy
	connCtx *ConnContext

	closeMu   sync.Mutex
	closed    bool
	closeErr  error
	closeChan chan struct{}
}

func newWrapClientConn(c net.Conn, proxy *Proxy) *wrapClientConn {
	return &wrapClientConn{
		Conn:      c,
		r:         bufio.NewReader(c),
		proxy:     proxy,
		closeChan: make(chan struct{}),
	}
}

func (c *wrapClientConn) Peek(n int) ([]byte, error) {
	return c.r.Peek(n)
}

func (c *wrapClientConn) PeekBuffered() ([]byte, error) {
	return c.r.Peek(c.r.Buffered())
}

func (c *wrapClientConn) Read(data []byte) (int, error) {
	return c.r.Read(data)
}

func (c *wrapClientConn) Close() error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return c.closeErr
	}
	log.Debugln("in wrapClientConn close", c.connCtx.ClientConn.Conn.RemoteAddr())

	c.closed = true
	c.closeErr = c.Conn.Close()
	c.closeMu.Unlock()
	close(c.closeChan)

	for _, addon := range c.proxy.Addons {
		addon.ClientDisconnected(c.connCtx.ClientConn)
	}

	if c.connCtx.ServerConn != nil && c.connCtx.ServerConn.Conn != nil {
		c.connCtx.ServerConn.Conn.Close()
	}

	return c.closeErr
}

type peekableClientConn interface {
	net.Conn
	Peek(n int) ([]byte, error)
	PeekBuffered() ([]byte, error)
}

type hijackedClientConn struct {
	net.Conn
	r       *bufio.Reader
	connCtx *ConnContext
}

func newHijackedClientConn(c net.Conn, rw *bufio.ReadWriter, connCtx *ConnContext) *hijackedClientConn {
	return &hijackedClientConn{
		Conn:    c,
		r:       rw.Reader,
		connCtx: connCtx,
	}
}

func (c *hijackedClientConn) Peek(n int) ([]byte, error) {
	return c.r.Peek(n)
}

func (c *hijackedClientConn) PeekBuffered() ([]byte, error) {
	return c.r.Peek(c.r.Buffered())
}

func (c *hijackedClientConn) Read(data []byte) (int, error) {
	return c.r.Read(data)
}

func clientConnContext(c net.Conn) *ConnContext {
	switch conn := c.(type) {
	case *wrapClientConn:
		return conn.connCtx
	case *hijackedClientConn:
		return conn.connCtx
	default:
		return nil
	}
}

// wrap tcpConn for remote server
type wrapServerConn struct {
	net.Conn
	proxy   *Proxy
	connCtx *ConnContext

	closeMu  sync.Mutex
	closed   bool
	closeErr error
}

func (c *wrapServerConn) Close() error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return c.closeErr
	}
	log.Debugln("in wrapServerConn close", c.connCtx.ClientConn.Conn.RemoteAddr())

	c.closed = true
	c.closeErr = c.Conn.Close()
	c.closeMu.Unlock()

	for _, addon := range c.proxy.Addons {
		addon.ServerDisconnected(c.connCtx)
	}

	if !c.connCtx.ClientConn.Tls {
		c.connCtx.ClientConn.Conn.(*wrapClientConn).Conn.(*net.TCPConn).CloseRead()
	} else {
		// if keep-alive connection close
		if !c.connCtx.closeAfterResponse {
			c.connCtx.ClientConn.Conn.Close()
		}
	}

	return c.closeErr
}

type entry struct {
	proxy       *Proxy
	server      *http.Server
	httpsServer *http.Server
}

func newEntry(proxy *Proxy) *entry {
	e := &entry{proxy: proxy}
	e.server = &http.Server{
		Addr:    proxy.Opts.Addr,
		Handler: e,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return context.WithValue(ctx, connContextKey, connContextFromClientConn(c))
		},
	}
	if proxy.Opts.WithTLS {
		e.server.TLSConfig = &tls.Config{
			NextProtos: []string{"http/1.1"},
		}
	}
	if proxy.Opts.HTTPSAddr != "" {
		e.httpsServer = &http.Server{
			Addr:    proxy.Opts.HTTPSAddr,
			Handler: e,
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return context.WithValue(ctx, connContextKey, connContextFromClientConn(c))
			},
			TLSConfig: &tls.Config{
				NextProtos: []string{"http/1.1"},
			},
		}
	}
	return e
}

func connContextFromClientConn(c net.Conn) *ConnContext {
	if tlsConn, ok := c.(*tls.Conn); ok {
		c = tlsConn.NetConn()
	}
	return c.(*wrapClientConn).connCtx
}

func (e *entry) loadServerCert() (tls.Certificate, error) {
	if e.proxy.Opts.HTTPSCertFile == "" || e.proxy.Opts.HTTPSKeyFile == "" {
		return tls.Certificate{}, fmt.Errorf("https proxy requires both cert and key files")
	}
	return tls.LoadX509KeyPair(e.proxy.Opts.HTTPSCertFile, e.proxy.Opts.HTTPSKeyFile)
}

func (e *entry) validateHTTPSConfig() error {
	if !e.proxy.Opts.WithTLS && e.httpsServer == nil {
		return nil
	}
	cert, err := e.loadServerCert()
	if err != nil {
		return err
	}
	if e.proxy.Opts.WithTLS {
		e.server.TLSConfig.Certificates = []tls.Certificate{cert}
	}
	if e.httpsServer != nil {
		e.httpsServer.TLSConfig.Certificates = []tls.Certificate{cert}
	}
	return nil
}

func (e *entry) start() error {
	if e.httpsServer != nil {
		return e.startHTTPAndHTTPS()
	}
	return e.startServer(e.server, e.proxy.Opts.WithTLS)
}

func (e *entry) startHTTPAndHTTPS() error {
	errCh := make(chan error, 2)
	go func() {
		errCh <- e.startServer(e.server, e.proxy.Opts.WithTLS)
	}()
	go func() {
		errCh <- e.startServer(e.httpsServer, true)
	}()
	err := <-errCh
	if err != http.ErrServerClosed {
		e.close()
	}
	return err
}

func (e *entry) startServer(server *http.Server, isHTTPS bool) error {
	addr := server.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	switch {
	case server == e.server && isHTTPS:
		log.Infof("Proxy start listen at %v (over TLS)\n", server.Addr)
	case isHTTPS:
		log.Infof("HTTPS proxy start listen at %v\n", server.Addr)
	default:
		log.Infof("Proxy start listen at %v\n", server.Addr)
	}
	pln := &wrapListener{
		Listener: ln,
		proxy:    e.proxy,
	}
	if isHTTPS {
		return server.Serve(tls.NewListener(pln, server.TLSConfig))
	}
	return server.Serve(pln)
}

func (e *entry) close() error {
	if e.httpsServer == nil {
		return e.server.Close()
	}
	return errors.Join(e.server.Close(), e.httpsServer.Close())
}

func (e *entry) shutdown(ctx context.Context) error {
	if e.httpsServer == nil {
		return e.server.Shutdown(ctx)
	}
	return errors.Join(e.server.Shutdown(ctx), e.httpsServer.Shutdown(ctx))
}

func (e *entry) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	proxy := e.proxy

	log := log.WithFields(log.Fields{
		"in":   "Proxy.entry.ServeHTTP",
		"host": req.Host,
	})
	// Add entry proxy authentication
	if e.proxy.authProxy != nil {
		b, err := e.proxy.authProxy(res, req)
		if !b {
			log.Errorf("Proxy authentication failed: %s", err.Error())
			httpError(res, "", http.StatusProxyAuthRequired)
			return
		}
	}
	// proxy via connect tunnel
	if req.Method == "CONNECT" {
		e.handleConnect(res, req)
		return
	}

	if !req.URL.IsAbs() || req.URL.Host == "" {
		res = helper.NewResponseCheck(res)
		for _, addon := range proxy.Addons {
			addon.AccessProxyServer(req, res)
		}
		if res, ok := res.(*helper.ResponseCheck); ok {
			if !res.Wrote {
				res.WriteHeader(400)
				io.WriteString(res, "此为代理服务器，不能直接发起请求")
			}
		}
		return
	}

	// http proxy
	proxy.attacker.initHttpDialFn(req)
	proxy.attacker.attack(res, req)
}

func (e *entry) handleConnect(res http.ResponseWriter, req *http.Request) {
	proxy := e.proxy

	log := log.WithFields(log.Fields{
		"in":   "Proxy.entry.handleConnect",
		"host": req.Host,
	})

	shouldIntercept := proxy.shouldIntercept == nil || proxy.shouldIntercept(req)
	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = req.Context().Value(connContextKey).(*ConnContext)
	f.ConnContext.Intercept = shouldIntercept
	defer f.finish()

	// trigger addon event Requestheaders
	for _, addon := range proxy.Addons {
		addon.Requestheaders(f)
	}

	if !shouldIntercept {
		log.Debugf("begin transpond %v", req.Host)
		e.directTransfer(res, req, f)
		return
	}

	if f.ConnContext.ClientConn.UpstreamCert {
		e.httpsDialFirstAttack(res, req, f)
		return
	}

	log.Debugf("begin intercept %v", req.Host)
	e.httpsDialLazyAttack(res, req, f)
}

func (e *entry) establishConnection(res http.ResponseWriter, f *Flow) (net.Conn, error) {
	cconn, rw, err := res.(http.Hijacker).Hijack()
	if err != nil {
		for _, addon := range e.proxy.Addons {
			addon.HTTPConnectError(f, err)
		}
		res.WriteHeader(502)
		return nil, err
	}
	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		cconn.Close()
		for _, addon := range e.proxy.Addons {
			addon.HTTPConnectError(f, err)
		}
		return nil, err
	}

	f.Response = &Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}

	// trigger addon event Responseheaders
	for _, addon := range e.proxy.Addons {
		addon.Responseheaders(f)
	}

	return newHijackedClientConn(cconn, rw, f.ConnContext), nil
}

func (e *entry) directTransfer(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	log := log.WithFields(log.Fields{
		"in":   "Proxy.entry.directTransfer",
		"host": req.Host,
	})

	conn, err := proxy.getUpstreamConn(req.Context(), req)
	if err != nil {
		for _, addon := range proxy.Addons {
			addon.HTTPConnectError(f, err)
		}
		res.WriteHeader(502)
		return
	}
	defer conn.Close()

	cconn, err := e.establishConnection(res, f)
	if err != nil {
		return
	}
	defer cconn.Close()

	transfer(log, conn, cconn)
}

func (e *entry) httpsDialFirstAttack(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	log := log.WithFields(log.Fields{
		"in":   "Proxy.entry.httpsDialFirstAttack",
		"host": req.Host,
	})

	conn, err := proxy.attacker.httpsDial(req.Context(), req)
	if err != nil {
		for _, addon := range proxy.Addons {
			addon.HTTPConnectError(f, err)
		}
		res.WriteHeader(502)
		return
	}

	cconn, err := e.establishConnection(res, f)
	if err != nil {
		conn.Close()
		return
	}

	clientConn := cconn.(peekableClientConn)
	peek, err := clientConn.Peek(3)
	if err != nil {
		cconn.Close()
		conn.Close()
		log.Error(err)
		return
	}

	if helper.IsTls(peek) {
		f.ConnContext.ClientConn.Tls = true
		proxy.attacker.httpsTlsDial(req.Context(), cconn, conn)
		return
	}

	wsPeek, err := clientConn.PeekBuffered()
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		cconn.Close()
		conn.Close()
		log.Error(err)
		return
	}

	if helper.IsWebSocket(wsPeek) {
		err = proxy.webSocketHandler.handle(conn, cconn, f)
		if err != nil {
			log.Errorf("WebSocket handle error: %v", err)
			cconn.Close()
			conn.Close()
		}
		return
	}

	transfer(log, conn, cconn)
	cconn.Close()
	conn.Close()
}

func (e *entry) httpsDialLazyAttack(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	log := log.WithFields(log.Fields{
		"in":   "Proxy.entry.httpsDialLazyAttack",
		"host": req.Host,
	})

	cconn, err := e.establishConnection(res, f)
	if err != nil {
		log.Error(err)
		return
	}

	clientConn := cconn.(peekableClientConn)
	peek, err := clientConn.Peek(3)
	if err != nil {
		cconn.Close()
		log.Error(err)
		return
	}

	if helper.IsTls(peek) {
		f.ConnContext.ClientConn.Tls = true
		proxy.attacker.httpsLazyAttack(req.Context(), cconn, req)
		return
	}

	wsPeek, err := clientConn.PeekBuffered()
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		cconn.Close()
		log.Error(err)
		return
	}

	if helper.IsWebSocket(wsPeek) {
		conn, err := proxy.attacker.httpsDial(req.Context(), req)
		if err != nil {
			cconn.Close()
			log.Error(err)
			return
		}
		err = proxy.webSocketHandler.handle(conn, cconn, f)
		if err != nil {
			log.Errorf("WebSocket handle error: %v", err)
			cconn.Close()
			conn.Close()
		}
		return
	}

	conn, err := proxy.attacker.httpsDial(req.Context(), req)
	if err != nil {
		cconn.Close()
		log.Error(err)
		return
	}
	transfer(log, conn, cconn)
	conn.Close()
	cconn.Close()
}
