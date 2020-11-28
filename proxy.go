package proxy

import (
	"io"
	"log"
	"net/http"
	"time"
)

type Proxy struct {
	Server *http.Server
}

func (proxy *Proxy) Start() error {
	log.Printf("Proxy start listen at :8080")
	return proxy.Server.ListenAndServe()
}

func (proxy *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if !req.URL.IsAbs() || req.URL.Host == "" {
		res.WriteHeader(400)
		_, err := io.WriteString(res, "此为代理服务器，不能直接发起请求")
		if err != nil {
			log.Printf("error: %v", err)
		}
		return
	}

	start := time.Now()

	proxyReq, _ := http.NewRequest(req.Method, req.URL.String(), req.Body)

	// TODO: handle Proxy- header
	for key, value := range req.Header {
		proxyReq.Header[key] = value
	}
	proxyRes, _ := http.DefaultClient.Do(proxyReq)

	for key, value := range proxyRes.Header {
		res.Header()[key] = value
	}
	res.WriteHeader(proxyRes.StatusCode)
	_, err := io.Copy(res, proxyRes.Body)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	err = proxyRes.Body.Close()
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	log.Printf("%v %v %v - %v ms", req.Method, req.URL.String(), proxyRes.StatusCode, time.Since(start).Milliseconds())
}

func NewProxy() *Proxy {
	proxy := new(Proxy)
	proxy.Server = &http.Server{
		Addr:    ":8080",
		Handler: proxy,
	}
	return proxy
}
