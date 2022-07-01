package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"
	log "github.com/sirupsen/logrus"
)

// reference
// https://docs.mitmproxy.org/stable/concepts-certificates/
// https://github.com/mitmproxy/mitmproxy/blob/master/mitmproxy/certs.py

var errCaNotFound = errors.New("ca not found")

type CA struct {
	rsa.PrivateKey
	RootCert  x509.Certificate
	StorePath string

	cache *lru.Cache
	group *singleflight.Group

	cacheMu sync.Mutex
}

func createCert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() / 100000),
		Subject: pkix.Name{
			CommonName:   "mitmproxy",
			Organization: []string{"mitmproxy"},
		},
		NotBefore:             time.Now().Add(-time.Hour * 48),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365 * 3),
		BasicConstraintsValid: true,
		IsCA:                  true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageEmailProtection,
			x509.ExtKeyUsageTimeStamping,
			x509.ExtKeyUsageCodeSigning,
			x509.ExtKeyUsageMicrosoftCommercialCodeSigning,
			x509.ExtKeyUsageMicrosoftServerGatedCrypto,
			x509.ExtKeyUsageNetscapeServerGatedCrypto,
		},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

// Create new ca only live in memory, will change when process restart
func NewCAMemory() (*CA, error) {
	key, cert, err := createCert()
	if err != nil {
		return nil, err
	}
	return &CA{
		PrivateKey: *key,
		RootCert:   *cert,
		StorePath:  "",
		cache:      lru.New(100),
		group:      new(singleflight.Group),
	}, nil
}

// Load ca from store path or create new ca then store
func NewCA(path string) (*CA, error) {
	storePath, err := getStorePath(path)
	if err != nil {
		return nil, err
	}

	ca := &CA{
		StorePath: storePath,
		cache:     lru.New(100),
		group:     new(singleflight.Group),
	}

	if err := ca.load(); err != nil {
		if err != errCaNotFound {
			return nil, err
		}
	} else {
		log.Debug("load root ca")
		return ca, nil
	}

	if err := ca.create(); err != nil {
		return nil, err
	}
	log.Debug("create root ca")
	return ca, nil
}

func getStorePath(path string) (string, error) {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, ".mitmproxy")
	}

	if !filepath.IsAbs(path) {
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(dir, path)
	}

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	} else {
		if !stat.Mode().IsDir() {
			return "", fmt.Errorf("路径 %v 不是文件夹，请移除此文件重试", path)
		}
	}

	return path, nil
}

// The certificate and the private key in PEM format.
func (ca *CA) caFile() string {
	return filepath.Join(ca.StorePath, "mitmproxy-ca.pem")
}

// The certificate in PEM format.
func (ca *CA) caCertFile() string {
	return filepath.Join(ca.StorePath, "mitmproxy-ca-cert.pem")
}

func (ca *CA) caCertCerFile() string {
	return filepath.Join(ca.StorePath, "mitmproxy-ca-cert.cer")
}

func (ca *CA) load() error {
	caFile := ca.caFile()
	stat, err := os.Stat(caFile)
	if err != nil {
		if os.IsNotExist(err) {
			return errCaNotFound
		}
		return err
	}

	if !stat.Mode().IsRegular() {
		return fmt.Errorf("%v 不是文件", caFile)
	}

	data, err := ioutil.ReadFile(caFile)
	if err != nil {
		return err
	}

	keyDERBlock, data := pem.Decode(data)
	if keyDERBlock == nil {
		return fmt.Errorf("%v 中不存在 PRIVATE KEY", caFile)
	}
	certDERBlock, _ := pem.Decode(data)
	if certDERBlock == nil {
		return fmt.Errorf("%v 中不存在 CERTIFICATE", caFile)
	}

	var privateKey *rsa.PrivateKey
	key, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		// fix #14
		if strings.Contains(err.Error(), "use ParsePKCS1PrivateKey instead") {
			privateKey, err = x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if v, ok := key.(*rsa.PrivateKey); ok {
			privateKey = v
		} else {
			return errors.New("found unknown rsa private key type in PKCS#8 wrapping")
		}
	}
	ca.PrivateKey = *privateKey

	x509Cert, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		return err
	}
	ca.RootCert = *x509Cert

	return nil
}

func (ca *CA) create() error {
	key, cert, err := createCert()
	if err != nil {
		return err
	}

	ca.PrivateKey = *key
	ca.RootCert = *cert

	if err := ca.save(); err != nil {
		return err
	}
	return ca.saveCert()
}

func (ca *CA) saveTo(out io.Writer) error {
	keyBytes, err := x509.MarshalPKCS8PrivateKey(&ca.PrivateKey)
	if err != nil {
		return err
	}
	err = pem.Encode(out, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	if err != nil {
		return err
	}

	return pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: ca.RootCert.Raw})
}

func (ca *CA) saveCertTo(out io.Writer) error {
	return pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: ca.RootCert.Raw})
}

func (ca *CA) save() error {
	file, err := os.Create(ca.caFile())
	if err != nil {
		return err
	}
	defer file.Close()
	return ca.saveTo(file)
}

func (ca *CA) saveCert() error {
	file, err := os.Create(ca.caCertFile())
	if err != nil {
		return err
	}
	defer file.Close()
	err = ca.saveCertTo(file)
	if err != nil {
		return err
	}

	cerFile, err := os.Create(ca.caCertCerFile())
	if err != nil {
		return err
	}
	defer cerFile.Close()
	err = ca.saveCertTo(cerFile)
	if err != nil {
		return err
	}
	return err
}

func (ca *CA) GetCert(commonName string) (*tls.Certificate, error) {
	ca.cacheMu.Lock()
	if val, ok := ca.cache.Get(commonName); ok {
		ca.cacheMu.Unlock()
		log.Debugf("ca GetCert: %v", commonName)
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
		return cert, err
	})

	if err != nil {
		return nil, err
	}

	return val.(*tls.Certificate), nil
}

// TODO: 是否应该支持多个 SubjectAltName
func (ca *CA) DummyCert(commonName string) (*tls.Certificate, error) {
	log.Debugf("ca DummyCert: %v", commonName)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() / 100000),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"mitmproxy"},
		},
		NotBefore:          time.Now().Add(-time.Hour * 48),
		NotAfter:           time.Now().Add(time.Hour * 24 * 365),
		SignatureAlgorithm: x509.SHA256WithRSA,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	ip := net.ParseIP(commonName)
	if ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{commonName}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, &ca.RootCert, &ca.PrivateKey.PublicKey, &ca.PrivateKey)
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  &ca.PrivateKey,
	}

	return cert, nil
}
