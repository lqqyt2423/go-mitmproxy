package proxy

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/flow"
	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "proxy")

type Options struct {
	Addr              string
	StreamLargeBodies int64
	SslInsecure       bool
	CaRootPath        string
}

type Proxy struct {
	Version           string
	Server            *http.Server
	Client            *http.Client
	Interceptor       Interceptor
	StreamLargeBodies int64 // 当请求或响应体大于此字节时，转为 stream 模式
	Addons            []addon.Addon
}

func NewProxy(opts *Options) (*Proxy, error) {
	proxy := new(Proxy)
	proxy.Version = "0.1.9"

	proxy.Server = &http.Server{
		Addr:        opts.Addr,
		Handler:     proxy,
		IdleTimeout: 5 * time.Second,
	}

	proxy.Client = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       5 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     false, // disable http2
			DisableCompression:    true,  // To get the original response from the server, set Transport.DisableCompression to true.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.SslInsecure,
				KeyLogWriter:       GetTlsKeyLogWriter(),
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 禁止自动重定向
			return http.ErrUseLastResponse
		},
	}

	interceptor, err := NewMiddle(proxy, opts.CaRootPath)
	if err != nil {
		return nil, err
	}
	proxy.Interceptor = interceptor

	if opts.StreamLargeBodies > 0 {
		proxy.StreamLargeBodies = opts.StreamLargeBodies
	} else {
		proxy.StreamLargeBodies = 1024 * 1024 * 5 // default: 5mb
	}

	proxy.Addons = make([]addon.Addon, 0)

	return proxy, nil
}

func (proxy *Proxy) AddAddon(addon addon.Addon) {
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
		err := proxy.Interceptor.Start()
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

	reply := func(response *flow.Response, body io.Reader) {
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
			if err != nil {
				LogErr(log, err)
			}
		} else if response.Body != nil && len(response.Body) > 0 {
			_, err := res.Write(response.Body)
			if err != nil {
				LogErr(log, err)
			}
		}
	}

	// when addons panic
	defer func() {
		if err := recover(); err != nil {
			log.Warnf("Recovered: %v\n", err)
		}
	}()

	f := flow.NewFlow()
	f.Request = flow.NewRequest(req)
	defer f.Finish()

	// trigger addon event Requestheaders
	for _, addon := range proxy.Addons {
		addon.Requestheaders(f)
		if f.Response != nil {
			reply(f.Response, nil)
			return
		}
	}

	// Read request body
	var reqBody io.Reader = req.Body
	if !f.Stream {
		reqBuf, r, err := ReaderToBuffer(req.Body, proxy.StreamLargeBodies)
		reqBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}

		if reqBuf == nil {
			log.Warnf("request body size >= %v\n", proxy.StreamLargeBodies)
			f.Stream = true
		} else {
			f.Request.Body = reqBuf

			// trigger addon event Request
			for _, addon := range proxy.Addons {
				addon.Request(f)
				if f.Response != nil {
					reply(f.Response, nil)
					return
				}
			}
			reqBody = bytes.NewReader(f.Request.Body)
		}
	}

	proxyReq, err := http.NewRequest(f.Request.Method, f.Request.URL.String(), reqBody)
	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}

	for key, value := range f.Request.Header {
		for _, v := range value {
			proxyReq.Header.Add(key, v)
		}
	}
	proxyRes, err := proxy.Client.Do(proxyReq)
	if err != nil {
		LogErr(log, err)
		res.WriteHeader(502)
		return
	}
	defer proxyRes.Body.Close()

	f.Response = &flow.Response{
		StatusCode: proxyRes.StatusCode,
		Header:     proxyRes.Header,
	}

	// trigger addon event Responseheaders
	for _, addon := range proxy.Addons {
		addon.Responseheaders(f)
		if f.Response.Body != nil {
			reply(f.Response, nil)
			return
		}
	}

	// Read response body
	var resBody io.Reader = proxyRes.Body
	if !f.Stream {
		resBuf, r, err := ReaderToBuffer(proxyRes.Body, proxy.StreamLargeBodies)
		resBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}
		if resBuf == nil {
			log.Warnf("response body size >= %v\n", proxy.StreamLargeBodies)
			f.Stream = true
		} else {
			f.Response.Body = resBuf

			// trigger addon event Response
			for _, addon := range proxy.Addons {
				addon.Response(f)
			}
		}
	}

	reply(f.Response, resBody)
}

func (proxy *Proxy) handleConnect(res http.ResponseWriter, req *http.Request) {
	log := log.WithFields(_log.Fields{
		"in":   "Proxy.handleConnect",
		"host": req.Host,
	})

	log.Debug("receive connect")

	conn, err := proxy.Interceptor.Dial(req)
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

	cconn.(*net.TCPConn).SetLinger(0) // send RST other than FIN when finished, to avoid TIME_WAIT state
	cconn.(*net.TCPConn).SetKeepAlive(false)
	defer cconn.Close()

	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		log.Error(err)
		return
	}

	Transfer(log, conn, cconn)
}
