package local

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Provider struct {
	*k8s.Provider

	Registry string
	Secret   string
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider: k,
		Registry: os.Getenv("REGISTRY"),
		Secret:   os.Getenv("SECRET"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	opts.IgnorePriorityClass = true

	// Ensure a self-signed CA secret exists before the parent initializes
	// cert-manager. The k8s provider's installCertManagerConfig goroutine
	// will find this secret and create the self-signed ClusterIssuer from it.
	if err := p.ensureSelfSignedCA(); err != nil {
		fmt.Printf("warning: could not ensure self-signed CA: %s\n", err)
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	pc, err := NewPodController(p)
	if err != nil {
		return err
	}

	go pc.Run()

	return nil
}

func (p *Provider) ensureSelfSignedCA() error {
	_, err := p.Provider.Cluster.CoreV1().Secrets(p.Provider.Namespace).Get(
		context.TODO(), "ca", am.GetOptions{},
	)
	if err == nil {
		return nil // CA already exists
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serial, err := crand.Int(crand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "Convox Local CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(crand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal CA key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	secret := &corev1.Secret{
		ObjectMeta: am.ObjectMeta{
			Name:      "ca",
			Namespace: p.Provider.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
		Type: corev1.SecretTypeTLS,
	}

	if _, err := p.Provider.Cluster.CoreV1().Secrets(p.Provider.Namespace).Create(
		context.TODO(), secret, am.CreateOptions{},
	); err != nil {
		return fmt.Errorf("failed to create CA secret: %w", err)
	}

	fmt.Printf("generated self-signed CA in %s/ca\n", p.Provider.Namespace)
	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}
