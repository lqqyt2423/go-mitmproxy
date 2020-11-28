package main

import (
	"log"

	proxy "github.com/lqqyt2423/go-mitmproxy"
)

func main() {
	log.Fatal(proxy.NewProxy().Start())
}
