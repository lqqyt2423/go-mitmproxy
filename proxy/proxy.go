package proxy

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/flow"
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
	Server            *http.Server
	Client            *http.Client
	Mitm              Mitm
	StreamLargeBodies int64
	Addons            []flow.Addon
}

func (proxy *Proxy) AddAddon(addon flow.Addon) {
	proxy.Addons = append(proxy.Addons, addon)
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

	endRes := func(response *flow.Response, body io.Reader) {
		if response.Header != nil {
			for key, value := range response.Header {
				for _, v := range value {
					res.Header().Add(key, v)
				}
			}
		}
		res.WriteHeader(response.StatusCode)

		if body != nil {
			_, err := io.Copy(res, body)
			if err != nil && !ignoreErr(log, err) {
				log.Error(err)
			}
		} else if response.Body != nil && len(response.Body) > 0 {
			_, err := res.Write(response.Body)
			if err != nil && !ignoreErr(log, err) {
				log.Error(err)
			}
		}
	}

	// when addons panic
	defer func() {
		if err := recover(); err != nil {
			log.Warnf("Recovered: %v\n", err)
		}
	}()

	flo := flow.NewFlow()
	flo.Request = &flow.Request{
		Method: req.Method,
		URL:    req.URL,
		Proto:  req.Proto,
		Header: req.Header,
	}
	defer flo.Finish()

	// trigger addon event Requestheaders
	for _, addon := range proxy.Addons {
		addon.Requestheaders(flo)
		if flo.Response != nil {
			endRes(flo.Response, nil)
			return
		}
	}

	// 读 request body
	var reqBody io.Reader = req.Body
	if !flo.Stream {
		reqBuf, r, err := ReaderToBuffer(req.Body, proxy.StreamLargeBodies)
		reqBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}
		if reqBuf == nil {
			log.Warnf("request body size >= %v\n", proxy.StreamLargeBodies)
			flo.Stream = true
		} else {
			flo.Request.Body = reqBuf
		}

		// trigger addon event Request
		if !flo.Stream {
			for _, addon := range proxy.Addons {
				addon.Request(flo)
				if flo.Response != nil {
					endRes(flo.Response, nil)
					return
				}
			}
			reqBody = bytes.NewReader(flo.Request.Body)
		}
	}

	proxyReq, err := http.NewRequest(flo.Request.Method, flo.Request.URL.String(), reqBody)
	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}

	for key, value := range flo.Request.Header {
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

	flo.Response = &flow.Response{
		StatusCode: proxyRes.StatusCode,
		Header:     proxyRes.Header,
	}

	// trigger addon event Responseheaders
	for _, addon := range proxy.Addons {
		addon.Responseheaders(flo)
		if flo.Response.Body != nil {
			endRes(flo.Response, nil)
			return
		}
	}

	// 读 response body
	var resBody io.Reader = proxyRes.Body
	if !flo.Stream {
		resBuf, r, err := ReaderToBuffer(proxyRes.Body, proxy.StreamLargeBodies)
		resBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}
		if resBuf == nil {
			log.Warnf("response body size >= %v\n", proxy.StreamLargeBodies)
			flo.Stream = true
		} else {
			flo.Response.Body = resBuf
		}

		// trigger addon event Response
		if !flo.Stream {
			for _, addon := range proxy.Addons {
				addon.Response(flo)
			}
		}
	}

	endRes(flo.Response, resBody)
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

	proxy.StreamLargeBodies = 1024 * 1024 * 5 // 5mb
	proxy.Addons = make([]flow.Addon, 0)
	proxy.AddAddon(&flow.LogAddon{})

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
