package proxy

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var caErrNotFound = errors.New("ca not found")

type CA struct {
	rsa.PrivateKey
	RootCert  x509.Certificate
	StorePath string
}

func NewCA(path string) (*CA, error) {
	storePath, err := getStorePath(path)
	if err != nil {
		return nil, err
	}

	ca := &CA{StorePath: storePath}

	if err := ca.load(); err != nil {
		if err != caErrNotFound {
			return nil, err
		}
	}

	if err := ca.create(); err != nil {
		return nil, err
	}

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

func (ca *CA) load() error {
	caFile := filepath.Join(ca.StorePath, "mitmproxy-ca.pem")
	stat, err := os.Stat(caFile)
	if err != nil {
		if os.IsNotExist(err) {
			return caErrNotFound
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

	key, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return err
	}
	if v, ok := key.(*rsa.PrivateKey); ok {
		ca.PrivateKey = *v
	} else {
		return errors.New("found unknown rsa private key type in PKCS#8 wrapping")
	}

	x509Cert, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		return err
	}
	ca.RootCert = *x509Cert

	return nil
}

func (ca *CA) create() error {
	return nil
}
