package k8s

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureSelfSignedCA(t *testing.T) {
	p := &Provider{
		Cluster:   fake.NewSimpleClientset(),
		Namespace: "test-system",
	}

	require.NoError(t, p.EnsureSelfSignedCA())

	sec, err := p.Cluster.CoreV1().Secrets("test-system").Get(context.TODO(), "ca", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, corev1.SecretTypeTLS, sec.Type)
	require.NotEmpty(t, sec.Data["tls.crt"])
	require.NotEmpty(t, sec.Data["tls.key"])

	block, _ := pem.Decode(sec.Data["tls.crt"])
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	require.True(t, cert.IsCA)
	require.Equal(t, "Convox Rack CA", cert.Subject.CommonName)

	require.NoError(t, p.EnsureSelfSignedCA())
	sec2, err := p.Cluster.CoreV1().Secrets("test-system").Get(context.TODO(), "ca", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, sec.Data["tls.crt"], sec2.Data["tls.crt"])
}
