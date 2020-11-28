package main

import (
	"log"

	proxy "github.com/lqqyt2423/go-mitmproxy"
)

func main() {
	opts := &proxy.Options{
		Addr: ":8080",
	}
	log.Fatal(proxy.NewProxy(opts).Start())
}
