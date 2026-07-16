package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	k8s "github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReleaseTemplateServicesBuildArchWiring(t *testing.T) {
	render := func(t *testing.T, arch string) string {
		out, _, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
			kk, _ := p.Cluster.(*fake.Clientset)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})
			ss := scaleManifestServices(t, map[string]int{"web": 1})

			a := &structs.App{
				Name:       "app1",
				Release:    "release1",
				Parameters: map[string]string{structs.AppParamBuildArch: arch},
			}
			r := &structs.Release{Id: "release1", App: "app1"}
			return a, r, ss
		})
		require.NoError(t, err)
		return string(out)
	}

	t.Run("valid arch reaches promoted pods through appBuildArch wiring", func(t *testing.T) {
		out := render(t, "arm64")
		require.Contains(t, out, "kubernetes.io/arch")
		require.Contains(t, out, "arm64")
	})

	t.Run("invalid arch is sanitized by the wiring", func(t *testing.T) {
		out := render(t, "x86")
		require.NotContains(t, out, "kubernetes.io/arch")
	})

	t.Run("unset arch renders no pin", func(t *testing.T) {
		out := render(t, "")
		require.NotContains(t, out, "kubernetes.io/arch")
	})
}
