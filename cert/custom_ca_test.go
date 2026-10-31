package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// genTestECCA generates a test EC P-384 CA (mimics eMudhra key type).
func genTestECCA(t *testing.T) (keyPath, certPath string) {
	t.Helper()
	dir := t.TempDir()

	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test EC CA",
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		SignatureAlgorithm:    x509.ECDSAWithSHA384,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyPath = filepath.Join(dir, "ca-key.pem")
	certPath = filepath.Join(dir, "ca-cert.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	return keyPath, certPath
}

func TestCustomCAWithECKey(t *testing.T) {
	keyPath, certPath := genTestECCA(t)

	ca, err := NewCustomCA(CustomCAOptions{
		KeyFile:  keyPath,
		CertFile: certPath,
	})
	if err != nil {
		t.Fatalf("NewCustomCA: %v", err)
	}

	rootCA := ca.GetRootCA()
	if rootCA == nil {
		t.Fatal("GetRootCA returned nil")
	}

	leaf, err := ca.GetCert("example.com")
	if err != nil {
		t.Fatalf("GetCert: %v", err)
	}

	if len(leaf.Certificate) < 2 {
		t.Fatalf("expected chain of 2, got %d", len(leaf.Certificate))
	}

	leafCert, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if leafCert.DNSNames[0] != "example.com" {
		t.Fatalf("expected example.com, got %v", leafCert.DNSNames)
	}

	if err := leafCert.CheckSignatureFrom(ca.GetRootCA()); err != nil {
		t.Fatalf("leaf not signed by CA: %v", err)
	}
}
