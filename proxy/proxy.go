package proxy

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	Debug             int
	Addr              string
	StreamLargeBodies int64 // 当请求或响应体大于此字节时，转为 stream 模式
	SslInsecure       bool
	CaRootPath        string
}

type Proxy struct {
	Opts    *Options
	Version string
	Addons  []Addon

	server      *http.Server
	interceptor interceptor
}

func NewProxy(opts *Options) (*Proxy, error) {
	if opts.StreamLargeBodies <= 0 {
		opts.StreamLargeBodies = 1024 * 1024 * 5 // default: 5mb
	}

	proxy := &Proxy{
		Opts:    opts,
		Version: "1.0.0",
		Addons:  make([]Addon, 0),
	}

	proxy.server = &http.Server{
		Addr:    opts.Addr,
		Handler: proxy,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			connCtx := newConnContext(c, proxy)
			for _, addon := range proxy.Addons {
				addon.ClientConnected(connCtx.ClientConn)
			}
			c.(*wrapClientConn).connCtx = connCtx
			return context.WithValue(ctx, connContextKey, connCtx)
		},
	}

	interceptor, err := newMiddle(proxy)
	if err != nil {
		return nil, err
	}
	proxy.interceptor = interceptor

	return proxy, nil
}

func (proxy *Proxy) AddAddon(addon Addon) {
	proxy.Addons = append(proxy.Addons, addon)
}

func (proxy *Proxy) Start() error {
	errChan := make(chan error)

	go func() {
		log.Infof("Proxy start listen at %v\n", proxy.server.Addr)
		addr := proxy.server.Addr
		if addr == "" {
			addr = ":http"
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errChan <- err
			return
		}
		pln := &wrapListener{
			Listener: ln,
			proxy:    proxy,
		}
		err = proxy.server.Serve(pln)
		errChan <- err
	}()

	go func() {
		err := proxy.interceptor.Start()
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

	log := log.WithFields(log.Fields{
		"in":     "Proxy.ServeHTTP",
		"url":    req.URL,
		"method": req.Method,
	})

	if !req.URL.IsAbs() || req.URL.Host == "" {
		res.WriteHeader(400)
		_, err := io.WriteString(res, "此为代理服务器，不能直接发起请求")
		if err != nil {
			log.Error(err)
		}
		return
	}

	reply := func(response *Response, body io.Reader, streamFunc StreamFunc) {
		var f StreamFunc
		if streamFunc != nil {
			f = streamFunc
		} else {
			f = io.Copy
		}
		if response.Header != nil {
			for key, value := range response.Header {
				for _, v := range value {
					res.Header().Add(key, v)
				}
			}
		}
		res.WriteHeader(response.StatusCode)

		if body != nil {
			_, err := f(res, body)
			if err != nil {
				logErr(log, err)
			}
		} else if response.Body != nil && len(response.Body) > 0 {
			_, err := res.Write(response.Body)
			if err != nil {
				logErr(log, err)
			}
		}
	}

	// when addons panic
	defer func() {
		if err := recover(); err != nil {
			log.Warnf("Recovered: %v\n", err)
		}
	}()

	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = req.Context().Value(connContextKey).(*ConnContext)
	defer f.finish()

	// trigger addon event Requestheaders
	for _, addon := range proxy.Addons {
		addon.Requestheaders(f)
		if f.Response != nil {
			reply(f.Response, nil, nil)
			return
		}
	}

	// Read request body
	var reqBody io.Reader = req.Body
	if !f.Stream {
		reqBuf, r, err := readerToBuffer(req.Body, proxy.Opts.StreamLargeBodies)
		reqBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}

		if reqBuf == nil {
			log.Warnf("request body size >= %v\n", proxy.Opts.StreamLargeBodies)
			f.Stream = true
		} else {
			f.Request.Body = reqBuf

			// trigger addon event Request
			for _, addon := range proxy.Addons {
				addon.Request(f)
				if f.Response != nil {
					reply(f.Response, nil, nil)
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

	f.ConnContext.initHttpServerConn()
	proxyRes, err := f.ConnContext.ServerConn.client.Do(proxyReq)
	if err != nil {
		logErr(log, err)
		res.WriteHeader(502)
		return
	}
	defer proxyRes.Body.Close()

	f.Response = &Response{
		StatusCode: proxyRes.StatusCode,
		Header:     proxyRes.Header,
	}

	// trigger addon event Responseheaders
	for _, addon := range proxy.Addons {
		addon.Responseheaders(f)
		if f.Response.Body != nil {
			reply(f.Response, nil, nil)
			return
		}
	}

	// Read response body
	var resBody io.Reader = proxyRes.Body
	if !f.Stream {
		resBuf, r, err := readerToBuffer(proxyRes.Body, proxy.Opts.StreamLargeBodies)
		resBody = r
		if err != nil {
			log.Error(err)
			res.WriteHeader(502)
			return
		}
		if resBuf == nil {
			log.Warnf("response body size >= %v\n", proxy.Opts.StreamLargeBodies)
			f.Stream = true
		} else {
			f.Response.Body = resBuf

			// trigger addon event Response
			for _, addon := range proxy.Addons {
				addon.Response(f)
			}
		}
	}

	reply(f.Response, resBody, f.StreamFunc)
}

func (proxy *Proxy) handleConnect(res http.ResponseWriter, req *http.Request) {
	log := log.WithFields(log.Fields{
		"in":   "Proxy.handleConnect",
		"host": req.Host,
	})

	conn, err := proxy.interceptor.Dial(req)
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

	// cconn.(*net.TCPConn).SetLinger(0) // send RST other than FIN when finished, to avoid TIME_WAIT state
	// cconn.(*net.TCPConn).SetKeepAlive(false)
	defer cconn.Close()

	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		log.Error(err)
		return
	}

	transfer(log, conn, cconn)
}
