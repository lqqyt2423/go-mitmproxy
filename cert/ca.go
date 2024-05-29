package cert

import (
	"crypto/tls"
	"crypto/x509"
)

type CA interface {
	GetRootCA() *x509.Certificate
	GetCert(commonName string) (*tls.Certificate, error)
}
