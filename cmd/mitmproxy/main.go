package main

import (
	"log"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	opts := &proxy.Options{
		Addr: ":9080",
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(p.Start())
}
