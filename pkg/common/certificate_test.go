package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestCertificateSelfSigned_GeneratesECDSA(t *testing.T) {
	cert, err := CertificateSelfSigned("test.example.com")
	if err != nil {
		t.Fatalf("CertificateSelfSigned: %v", err)
	}

	key, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("expected *ecdsa.PrivateKey, got %T", cert.PrivateKey)
	}
	if key.Curve != elliptic.P256() {
		t.Fatalf("expected P-256 curve, got %v", key.Curve.Params().Name)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if x509Cert.Subject.CommonName != "test.example.com" {
		t.Errorf("CommonName = %q, want %q", x509Cert.Subject.CommonName, "test.example.com")
	}
	if x509Cert.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("PublicKeyAlgorithm = %v, want ECDSA", x509Cert.PublicKeyAlgorithm)
	}
}

func TestCertificateSelfSigned_RoundTrip(t *testing.T) {
	cert, err := CertificateSelfSigned("roundtrip.example.com")
	if err != nil {
		t.Fatalf("CertificateSelfSigned: %v", err)
	}

	pub, keyPEM, err := CertificateParts(cert)
	if err != nil {
		t.Fatalf("CertificateParts: %v", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		t.Fatal("failed to decode key PEM")
	}
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("PEM block type = %q, want %q", block.Type, "EC PRIVATE KEY")
	}

	_, err = tls.X509KeyPair(pub, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair round-trip failed: %v", err)
	}
}

func TestCertificateCA_GeneratesECDSA(t *testing.T) {
	ca, err := CertificateSelfSigned("ca.example.com")
	if err != nil {
		t.Fatalf("CertificateSelfSigned (CA): %v", err)
	}

	leaf, err := CertificateCA("leaf.example.com", ca)
	if err != nil {
		t.Fatalf("CertificateCA: %v", err)
	}

	key, ok := leaf.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("expected *ecdsa.PrivateKey, got %T", leaf.PrivateKey)
	}
	if key.Curve != elliptic.P256() {
		t.Fatalf("expected P-256 curve, got %v", key.Curve.Params().Name)
	}

	x509Leaf, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if x509Leaf.Subject.CommonName != "leaf.example.com" {
		t.Errorf("CommonName = %q, want %q", x509Leaf.Subject.CommonName, "leaf.example.com")
	}
	if len(x509Leaf.DNSNames) < 2 {
		t.Fatalf("expected wildcard + bare DNS names, got %v", x509Leaf.DNSNames)
	}
}

func TestCertificateParts_ECDSA(t *testing.T) {
	cert, err := CertificateSelfSigned("ecdsa.example.com")
	if err != nil {
		t.Fatalf("CertificateSelfSigned: %v", err)
	}

	pub, keyPEM, err := CertificateParts(cert)
	if err != nil {
		t.Fatalf("CertificateParts: %v", err)
	}
	if len(pub) == 0 {
		t.Fatal("empty public cert PEM")
	}
	if len(keyPEM) == 0 {
		t.Fatal("empty private key PEM")
	}

	block, _ := pem.Decode(keyPEM)
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("key PEM type = %q, want %q", block.Type, "EC PRIVATE KEY")
	}

	_, err = x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParseECPrivateKey: %v", err)
	}
}

func TestCertificateParts_RSABackwardCompat(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	cert := &tls.Certificate{
		PrivateKey: key,
	}

	_, keyPEM, err := CertificateParts(cert)
	if err != nil {
		t.Fatalf("CertificateParts (RSA): %v", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("key PEM type = %q, want %q", block.Type, "RSA PRIVATE KEY")
	}
}

func TestCertificateParts_UnsupportedKeyType(t *testing.T) {
	cert := &tls.Certificate{
		PrivateKey: "not-a-key",
	}

	_, _, err := CertificateParts(cert)
	if err == nil {
		t.Fatal("expected error for unsupported key type")
	}
}
