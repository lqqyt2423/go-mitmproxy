package proxy

import (
	"io"
	"log"
	"net/http"
	"time"
)

func Create() {
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		if !req.URL.IsAbs() || req.URL.Host == "" {
			res.WriteHeader(400)
			io.WriteString(res, "此为代理服务器，不能直接发起请求")
			return
		}

		start := time.Now()

		proxyReq, _ := http.NewRequest(req.Method, req.URL.String(), req.Body)

		// TODO: handle Proxy- header
		for key, value := range req.Header {
			proxyReq.Header[key] = value
		}
		proxyRes, _ := http.DefaultClient.Do(proxyReq)
		defer proxyRes.Body.Close()

		for key, value := range proxyRes.Header {
			res.Header()[key] = value
		}
		res.WriteHeader(proxyRes.StatusCode)
		io.Copy(res, proxyRes.Body)

		log.Printf("%v %v %v - %v ms", req.Method, req.URL.String(), proxyRes.StatusCode, time.Since(start))
	})
}

func Init() {
	log.Println("server begin listen at :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
