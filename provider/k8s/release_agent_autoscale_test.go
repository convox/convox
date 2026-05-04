package k8s_test

import (
	"os"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// agentAutoscaleManifestYaml — agent service with scale.autoscale set. Spec 04
// demoted manifest validation from hard-fail to WARNING, so this manifest now
// loads/validates and reaches releaseTemplateServices, which must drop
// wantsAutoscale and emit release:agent-autoscale-ignored with actor=system.
func agentAutoscaleManifestYaml() string {
	return `services:
  collector:
    image: docker.io/library/nginx
    agent: true
    scale:
      min: 0
      max: 5
      autoscale:
        queueDepth:
          threshold: 3
`
}

// setupAgentAutoscaleTest wires a Provider seeded with an agent+autoscale
// manifest. Mirrors setupKedaPrometheusTest in release_test.go.
func setupAgentAutoscaleTest(t *testing.T) func(*k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
	t.Helper()
	return func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		p.IsKedaEnabled = true

		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cc, _ := p.Convox.(*cvfake.Clientset)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", agentAutoscaleManifestYaml()))
		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		m, err := manifest.Load([]byte(agentAutoscaleManifestYaml()), structs.Environment{})
		require.NoError(t, err)
		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			m.Services
	}
}

// TestMain sets TEST=true so testProvider's Initialize short-circuits past
// the rack-mode initializePriorityClass step, which expects a real "api"
// Deployment in the rack namespace. Initialize already documents this env
// gate (provider/k8s/k8s.go:298). Without it, every test in this package
// that uses runReleaseTemplateServicesEvents fails at testProvider setup
// with "deployments.apps \"api\" not found", regardless of branch state.
func TestMain(m *testing.M) {
	if os.Getenv("TEST") == "" {
		os.Setenv("TEST", "true")
	}
	os.Exit(m.Run())
}

// TestRelease_AgentAutoscaleIgnored_FiresEvent — agent service with autoscale
// must trigger release:agent-autoscale-ignored with the four-key Data shape
// {actor, app, service, release}. Locks the system-actor convention shared by
// release:autoscale-disabled and release:prometheus-skipped (see
// d3_call_site_actor_test.go).
func TestRelease_AgentAutoscaleIgnored_FiresEvent(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, setupAgentAutoscaleTest(t))
	require.NoError(t, err, "release template render must succeed: %v", err)

	hits := findAllByAction(events, "release:agent-autoscale-ignored")
	require.Len(t, hits, 1, "exactly one release:agent-autoscale-ignored event expected for agent+autoscale service")

	data, _ := hits[0]["data"].(map[string]any)
	require.NotNil(t, data, "event payload must include data block")
	assert.Equal(t, "system", data["actor"], "Data.actor must be 'system'")
	assert.Equal(t, "app1", data["app"], "Data.app must name the app")
	assert.Equal(t, "collector", data["service"], "Data.service must name the agent service")
	assert.Equal(t, "release1", data["release"], "Data.release must be the release id")
}
