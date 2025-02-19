package main

import (
	"encoding/base64"
	"errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

type DefaultBasicAuth struct {
	Auth map[string]string
}

// Create a new BasicAuth instance from a user:password string
func NewDefaultBasicAuth(auth string) *DefaultBasicAuth {
	basicAuth := &DefaultBasicAuth{
		Auth: make(map[string]string),
	}
	for _, e := range strings.Split(auth, "|") {
		n := strings.SplitN(e, ":", 2)
		if len(n) != 2 {
			log.Fatalf("Invalid proxy auth format: %s, expected user:pass", e)
		}
		basicAuth.Auth[n[0]] = n[1]
	}
	return basicAuth
}

// Validate proxy authentication
func (auth *DefaultBasicAuth) EntryAuth(res http.ResponseWriter, req *http.Request) (bool, error) {
	get := req.Header.Get("Proxy-Authorization")
	if get == "" {
		return false, errors.New("missing authentication")
	}
	ret := auth.parseRequestAuth(get)
	if !ret {
		return false, errors.New("invalid credentials")
	}
	return true, nil
}

// Parse and verify the Proxy-Authorization header
func (user *DefaultBasicAuth) parseRequestAuth(proxyAuth string) bool {
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
	if s, ok := user.Auth[n[0]]; !ok || s != n[1] {
		return false
	}
	return true
}
