package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"
)

// CustomCA supports loading an existing CA certificate and private key (RSA or EC)
// from PEM files. Unlike SelfSignCA which always generates a self-signed root,
// CustomCA loads a pre-existing intermediate CA — enabling scenarios like
// eMudhra CA MITM where the forged chain anchors to a root already in the
// device system trust store (zero device configuration required).
type CustomCA struct {
	signer    crypto.Signer    // RSA or EC private key
	rootCert  *x509.Certificate // intermediate CA cert
	chain     []*x509.Certificate // intermediate + (optional) additional chain certs
	StorePath string

	cache  *lru.Cache
	group  *singleflight.Group
	cacheMu sync.Mutex
}

// CustomCAOptions configures CustomCA loading.
type CustomCAOptions struct {
	// KeyFile contains the private key in PEM (EC or RSA).
	KeyFile string
	// CertFile contains the CA certificate in PEM.
	CertFile string
	// ChainFiles contains optional additional chain certificates (PEM).
	ChainFiles []string
}

// NewCustomCA loads an existing CA from PEM files.
// The key can be RSA or EC (P-256/P-384/P-521). Leaf certificates are signed
// with the loaded key. The returned tls.Certificate includes the full chain
// (leaf + intermediate + extras) so clients receive a complete trust path.
func NewCustomCA(opts CustomCAOptions) (CA, error) {
	keyData, err := os.ReadFile(opts.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("read key file %s: %w", opts.KeyFile, err)
	}
	signer, err := parsePrivateKeyPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	certData, err := os.ReadFile(opts.CertFile)
	if err != nil {
		return nil, fmt.Errorf("read cert file %s: %w", opts.CertFile, err)
	}
	// Parse every CERTIFICATE block: the first is the signing intermediate;
	// the rest are additional chain certs (e.g. the eMudhra cross-sign) sent
	// alongside the leaf so clients can build a path to a trusted root.
	var chainCerts []*x509.Certificate
	for rest := certData; ; {
		block, more := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = more
		if block.Type != "CERTIFICATE" {
			continue
		}
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		chainCerts = append(chainCerts, parsed)
		if len(rest) == 0 {
			break
		}
	}
	if len(chainCerts) == 0 {
		return nil, errors.New("no CERTIFICATE block found in cert file")
	}
	rootCert := chainCerts[0]

	// Verify the key matches the certificate
	switch pub := rootCert.PublicKey.(type) {
	case *rsa.PublicKey:
		rsaKey, ok := signer.(*rsa.PrivateKey)
		if !ok || pub.N.Cmp(rsaKey.N) != 0 {
			return nil, errors.New("private key does not match certificate public key (RSA)")
		}
	case *ecdsa.PublicKey:
		ecKey, ok := signer.(*ecdsa.PrivateKey)
		if !ok || pub.Curve != elliptic.Curve(ecKey.Curve) || pub.X.Cmp(ecKey.X) != 0 {
			return nil, errors.New("private key does not match certificate public key (EC)")
		}
	default:
		return nil, fmt.Errorf("unsupported public key type: %T", rootCert.PublicKey)
	}

	ca := &CustomCA{
		signer:   signer,
		rootCert: rootCert,
		chain:    chainCerts,
		cache:    lru.New(100),
		group:    new(singleflight.Group),
	}

	for _, cf := range opts.ChainFiles {
		cd, err := os.ReadFile(cf)
		if err != nil {
			return nil, fmt.Errorf("read chain file %s: %w", cf, err)
		}
		for {
			block, rest := pem.Decode(cd)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				chained, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return nil, fmt.Errorf("parse chain cert: %w", err)
				}
				ca.chain = append(ca.chain, chained)
			}
			cd = rest
			if len(cd) == 0 {
				break
			}
		}
	}

	return ca, nil
}

// GetRootCA returns the loaded intermediate CA certificate.
func (ca *CustomCA) GetRootCA() *x509.Certificate {
	return ca.rootCert
}

// GetCert signs a leaf certificate for the given commonName using the loaded CA.
// The returned tls.Certificate includes the full chain so the client sees
// leaf → intermediate → root (already trusted by the system).
func (ca *CustomCA) GetCert(commonName string) (*tls.Certificate, error) {
	ca.cacheMu.Lock()
	if val, ok := ca.cache.Get(commonName); ok {
		ca.cacheMu.Unlock()
		return val.(*tls.Certificate), nil
	}
	ca.cacheMu.Unlock()

	val, err := ca.group.Do(commonName, func() (interface{}, error) {
		cert, err := ca.DummyCert(commonName)
		if err == nil {
			ca.cacheMu.Lock()
			ca.cache.Add(commonName, cert)
			ca.cacheMu.Unlock()
		}
		return cert, nil
	})
	if err != nil {
		return nil, err
	}
	return val.(*tls.Certificate), nil
}

// DummyCert creates a leaf certificate signed by the loaded CA key.
func (ca *CustomCA) DummyCert(commonName string) (*tls.Certificate, error) {
	// Random serial, like a real CA. Time-derived serials can collide across
	// concurrent issuances for different hosts.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		NotBefore:   time.Now().Add(-time.Hour * 48),
		NotAfter:    time.Now().Add(time.Hour * 24 * 365),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	// Set signature algorithm based on key type
	switch ca.signer.(type) {
	case *ecdsa.PrivateKey:
		template.SignatureAlgorithm = x509.ECDSAWithSHA256
	default:
		template.SignatureAlgorithm = x509.SHA256WithRSA
	}

	ip := net.ParseIP(commonName)
	if ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{commonName}
	}

	// Generate a leaf key (separate from the CA key)
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, ca.rootCert, &leafKey.PublicKey, ca.signer)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	// Build the full chain: leaf + intermediate (+ extras)
	chain := make([][]byte, 0, len(ca.chain)+1)
	chain = append(chain, certBytes)
	for _, c := range ca.chain {
		chain = append(chain, c.Raw)
	}

	return &tls.Certificate{
		Certificate: chain,
		PrivateKey:  leafKey,
	}, nil
}

// parsePrivateKeyPEM parses a PEM private key (RSA or EC, PKCS#1 or PKCS#8).
// Skips EC PARAMETERS blocks (openssl ecparam includes them).
func parsePrivateKeyPEM(data []byte) (crypto.Signer, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, errors.New("no PEM block found")
		}
		if block.Type == "EC PARAMETERS" {
			continue
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			switch k := key.(type) {
			case *rsa.PrivateKey:
				return k, nil
			case *ecdsa.PrivateKey:
				return k, nil
			default:
				return nil, fmt.Errorf("unsupported private key type: %T", key)
			}
		default:
			return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
		}
	}
}

var _ CA = (*CustomCA)(nil)
