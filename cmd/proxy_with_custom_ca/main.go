package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	mproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

func main() {
	keyFile := os.Getenv("CUSTOM_CA_KEY")
	certFile := os.Getenv("CUSTOM_CA_CERT")
	addr := os.Getenv("PROXY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if keyFile == "" || certFile == "" {
		fmt.Println("CUSTOM_CA_KEY and CUSTOM_CA_CERT required")
		os.Exit(1)
	}

	ca, err := cert.NewCustomCA(cert.CustomCAOptions{
		KeyFile:  keyFile,
		CertFile: certFile,
	})
	if err != nil {
		fmt.Printf("load CA: %v\n", err)
		os.Exit(1)
	}

	proxy, err := mproxy.NewProxy(&mproxy.Options{
		Addr:        addr,
		SslInsecure: true,
		CustomCA:    ca,
	})
	if err != nil {
		fmt.Printf("new proxy: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("proxy listening on %s (CustomCA: %s)\n", addr, ca.GetRootCA().Subject.CommonName)
	go proxy.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	proxy.Close()
}
