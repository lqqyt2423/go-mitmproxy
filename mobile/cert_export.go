package mobile

import (
	"bytes"
	"encoding/pem"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// getCACertPEM returns the root CA certificate in PEM format.
func getCACertPEM(p *proxy.Proxy) (string, error) {
	cert := p.GetCertificate()
	var buf bytes.Buffer
	err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// getCACertDER returns the root CA certificate in DER format.
// Useful for iOS .mobileconfig profile generation.
func getCACertDER(p *proxy.Proxy) ([]byte, error) {
	cert := p.GetCertificate()
	return cert.Raw, nil
}
