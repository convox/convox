package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const tenantIngressManifest = "services:\n  web:\n    image: docker.io/library/nginx\n    port: 5000\n"

func promoteTenantIngress(t *testing.T, tid, repromote bool, annotation, env string) (string, bool, error) {
	t.Helper()

	var template string
	var applied bool
	var perr error

	testProvider(t, func(p *k8s.Provider) {
		if tid {
			p.FeatureGates = map[string]bool{options.FeatureGateTid: true}
		}

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cc, ok := p.Convox.(*cvfake.Clientset)
		require.True(t, ok)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release0", tenantIngressManifest))
		_, err := cc.ConvoxV1().Releases("rack1-app1").Create(&ca.Release{
			ObjectMeta: am.ObjectMeta{Name: "release1"},
			Spec: ca.ReleaseSpec{
				Build:    "build1",
				Env:      env,
				Manifest: tenantIngressManifest + "    ingressAnnotations:\n      - " + annotation + "\n",
				Created:  "20200101.000000.000000000",
			},
		})
		require.NoError(t, err)
		require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

		current := "release0"
		if repromote {
			current = "release1"
		}

		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)
		aa.On("Status", "rack1-app1", "app").Return("Running", current, nil)
		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Maybe().Run(func(args mock.Arguments) {
			cfg, ok := args.Get(2).(*atom.ApplyConfig)
			require.True(t, ok)
			applied = true
			template = string(cfg.Template)
		})

		perr = p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{})
	})

	return template, applied, perr
}

func TestReleasePromoteTenantIngressAnnotations(t *testing.T) {
	t.Run("GateOffKeepsSnippet", func(t *testing.T) {
		template, applied, err := promoteTenantIngress(t, false, false, "nginx.ingress.kubernetes.io/configuration-snippet=return 418;", "")
		require.NoError(t, err)
		require.True(t, applied)
		require.Contains(t, template, "nginx.ingress.kubernetes.io/configuration-snippet")
	})

	t.Run("GateOnRejectsSnippet", func(t *testing.T) {
		_, applied, err := promoteTenantIngress(t, true, false, "nginx.ingress.kubernetes.io/configuration-snippet=return 418;", "")
		require.ErrorContains(t, err, "ingress annotation nginx.ingress.kubernetes.io/configuration-snippet is not supported on this rack")
		require.False(t, applied)
	})

	t.Run("GateOnSkipsRepromote", func(t *testing.T) {
		_, applied, err := promoteTenantIngress(t, true, true, "nginx.ingress.kubernetes.io/canary=true", "")
		require.NoError(t, err)
		require.True(t, applied)
	})

	t.Run("GateOnRejectsInterpolated", func(t *testing.T) {
		_, applied, err := promoteTenantIngress(t, true, false, "${ANN}", "ANN=nginx.ingress.kubernetes.io/configuration-snippet=x")
		require.ErrorContains(t, err, "ingress annotation nginx.ingress.kubernetes.io/configuration-snippet is not supported on this rack")
		require.False(t, applied)
	})

	t.Run("AllowedRendersSame", func(t *testing.T) {
		off, _, err := promoteTenantIngress(t, false, false, "nginx.ingress.kubernetes.io/limit-rpm=10", "")
		require.NoError(t, err)
		on, _, err := promoteTenantIngress(t, true, false, "nginx.ingress.kubernetes.io/limit-rpm=10", "")
		require.NoError(t, err)
		require.Contains(t, on, "nginx.ingress.kubernetes.io/limit-rpm")
		require.Equal(t, off, on)
	})
}
