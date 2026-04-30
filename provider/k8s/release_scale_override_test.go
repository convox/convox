package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

var errFakeTransient = errors.New("fake transient informer error")

// scaleSeedDeployment seeds a Deployment with explicit replica count and
// optional override annotation. Used by override-honor tests to exercise
// the (sc[s.Name], yaml.min, override) tuple matrix.
func scaleSeedDeployment(t *testing.T, c *fake.Clientset, ns, name string, replicas int32, annotate string) {
	t.Helper()
	r := replicas
	dep := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": "app1", "type": "service", "service": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
		},
	}
	if annotate != "" {
		dep.Annotations = map[string]string{
			k8s.ServiceScaleOverrideAnnotation: annotate,
		}
	}
	_, err := c.AppsV1().Deployments(ns).Create(context.TODO(), dep, am.CreateOptions{})
	require.NoError(t, err)
}

// scaleManifestYaml builds a minimal manifest YAML string with
// scale.count.{min,max} per service. Used by both the in-test
// manifest.Services (for the helper input) and the seeded
// Convox.Release CRD (which AppGet/ReleaseGet read at runtime).
func scaleManifestYaml(services map[string]int) string {
	var b strings.Builder
	b.WriteString("services:\n")
	for name, min := range services {
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteString(":\n")
		b.WriteString("    image: docker.io/library/nginx\n")
		b.WriteString("    port: 5000\n")
		b.WriteString("    scale:\n      count: ")
		b.WriteString(strconv.Itoa(min))
		b.WriteString("-")
		b.WriteString(strconv.Itoa(min + 5))
		b.WriteString("\n")
	}
	return b.String()
}

// scaleManifestServices builds a minimal manifest.Services slice. The
// yaml min is the value the override-active gate must SKIP when the
// annotation is honored.
func scaleManifestServices(t *testing.T, services map[string]int) manifest.Services {
	t.Helper()
	m, err := manifest.Load([]byte(scaleManifestYaml(services)), structs.Environment{})
	require.NoError(t, err)
	return m.Services
}

// scaleSeedAppRelease seeds a Convox CRD release for the manifest the
// test will pass to releaseTemplateServices. Required because the
// helper invokes p.ServiceList(a.Name) which calls AppGet -> Atom mock
// + ReleaseGet (CRD lookup).
func scaleSeedAppRelease(t *testing.T, p *k8s.Provider, ns, releaseID string, services map[string]int) {
	t.Helper()
	aa, _ := p.Atom.(*atom.MockInterface)
	cc, _ := p.Convox.(*cvfake.Clientset)

	aa.On("Status", ns, "app").Return("Running", releaseID, nil)

	require.NoError(t, releaseCreateInline(cc, ns, releaseID, scaleManifestYaml(services)))
}

// releaseCreateInline writes a Convox CRD Release with the supplied
// inline manifest (skipping the YAML-fixture-on-disk path used by
// releaseFixture). Used by scale-override tests to compose minimal
// per-test manifests.
func releaseCreateInline(cc *cvfake.Clientset, ns, id, manifestYaml string) error {
	r := &ca.Release{
		ObjectMeta: am.ObjectMeta{Name: id},
		Spec: ca.ReleaseSpec{
			Build:    "build1",
			Manifest: manifestYaml,
			Created:  "20200101.000000.000000000",
		},
	}
	if _, err := cc.ConvoxV1().Releases(ns).Create(r); err != nil {
		return err
	}
	return nil
}

// webhookCaptureServer spins up a webhook receiver that decodes the
// incoming JSON body into the captured slice (guarded by mu). Mirrors
// the captureMultiPayload pattern in service_scale_override_test.go.
func webhookCaptureServer(mu *sync.Mutex, captured *[]map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(b, &p); err == nil {
			mu.Lock()
			*captured = append(*captured, p)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
}

// ktAction / ktObject — type aliases so the reactor function signature
// in the InformerError test stays terse.
type ktAction = ktesting.Action
type ktObject = runtime.Object

// findAllByAction returns every captured event matching action.
func findAllByAction(events []map[string]any, action string) []map[string]any {
	var out []map[string]any
	for _, e := range events {
		if a, _ := e["action"].(string); a == action {
			out = append(out, e)
		}
	}
	return out
}

// runReleaseTemplateServicesAndCapture runs the release-template path
// against a seeded provider and returns the rendered bytes plus all
// emitted events. Centralizes the heavy setup so tests stay focused on
// their specific assertions.
func runReleaseTemplateServicesAndCapture(t *testing.T,
	provider *k8s.Provider,
	app *structs.App,
	release *structs.Release,
	ss manifest.Services,
) ([]byte, error) {
	t.Helper()
	out, err := k8s.ReleaseTemplateServicesForTest(provider, app, structs.Environment{}, release, ss, structs.ReleasePromoteOptions{})
	return out, err
}

// runReleaseTemplateServicesEvents drives the release-template helper
// while installing a webhook event-capture endpoint. Returns rendered
// bytes (or nil on error) and the list of captured event payloads.
func runReleaseTemplateServicesEvents(t *testing.T,
	setup func(*k8s.Provider) (*structs.App, *structs.Release, manifest.Services),
) ([]byte, []map[string]any, error) {
	t.Helper()
	var (
		mu       sync.Mutex
		captured []map[string]any
		out      []byte
		callErr  error
	)
	srv := webhookCaptureServer(&mu, &captured)
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		app, rel, ss := setup(p)
		out, callErr = runReleaseTemplateServicesAndCapture(t, p, app, rel, ss)
		drainPendingDispatches()
	})
	mu.Lock()
	defer mu.Unlock()
	cp := make([]map[string]any, len(captured))
	copy(cp, captured)
	return out, cp, callErr
}

// ----- Test 1: AnnotationPresent — override honored -----

func TestReleasePromote_ServiceScaleOverrideHonor_AnnotationPresent(t *testing.T) {
	out, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// runtime replicas = 5; annotation present.
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 5, k8s.ServiceScaleOverrideValueOn)

		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})

		a := &structs.App{Name: "app1", Release: "release1"}
		r := &structs.Release{Id: "release1", App: "app1"}
		return a, r, ss
	})
	require.NoError(t, err, "release template render must succeed: %v", err)
	require.NotEmpty(t, out)

	honored := findAllByAction(events, "app:scale-override:honored")
	require.Len(t, honored, 1, "exactly one app:scale-override:honored event expected")
	data, _ := honored[0]["data"].(map[string]any)
	assert.Equal(t, "system", data["actor"])
	assert.Equal(t, "app1", data["app"])
	assert.Equal(t, "web", data["service"])
	assert.Equal(t, "release1", data["release"])
	assert.Equal(t, "5", data["preserved_count"], "preserved_count must equal sc[s.Name] (runtime replicas=5)")
	assert.Equal(t, "2", data["yaml_count_min"], "yaml_count_min must equal manifest.scale.count.min=2")
}

// ----- Test 2: AnnotationAbsent — existing path -----

func TestReleasePromote_ServiceScaleOverrideHonor_AnnotationAbsent(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// runtime replicas = 5; NO annotation.
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 5, "")

		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})

		return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
	})
	require.NoError(t, err)

	honored := findAllByAction(events, "app:scale-override:honored")
	require.Empty(t, honored, "no honored event when annotation is absent")
}

// ----- Test 3: AnnotationStrictTrueOnly — table-driven -----

func TestReleasePromote_ServiceScaleOverrideHonor_AnnotationStrictTrueOnly(t *testing.T) {
	cases := []struct {
		value    string
		expected bool // true = override honored
	}{
		{"true", true},
		{"True", false},
		{"TRUE", false},
		{"yes", false},
		{"1", false},
		{"y", false},
		{"on", false},
		{" true ", false},
		{"\ttrue", false},
		{"true\n", false},
		{"", false},
		{"false", false},
		{"anything-else", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("value="+strconv.Quote(tc.value), func(t *testing.T) {
			_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
				kk, _ := p.Cluster.(*fake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				scaleSeedDeployment(t, kk, "rack1-app1", "web", 5, tc.value)

				scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
				ss := scaleManifestServices(t, map[string]int{"web": 2})
				return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
			})
			require.NoError(t, err)
			honored := findAllByAction(events, "app:scale-override:honored")
			if tc.expected {
				require.Lenf(t, honored, 1, "annotation value %q should activate override", tc.value)
			} else {
				require.Emptyf(t, honored, "annotation value %q must NOT activate override (strict-\"true\" only)", tc.value)
			}
		})
	}
}

// ----- Test 4: HonorsYamlMinNonzero — preserve runtime even when yaml.min nonzero -----

func TestServiceScaleOverride_HonorsYamlMinNonzero(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// runtime=5, yaml.min=2, override active → preserve 5.
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 5, k8s.ServiceScaleOverrideValueOn)
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})
		return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
	})
	require.NoError(t, err)
	honored := findAllByAction(events, "app:scale-override:honored")
	require.Len(t, honored, 1)
	data, _ := honored[0]["data"].(map[string]any)
	assert.Equal(t, "5", data["preserved_count"])
	assert.Equal(t, "2", data["yaml_count_min"])
}

// ----- Test 4a: RuntimeCountZero / yaml.min nonzero — preserve zero -----

func TestReleasePromote_ServiceScaleOverrideHonor_RuntimeCountZeroYamlMinNonzero(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// runtime=0, yaml.min=2, override active → preserve 0 (NOT yaml.min).
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 0, k8s.ServiceScaleOverrideValueOn)
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})
		return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
	})
	require.NoError(t, err)
	honored := findAllByAction(events, "app:scale-override:honored")
	require.Len(t, honored, 1)
	data, _ := honored[0]["data"].(map[string]any)
	assert.Equal(t, "0", data["preserved_count"], "preserved_count must equal sc[s.Name]=0 — explicit zero, NOT yaml fallback")
	assert.Equal(t, "2", data["yaml_count_min"])
}

// ----- Test 5: DeploymentNotYetExists — first-time deploy -----

func TestReleasePromote_ServiceScaleOverrideHonor_DeploymentNotYetExists(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// NO Deployment fixture — first-time deploy scenario.
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})
		return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
	})
	require.NoError(t, err)
	require.Empty(t, findAllByAction(events, "app:scale-override:honored"),
		"no Deployment yet → ServiceList returns empty slice → no override populate → no honored event")
}

// ----- Test 6: GetDeploymentsTransientError_DoesNotAffectOverridePopulate -----
//
// Post-I-1 fix the override populate-loop reads ScaleOverrideActive from
// pss (already populated by ServiceList from the LIST response) instead
// of issuing N additional GetDeploymentFromInformer calls. This test
// pins the invariant that I-1 eliminated: a transient error on any
// subsequent `get deployments` call cannot fail the promote OR change
// override determination, because releaseTemplateServices no longer
// issues such a call inside its override-populate loop. The reactor is
// retained as a regression guard — if a future refactor reintroduces a
// secondary GET inside releaseTemplateServices, this test would flip and
// flag the regression.
func TestReleasePromote_ServiceScaleOverrideHonor_GetTransientError_NoEffect(t *testing.T) {
	_, events, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 5, k8s.ServiceScaleOverrideValueOn)
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 2})
		ss := scaleManifestServices(t, map[string]int{"web": 2})

		// Inject a Get reactor that errors on every `get deployments`
		// call. ServiceList uses ListDeploymentsFromInformer (LIST, not
		// GET), so this reactor doesn't affect ServiceList. Post-I-1
		// the override populate-loop iterates pss[i].ScaleOverrideActive
		// rather than re-fetching the Deployment, so this reactor also
		// doesn't affect override determination. Promote completes
		// normally and the honored event emits because the annotation
		// is present.
		kk.PrependReactor("get", "deployments", func(action ktAction) (bool, ktObject, error) {
			return true, nil, errFakeTransient
		})
		return &structs.App{Name: "app1", Release: "release1"}, &structs.Release{Id: "release1", App: "app1"}, ss
	})
	require.NoError(t, err, "transient `get deployments` error must NOT cause promote failure")
	honored := findAllByAction(events, "app:scale-override:honored")
	require.Lenf(t, honored, 1, "with annotation present and pss-driven override, exactly one honored event emits")
}
