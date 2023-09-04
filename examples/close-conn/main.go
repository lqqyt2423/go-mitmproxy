package main

import (
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type CloseConn struct {
	proxy.BaseAddon
}

func (a *CloseConn) ClientConnected(client *proxy.ClientConn) {
	// necessary
	client.UpstreamCert = false
}

func (a *CloseConn) Requestheaders(f *proxy.Flow) {
	// give some response to client
	// then will not request remote server
	f.Response = &proxy.Response{
		StatusCode: 502,
	}
}

func main() {
	opts := &proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	p.AddAddon(&CloseConn{})
	p.AddAddon(&proxy.LogAddon{})

	log.Fatal(p.Start())
}
