package proxy

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

type Options struct {
	Addr string
}

type Proxy struct {
	Server *http.Server

	ca               *cert.CA
	extraNetListener net.Listener
	extraServer      *http.Server
}

func (proxy *Proxy) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:") // port number is automatically chosen
	if err != nil {
		return err
	}
	proxy.extraNetListener = ln
	proxy.extraServer.Addr = ln.Addr().String()
	log.Printf("Proxy extraServer Addr is %v\n", proxy.extraServer.Addr)
	go func() {
		defer ln.Close()
		log.Fatal(proxy.extraServer.ServeTLS(ln, "", ""))
	}()

	log.Printf("Proxy start listen at %v\n", proxy.Server.Addr)
	return proxy.Server.ListenAndServe()
}

func (proxy *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method == "CONNECT" {
		proxy.handleConnect(res, req)
		return
	}

	log.Printf("url: %v\n", req.URL.String())

	if !req.URL.IsAbs() || req.URL.Host == "" {
		res.WriteHeader(400)
		_, err := io.WriteString(res, "此为代理服务器，不能直接发起请求")
		if err != nil {
			log.Printf("error: %v, url: %v\n", err, req.URL.String())
		}
		return
	}

	start := time.Now()

	proxyReq, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		log.Printf("error: %v, url: %v\n", err, req.URL.String())
		res.WriteHeader(502)
		return
	}

	// TODO: handle Proxy- header
	for key, value := range req.Header {
		proxyReq.Header[key] = value
	}
	proxyRes, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		log.Printf("error: %v, url: %v\n", err, req.URL.String())
		res.WriteHeader(502)
		return
	}
	defer proxyRes.Body.Close()

	for key, value := range proxyRes.Header {
		res.Header()[key] = value
	}
	res.WriteHeader(proxyRes.StatusCode)
	_, err = io.Copy(res, proxyRes.Body)
	if err != nil {
		log.Printf("error: %v, url: %v\n", err, req.URL.String())
		return
	}

	log.Printf("%v %v %v - %v ms", req.Method, req.URL.String(), proxyRes.StatusCode, time.Since(start).Milliseconds())
}

func (proxy *Proxy) handleConnect(res http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT: %v\n", req.Host)

	// 直接转发
	// conn, err := net.Dial("tcp", req.Host)

	// 内部解析 HTTPS
	conn, err := net.Dial("tcp", proxy.extraServer.Addr)

	if err != nil {
		log.Printf("error: %v, host: %v\n", err, req.Host)
		res.WriteHeader(502)
		return
	}
	defer conn.Close()

	cconn, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("error: %v, host: %v\n", err, req.Host)
		res.WriteHeader(502)
		return
	}
	defer cconn.Close()

	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		log.Printf("error: %v, host: %v\n", err, req.Host)
		return
	}

	ch := make(chan bool)
	go func() {
		_, err := io.Copy(conn, cconn)
		if err != nil {
			log.Printf("error: %v, host: %v\n", err, req.Host)
		}
		ch <- true
	}()

	_, err = io.Copy(cconn, conn)
	if err != nil {
		log.Printf("error: %v, host: %v\n", err, req.Host)
	}

	<-ch
}

func NewProxy(opts *Options) *Proxy {
	proxy := new(Proxy)
	proxy.Server = &http.Server{
		Addr:    opts.Addr,
		Handler: proxy,
	}

	ca, err := cert.NewCA("")
	if err != nil {
		panic(err)
	}
	proxy.ca = ca
	proxy.extraServer = &http.Server{
		Handler: proxy,
		TLSConfig: &tls.Config{
			PreferServerCipherSuites: true,
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Printf("GetCertificate ServerName: %v\n", chi.ServerName)
				return proxy.ca.DummyCert(chi.ServerName)
			},
		},
	}

	return proxy
}
