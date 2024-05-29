package main

import (
	"net"
	"net/http"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

func main() {
	opts := &proxy.Options{
		Addr:              ":8081",
		StreamLargeBodies: 1024 * 1024 * 5,
		NewCaFunc:         NewTrustedCA, // use custom cert
	}
	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}
	p.SetShouldInterceptRule(func(req *http.Request) bool {
		host, _, err2 := net.SplitHostPort(req.URL.Host)
		if err2 != nil {
			return false
		}
		return host == "your-domain.xx.com" || host == "your-domain2.xx.com" // filter your-domain
	})
	p.AddAddon(&YourAddOn{})
	log.Fatal(p.Start())
}

type YourAddOn struct {
	proxy.BaseAddon
}

func (m *YourAddOn) ClientConnected(client *proxy.ClientConn) {
	client.UpstreamCert = false // don't connect to upstream server
}

func (m *YourAddOn) Request(flow *proxy.Flow) {
	flow.Done()
	resp := &proxy.Response{
		StatusCode: 200,
		Header:     nil,
		Body:       []byte("changed response"),
		BodyReader: nil,
	}
	flow.Response = resp
}
