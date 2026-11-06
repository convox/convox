package k8s_test

import (
	"context"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTenantNamespacePrefix(t *testing.T) {
	p := &k8s.Provider{Name: "rk"}

	require.Equal(t, "rk-ab12-", p.TenantNamespacePrefix("ab12"))
	require.Equal(t, "", p.TenantNamespacePrefix(""))
	require.Equal(t, "", (&k8s.Provider{}).TenantNamespacePrefix("ab12"))

	var nilp *k8s.Provider
	require.Equal(t, "", nilp.TenantNamespacePrefix("ab12"))
}

func TestProcessBuildNamespaceStaysInAppNamespaceWithoutTidGate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		require.Equal(t, p.AppNamespace("app1"), p.ProcessBuildNamespaceForTest("app1"))
	})
}

func TestProcessBuildNamespaceIsolatedOnTenantRack(t *testing.T) {
	testProvider(t, func(base *k8s.Provider) {
		base.FeatureGates = map[string]bool{options.FeatureGateTid: true}
		p := withTID(t, base, "ab12")

		ns := p.ProcessBuildNamespaceForTest("app1")

		require.NotEqual(t, p.AppNamespace("app1"), ns,
			"the build pod carries the rack password in its environment, so a tenant must not share its namespace")
		require.False(t, strings.HasPrefix(ns, p.TenantNamespacePrefix("ab12")),
			"the build namespace must also fall outside the prefix the proxy guard admits")
	})
}

func TestBuildCreateExternalWithholdsCredentialOnTenantRack(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		p.FeatureGates = map[string]bool{options.FeatureGateTid: true}
		p.Password = "rack-pass"

		b, err := p.BuildCreate("tidapp", "", externalBuildFixture(t, p, "tidapp"))
		require.NoError(t, err, "image import uses this same path and must keep working")
		require.Equal(t, "", b.Repository, "the repository URL embeds the rack password")
	})
}

func TestBuildCreateExternalReturnsCredentialWithoutTidGate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		p.Password = "rack-pass"

		b, err := p.BuildCreate("plainapp", "", externalBuildFixture(t, p, "plainapp"))
		require.NoError(t, err)
		require.Contains(t, b.Repository, "rack-pass", "an ungated rack must still return a pushable URL")
	})
}

func withTID(t *testing.T, p *k8s.Provider, tid string) *k8s.Provider {
	t.Helper()

	tp, ok := p.WithContext(context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, tid)).(*k8s.Provider)
	require.True(t, ok)

	return tp
}

func externalBuildFixture(t *testing.T, p *k8s.Provider, app string) structs.BuildCreateOptions {
	t.Helper()

	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)
	require.NoError(t, appCreate(kk, p.Name, app))

	aa, ok := p.Atom.(*atom.MockInterface)
	require.True(t, ok)
	aa.On("Status", p.AppNamespace(app), "app").Return("Running", "R1234567", nil).Maybe()

	return structs.BuildCreateOptions{External: options.Bool(true)}
}
