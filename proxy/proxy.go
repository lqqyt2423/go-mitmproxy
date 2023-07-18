package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	Debug             int
	Addr              string
	StreamLargeBodies int64 // 当请求或响应体大于此字节时，转为 stream 模式
	SslInsecure       bool
	CaRootPath        string
	Upstream          string
}

type Proxy struct {
	Opts    *Options
	Version string
	Addons  []Addon

	client              *http.Client
	server              *http.Server
	interceptor         *middle
	shouldIntercept     func(address string) bool
	dynamicUpstreamFunc func(*http.Request) (*url.URL, error)
}

// dynamicUpstreamFunc use case:
/*
var dynamicUpstreamFunc = func(req *http.Request) (*url.URL, error) {
	realip := GetRealClientIP(req)
	rip := net.ParseIP(realip)
	_, net, err := net.ParseCIDR("192.168.1.0/24")
	defaultf := http.ProxyFromEnvironment
	if err != nil {
		return defaultf(req)
	}

	proxy1, _ := url.Parse("http://upstream1.com")
	proxy2, _ := url.Parse("http://upstream2.com")

	if strings.Contains(req.URL.String(), "somechar") {
		return http.ProxyURL(proxy1)(req)
	}
	if net.Contains(rip) {
		return http.ProxyURL(proxy2)(req)
	} else {
		return defaultf(req)
	}
}
*/

func (proxy *Proxy) SetUpstreamProxy(dynamicUpstreamFunc func(*http.Request) (*url.URL, error)) {
	proxy.dynamicUpstreamFunc = dynamicUpstreamFunc
}

func NewProxy(opts *Options) (*Proxy, error) {
	if opts.StreamLargeBodies <= 0 {
		opts.StreamLargeBodies = 1024 * 1024 * 5 // default: 5mb
	}

	proxy := &Proxy{
		Opts:    opts,
		Version: "1.6.1",
		Addons:  make([]Addon, 0),
	}

	proxy.client = &http.Client{
		Transport: &http.Transport{
			Proxy:              clientProxy(opts.Upstream, proxy.dynamicUpstreamFunc),
			ForceAttemptHTTP2:  false, // disable http2
			DisableCompression: true,  // To get the original response from the server, set Transport.DisableCompression to true.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.SslInsecure,
				KeyLogWriter:       getTlsKeyLogWriter(),
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 禁止自动重定向
			return http.ErrUseLastResponse
		},
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
	addr := proxy.server.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go proxy.interceptor.start()

	log.Infof("Proxy start listen at %v\n", proxy.server.Addr)
	pln := &wrapListener{
		Listener: ln,
		proxy:    proxy,
	}
	return proxy.server.Serve(pln)
}

func (proxy *Proxy) Close() error {
	err := proxy.server.Close()
	proxy.interceptor.close()
	return err
}

func (proxy *Proxy) Shutdown(ctx context.Context) error {
	err := proxy.server.Shutdown(ctx)
	proxy.interceptor.close()
	return err
}

func (proxy *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// 记录下来真实的客户端IP，后续步骤可能需要用到，这儿需要的原因是因为初始化connection的时候，需要用到
	req = SetRealClientIP(req, req.RemoteAddr)
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
		if len(proxy.Addons) == 0 {
			res.WriteHeader(400)
			io.WriteString(res, "此为代理服务器，不能直接发起请求")
			return
		}
		for _, addon := range proxy.Addons {
			addon.AccessProxyServer(req, res)
		}
		return
	}

	reply := func(response *Response, body io.Reader) {
		if response.Header != nil {
			for key, value := range response.Header {
				for _, v := range value {
					res.Header().Add(key, v)
				}
			}
		}
		if response.close {
			res.Header().Add("Connection", "close")
		}
		res.WriteHeader(response.StatusCode)

		if body != nil {
			_, err := io.Copy(res, body)
			if err != nil {
				logErr(log, err)
			}
		}
		if response.BodyReader != nil {
			_, err := io.Copy(res, response.BodyReader)
			if err != nil {
				logErr(log, err)
			}
		}
		if response.Body != nil && len(response.Body) > 0 {
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

	f.ConnContext.FlowCount = f.ConnContext.FlowCount + 1

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
					reply(f.Response, nil)
					return
				}
			}
			reqBody = bytes.NewReader(f.Request.Body)
		}
	}

	for _, addon := range proxy.Addons {
		reqBody = addon.StreamRequestModifier(f, reqBody)
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
	// 记录下来真实的客户端IP，后续步骤可能需要用到
	proxyReq = SetRealClientIP(proxyReq, req.RemoteAddr)

	f.ConnContext.initHttpServerConn()
	var proxyRes *http.Response
	if f.UseSeparateClient {
		proxyRes, err = proxy.client.Do(proxyReq)
	} else {
		proxyRes, err = f.ConnContext.ServerConn.client.Do(proxyReq)
	}
	if err != nil {
		logErr(log, err)
		res.WriteHeader(502)
		return
	}

	if proxyRes.Close {
		f.ConnContext.closeAfterResponse = true
	}

	defer proxyRes.Body.Close()

	f.Response = &Response{
		StatusCode: proxyRes.StatusCode,
		Header:     proxyRes.Header,
		close:      proxyRes.Close,
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
	for _, addon := range proxy.Addons {
		resBody = addon.StreamResponseModifier(f, resBody)
	}

	reply(f.Response, resBody)
}

func (proxy *Proxy) handleConnect(res http.ResponseWriter, req *http.Request) {
	log := log.WithFields(log.Fields{
		"in":   "Proxy.handleConnect",
		"host": req.Host,
	})

	shouldIntercept := proxy.shouldIntercept == nil || proxy.shouldIntercept(req.Host)
	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = req.Context().Value(connContextKey).(*ConnContext)
	f.ConnContext.Intercept = shouldIntercept
	defer f.finish()

	// trigger addon event Requestheaders
	for _, addon := range proxy.Addons {
		addon.Requestheaders(f)
	}

	var conn net.Conn
	var err error
	if shouldIntercept {
		log.Debugf("begin intercept %v", req.Host)
		conn, err = proxy.interceptor.dial(req)
	} else {
		log.Debugf("begin transpond %v", req.Host)
		conn, err = getConnFrom(req, proxy.Opts.Upstream, proxy.dynamicUpstreamFunc)
	}
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

	f.Response = &Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}

	// trigger addon event Responseheaders
	for _, addon := range proxy.Addons {
		addon.Responseheaders(f)
	}
	defer func(f *Flow) {
		// trigger addon event Response
		for _, addon := range proxy.Addons {
			addon.Response(f)
		}
	}(f)

	transfer(log, conn, cconn)
}

func (proxy *Proxy) GetCertificate() x509.Certificate {
	return proxy.interceptor.ca.RootCert
}

func (proxy *Proxy) SetShouldInterceptRule(rule func(address string) bool) {
	proxy.shouldIntercept = rule
}
