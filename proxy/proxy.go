package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "proxy")

var ignoreErr = func(log *_log.Entry, err error) bool {
	errs := err.Error()
	strs := []string{
		"read: connection reset by peer",
		"write: broken pipe",
		"i/o timeout",
		"net/http: TLS handshake timeout",
		"io: read/write on closed pipe",
		"connect: connection refused",
		"connect: connection reset by peer",
	}

	for _, str := range strs {
		if strings.Contains(errs, str) {
			log.Debug(err)
			return true
		}
	}

	return false
}

func transfer(log *_log.Entry, a, b io.ReadWriter) {
	done := make(chan struct{})
	defer close(done)

	forward := func(dst io.Writer, src io.Reader, ec chan<- error) {
		_, err := io.Copy(dst, src)

		if v, ok := dst.(*conn); ok {
			// 避免内存泄漏的关键
			_ = v.Writer.CloseWithError(nil)
		}

		select {
		case <-done:
			return
		case ec <- err:
		}
	}

	errChan := make(chan error)
	go forward(a, b, errChan)
	go forward(b, a, errChan)

	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			if !ignoreErr(log, err) {
				log.Error(err)
			}
			return // 如果有错误，直接返回
		}
	}
}

type Options struct {
	Addr string
}

type Proxy struct {
	Server *http.Server
	Client *http.Client
	Mitm   Mitm
}

func (proxy *Proxy) Start() error {
	errChan := make(chan error)

	go func() {
		log.Infof("Proxy start listen at %v\n", proxy.Server.Addr)
		err := proxy.Server.ListenAndServe()
		errChan <- err
	}()

	go func() {
		err := proxy.Mitm.Start()
		errChan <- err
	}()

	err := <-errChan
	return err
}

func (proxy *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method == "CONNECT" {
		proxy.handleConnect(res, req)
		return
	}

	log := log.WithFields(_log.Fields{
		"in":     "Proxy.ServeHTTP",
		"url":    req.URL,
		"method": req.Method,
	})

	log.Debug("receive request")

	if !req.URL.IsAbs() || req.URL.Host == "" {
		res.WriteHeader(400)
		_, err := io.WriteString(res, "此为代理服务器，不能直接发起请求")
		if err != nil {
			log.Error(err)
		}
		return
	}

	start := time.Now()

	proxyReq, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}

	for key, value := range req.Header {
		for _, v := range value {
			proxyReq.Header.Add(key, v)
		}
	}
	proxyRes, err := proxy.Client.Do(proxyReq)
	if err != nil {
		if !ignoreErr(log, err) {
			log.Error(err)
		}
		res.WriteHeader(502)
		return
	}
	defer proxyRes.Body.Close()

	for key, value := range proxyRes.Header {
		for _, v := range value {
			res.Header().Add(key, v)
		}
	}
	res.WriteHeader(proxyRes.StatusCode)
	_, err = io.Copy(res, proxyRes.Body)
	if err != nil && !ignoreErr(log, err) {
		log.Error(err)
		return
	}

	log.Infof("status code: %v cost %v ms\n", proxyRes.StatusCode, time.Since(start).Milliseconds())
}

func (proxy *Proxy) handleConnect(res http.ResponseWriter, req *http.Request) {
	log := log.WithFields(_log.Fields{
		"in":   "Proxy.handleConnect",
		"host": req.Host,
	})

	log.Debug("receive connect")

	conn, err := proxy.Mitm.Dial(req.Host)

	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}
	defer conn.Close()

	cconn, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}
	defer cconn.Close()

	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		log.Error(err)
		return
	}

	transfer(log, conn, cconn)
}

func NewProxy(opts *Options) (*Proxy, error) {
	proxy := new(Proxy)
	proxy.Server = &http.Server{
		Addr:    opts.Addr,
		Handler: proxy,
	}

	proxy.Client = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,

			ForceAttemptHTTP2:  false, // disable http2
			DisableCompression: true,
			TLSClientConfig: &tls.Config{
				KeyLogWriter: GetTlsKeyLogWriter(),
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 禁止自动重定向
			return http.ErrUseLastResponse
		},
	}

	mitm, err := NewMitmMemory(proxy)
	if err != nil {
		return nil, err
	}

	proxy.Mitm = mitm

	return proxy, nil
}

var tlsKeyLogWriter io.Writer
var tlsKeyLogOnce sync.Once

// Wireshark 解析 https 设置
func GetTlsKeyLogWriter() io.Writer {
	tlsKeyLogOnce.Do(func() {
		logfile := os.Getenv("SSLKEYLOGFILE")
		if logfile == "" {
			return
		}

		writer, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.WithField("in", "GetTlsKeyLogWriter").Debug(err)
			return
		}

		tlsKeyLogWriter = writer
	})
	return tlsKeyLogWriter
}
