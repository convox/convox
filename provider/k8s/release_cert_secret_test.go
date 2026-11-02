package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReleaseTemplateCertSecretTenantGate(t *testing.T) {
	secret := func(name string, kind ac.SecretType, labels map[string]string) *ac.Secret {
		return &ac.Secret{
			ObjectMeta: am.ObjectMeta{Namespace: "rack1", Name: name, Labels: labels},
			Type:       kind,
			Data:       map[string][]byte{"tls.crt": []byte("crt"), "tls.key": []byte("key")},
		}
	}

	objs := []*ac.Secret{
		secret("api-password", ac.SecretTypeOpaque, nil),
		secret("ca", ac.SecretTypeTLS, nil),
		secret("cert-import", ac.SecretTypeTLS, map[string]string{"system": "convox", "type": "certificate"}),
		secret("cert-le", ac.SecretTypeTLS, map[string]string{"system": "convox", "type": "letsencrypt-certificate"}),
		secret("cert-fake", ac.SecretTypeOpaque, map[string]string{"system": "convox", "type": "certificate"}),
	}

	tests := []struct {
		id      string
		gateOn  string
		gateOff string
	}{
		{id: "cert-import"},
		{id: "cert-le"},
		{id: "api-password", gateOn: "certificate api-password not found"},
		{id: "ca", gateOn: "certificate ca not found"},
		{id: "cert-fake", gateOn: "certificate cert-fake not found"},
		{id: "missing", gateOn: "certificate missing not found", gateOff: `secrets "missing" not found`},
	}

	for _, tt := range tests {
		for _, gate := range []bool{true, false} {
			p := newTestAwsProvider()
			for _, o := range objs {
				_, err := p.Cluster.CoreV1().Secrets("rack1").Create(p.ctx, o, am.CreateOptions{})
				require.NoError(t, err)
			}
			p.FeatureGates = map[string]bool{options.FeatureGateTid: gate}

			_, err := p.releaseTemplateCertSecret(&structs.App{Name: "app1"}, manifest.Service{Name: "web", Certificate: manifest.Certificate{Id: tt.id}})

			want := tt.gateOff
			if gate {
				want = tt.gateOn
			}

			if want == "" {
				require.NoError(t, err, "%s gate=%v", tt.id, gate)
			} else {
				require.EqualError(t, err, want, "%s gate=%v", tt.id, gate)
			}
		}
	}
}
