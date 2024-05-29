package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"
	"github.com/lqqyt2423/go-mitmproxy/cert"
	log "github.com/sirupsen/logrus"
)

type TrustedCA struct {
	cache   *lru.Cache
	group   *singleflight.Group
	cacheMu sync.Mutex
}

func NewTrustedCA() (cert.CA, error) {
	ca := &TrustedCA{
		cache: lru.New(100),
		group: new(singleflight.Group),
	}
	return ca, nil
}

func (ca *TrustedCA) GetRootCA() *x509.Certificate {
	panic("not supported")
}

func (ca *TrustedCA) GetCert(commonName string) (*tls.Certificate, error) {
	ca.cacheMu.Lock()
	if val, ok := ca.cache.Get(commonName); ok {
		ca.cacheMu.Unlock()
		log.Debugf("ca GetCert: %v", commonName)
		return val.(*tls.Certificate), nil
	}
	ca.cacheMu.Unlock()

	val, err := ca.group.Do(commonName, func() (interface{}, error) {
		certificate, err := ca.loadCert(commonName)
		if err == nil {
			ca.cacheMu.Lock()
			ca.cache.Add(commonName, certificate)
			ca.cacheMu.Unlock()
		}
		return certificate, err
	})

	if err != nil {
		return nil, err
	}

	return val.(*tls.Certificate), nil
}

func (ca *TrustedCA) loadCert(commonName string) (*tls.Certificate, error) {
	switch commonName {
	case "your-domain.xx.com":
		certificate, err := tls.LoadX509KeyPair("cert Path", "key Path")
		if err != nil {
			return nil, err
		}
		return &certificate, err
	case "your-domain2.xx.com":
		certificate, err := tls.X509KeyPair([]byte("cert Block"), []byte("key Block"))
		if err != nil {
			return nil, err
		}
		return &certificate, err
	default:
		return nil, errors.New("invalid certificate name")
	}
}
