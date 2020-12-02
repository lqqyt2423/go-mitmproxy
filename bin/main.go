package main

import (
	"log"

	proxy "github.com/lqqyt2423/go-mitmproxy"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	opts := &proxy.Options{
		Addr: ":8080",
	}
	log.Fatal(proxy.NewProxy(opts).Start())
}
