package main

import (
	"encoding/base64"
	"errors"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

type UserAuth struct {
	Username string
	Password string
}

// AuthEntrypAuth handles the proxy authentication for the entry point.
func (user *UserAuth) AuthEntrypAuth(res http.ResponseWriter, req *http.Request) (bool, error) {
	get := req.Header.Get("Proxy-Authorization")
	if get == "" {
		return false, errors.New("empty auth")
	}
	auth := user.parseRequestAuth(get)
	if !auth {
		return false, errors.New("error auth")
	}
	return true, nil
}

// parseRequestAuth decodes and validates the Proxy-Authorization header.
func (user *UserAuth) parseRequestAuth(proxyAuth string) bool {
	if !strings.HasPrefix(proxyAuth, "Basic ") {
		return false
	}
	encodedAuth := strings.TrimPrefix(proxyAuth, "Basic ")
	decodedAuth, err := base64.StdEncoding.DecodeString(encodedAuth)
	if err != nil {
		log.Warnf("Failed to decode Proxy-Authorization header: %v", err)
		return false
	}

	n := strings.SplitN(string(decodedAuth), ":", 2)
	if len(n) < 2 {
		return false
	}
	if user.Username != n[0] || user.Password != n[1] {
		return false
	}
	return true
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
	auth := &UserAuth{
		Username: "proxy",
		Password: "proxy",
	}
	// Set up the authentication handler for the proxy.
	p.SetAuthProxy(auth.AuthEntrypAuth)
	p.AddAddon(&proxy.LogAddon{})

	log.Fatal(p.Start())
}
