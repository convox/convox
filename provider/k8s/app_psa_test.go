package k8s_test

import (
	"context"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureBuildNamespaceUnlabeled(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		require.NoError(t, p.EnsureBuildNamespaceForTest("app1"))

		kk, _ := p.Cluster.(*fake.Clientset)
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-build-app1", am.GetOptions{})
		require.NoError(t, err)

		for k := range ns.Labels {
			assert.NotContains(t, k, "pod-security.kubernetes.io", "build namespace must not carry a PSA label")
		}
		assert.Equal(t, "build", ns.Labels["type"])
		_, hasApp := ns.Labels["app"]
		assert.False(t, hasApp, "build namespace must omit the app label to avoid budget double-count")

		// idempotent: a second ensure does not error
		require.NoError(t, p.EnsureBuildNamespaceForTest("app1"))
	})
}

func TestAppDeleteBuildNamespaceCollisionSafe(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa, _ := p.Atom.(*atom.MockInterface)
		kk, _ := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// A separate real app whose namespace name (rack1-build-app1) collides
		// with app1's build-namespace name. Deleting app1 must NOT delete it.
		require.NoError(t, appCreate(kk, "rack1", "build-app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()

		require.NoError(t, p.AppDelete("app1"))

		_, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.Error(t, err)

		_, err = kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-build-app1", am.GetOptions{})
		require.NoError(t, err, "a different app's live namespace must survive AppDelete of the colliding app")
	})
}

func TestProcessBuildNamespace(t *testing.T) {
	cases := []struct {
		name     string
		standard string
		mode     string
		want     string
	}{
		{name: "off", standard: "", mode: "warn", want: "rack1-app1"},
		{name: "warn", standard: "baseline", mode: "warn", want: "rack1-app1"},
		{name: "audit", standard: "baseline", mode: "audit", want: "rack1-app1"},
		{name: "enforce", standard: "baseline", mode: "enforce", want: "rack1-build-app1"},
		{name: "restricted-enforce", standard: "restricted", mode: "enforce", want: "rack1-build-app1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				p.PodSecurityStandard = tc.standard
				p.PodSecurityMode = tc.mode
				assert.Equal(t, tc.want, p.ProcessBuildNamespaceForTest("app1"))
			})
		})
	}
}

func TestAppCreatePodSecurity(t *testing.T) {
	cases := []struct {
		name     string
		standard string
		mode     string
		want     string
		absent   bool
	}{
		{name: "baseline-warn", standard: "baseline", mode: "warn", want: "pod-security.kubernetes.io/warn: baseline"},
		{name: "baseline-enforce", standard: "baseline", mode: "enforce", want: "pod-security.kubernetes.io/enforce: baseline"},
		{name: "restricted-audit", standard: "restricted", mode: "audit", want: "pod-security.kubernetes.io/audit: restricted"},
		{name: "default-off", standard: "", mode: "warn", absent: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				p.PodSecurityStandard = tc.standard
				p.PodSecurityMode = tc.mode

				aa, _ := p.Atom.(*atom.MockInterface)
				aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
					cfg, _ := args.Get(2).(*atom.ApplyConfig)
					rendered := string(cfg.Template)
					if tc.absent {
						assert.NotContains(t, rendered, "pod-security.kubernetes.io/")
					} else {
						assert.Contains(t, rendered, tc.want)
						assert.Equal(t, 1, strings.Count(rendered, "pod-security.kubernetes.io/"))
					}
				})
				aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Times(3)

				_, err := p.AppCreate("app1", structs.AppCreateOptions{})
				require.NoError(t, err)
			})
		})
	}
}
