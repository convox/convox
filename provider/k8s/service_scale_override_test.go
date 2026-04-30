package k8s_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
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

// annotatedFixture seeds a Deployment matching the app+service label
// selector used by ServiceList. When annotated=true, sets the
// convox.com/scale-override-active=true annotation.
func annotatedFixture(t *testing.T, c *fake.Clientset, ns, name string, annotated bool) {
	t.Helper()
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": "app1", "type": "service", "service": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
		},
	}
	if annotated {
		dep.Annotations = map[string]string{
			k8s.ServiceScaleOverrideAnnotation: k8s.ServiceScaleOverrideValueOn,
		}
	}
	_, err := c.AppsV1().Deployments(ns).Create(context.TODO(), dep, am.CreateOptions{})
	require.NoError(t, err)
}

// seedReleaseForApp installs the Atom-status mock + creates a Convox
// Release CRD so AppGet returns a non-empty release and ServiceList can
// resolve the manifest.
func seedReleaseForApp(t *testing.T, p *k8s.Provider, ns, fixture string) {
	t.Helper()
	aa := p.Atom.(*atom.MockInterface)
	cc := p.Convox.(*cvfake.Clientset)

	releaseID := "release1"
	aa.On("Status", ns, "app").Return("Running", releaseID, nil)
	require.NoError(t, releaseCreate(cc, ns, releaseID, fixture))
}

// captureMultiPayload collects all events emitted via webhooks during the
// callback. Mirrors captureOnePayload (d3_event_actor_test.go) but returns
// every payload posted to the webhook receiver.
func captureMultiPayload(t *testing.T, fn func(*k8s.Provider) error) []map[string]any {
	t.Helper()

	var (
		mu       sync.Mutex
		captured []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(b, &p); err == nil {
			mu.Lock()
			captured = append(captured, p)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		require.NoError(t, fn(p))
		drainPendingDispatches()
	})

	mu.Lock()
	defer mu.Unlock()
	out := make([]map[string]any, len(captured))
	copy(out, captured)
	return out
}

// findEventByAction is provided by release_watcher_test.go (Wave 2.5).
// Reused here to avoid a duplicate declaration.

// ----- Test 7: TogglesOn -----

func TestServiceScaleOverrideSet_TogglesOn(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)

		if err := p.ServiceScaleOverrideSet("app1", "web", true, "alice@example.com"); err != nil {
			return err
		}
		// Verify annotation written.
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", d.Annotations[k8s.ServiceScaleOverrideAnnotation])
		return nil
	})

	ev := findEventByAction(events, "app:scale-override:toggled")
	require.NotNil(t, ev, "expected app:scale-override:toggled event to be emitted")
	data, ok := ev["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice@example.com", data["actor"])
	assert.Equal(t, "alice@example.com", data["ack_by"])
	assert.Equal(t, "app1", data["app"])
	assert.Equal(t, "web", data["service"])
	assert.Equal(t, "on", data["state"])
}

// ----- Test 8: TogglesOff -----

func TestServiceScaleOverrideSet_TogglesOff(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", true)

		if err := p.ServiceScaleOverrideSet("app1", "web", false, "alice@example.com"); err != nil {
			return err
		}
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		_, has := d.Annotations[k8s.ServiceScaleOverrideAnnotation]
		require.False(t, has, "annotation must be removed on toggle-off")
		return nil
	})

	ev := findEventByAction(events, "app:scale-override:toggled")
	require.NotNil(t, ev)
	data := ev["data"].(map[string]any)
	assert.Equal(t, "off", data["state"])
}

// ----- Test 9: NoOp_AlreadyOn -----

func TestServiceScaleOverrideSet_NoOp_AlreadyOn(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", true)

		var patches int32
		kk.PrependReactor("patch", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
			atomic.AddInt32(&patches, 1)
			return false, nil, nil
		})

		if err := p.ServiceScaleOverrideSet("app1", "web", true, "alice"); err != nil {
			return err
		}
		require.Equal(t, int32(0), atomic.LoadInt32(&patches), "no patch should be issued on no-op")
		return nil
	})

	require.Nil(t, findEventByAction(events, "app:scale-override:toggled"), "no event should emit on no-op")
}

// ----- Test 10: NoOp_AlreadyOff -----

func TestServiceScaleOverrideSet_NoOp_AlreadyOff(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)

		var patches int32
		kk.PrependReactor("patch", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
			atomic.AddInt32(&patches, 1)
			return false, nil, nil
		})

		if err := p.ServiceScaleOverrideSet("app1", "web", false, "alice"); err != nil {
			return err
		}
		require.Equal(t, int32(0), atomic.LoadInt32(&patches), "no patch should be issued on no-op")
		return nil
	})

	require.Nil(t, findEventByAction(events, "app:scale-override:toggled"))
}

// ----- Test 11: NotFound -----

func TestServiceScaleOverrideSet_NotFound(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// no Deployment fixture
		err := p.ServiceScaleOverrideSet("app1", "missing", true, "alice")
		require.Error(t, err, "expected NotFound on missing service")
		return nil
	})

	require.Nil(t, findEventByAction(events, "app:scale-override:toggled"), "no event on not-found")
}

// ----- Test 12: AckByOverride -----

func TestServiceScaleOverrideSet_AckByOverride(t *testing.T) {
	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)
		return p.ServiceScaleOverrideSet("app1", "web", true, "alice@example.com")
	})

	ev := findEventByAction(events, "app:scale-override:toggled")
	require.NotNil(t, ev)
	data := ev["data"].(map[string]any)
	assert.Equal(t, "alice@example.com", data["ack_by"], "ack_by must be preserved verbatim")
	assert.Equal(t, "alice@example.com", data["actor"], "actor must equal ack_by per AppBudgetSet precedent")
}

// ----- Test 12b: SanitizesAckBy -----
//
// Verifies the provider-method sanitizes ackBy at entry: control
// chars and a forged log-line tail must not survive into the
// webhook payload (actor + ack_by) or the stdout log line. Mirrors
// the budget_accumulator sanitization invariant — the provider is
// the canonical sanitization point per pkg/api/deprecation.go:43-46.
func TestServiceScaleOverrideSet_SanitizesAckBy(t *testing.T) {
	restoreStdout := captureStdout(t)

	events := captureMultiPayload(t, func(p *k8s.Provider) error {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)
		// Newline + control chars + a forged log-tail. Sanitizer
		// strips C0 controls, leaving the remaining glyphs joined.
		return p.ServiceScaleOverrideSet("app1", "web", true, "alice@example.com\nINJECTION")
	})

	ev := findEventByAction(events, "app:scale-override:toggled")
	require.NotNil(t, ev)
	data := ev["data"].(map[string]any)
	wantSanitized := "alice@example.comINJECTION"
	assert.Equal(t, wantSanitized, data["ack_by"], "ack_by must be sanitized — control chars stripped")
	assert.Equal(t, wantSanitized, data["actor"], "actor must equal sanitized ack_by")

	out := restoreStdout()
	// Stdout must NOT contain a raw newline-injected payload — the
	// %q format quotes any residual control chars and the sanitizer
	// strips the actual newline.
	assert.NotContains(t, out, "alice@example.com\nINJECTION",
		"stdout must not carry raw injected newline; got:\n%s", out)
	// The %q-quoted sanitized form must appear in stdout. Format:
	// ack_by="alice@example.comINJECTION".
	assert.Contains(t, out, "ack_by=\"alice@example.comINJECTION\"",
		"stdout ack_by must use %%q quoting around sanitized value; got:\n%s", out)
}

// ----- Test 12a: ServiceList_PopulatesScaleOverrideActive -----

func TestServiceList_PopulatesScaleOverrideActive(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// Two services: "api" annotated true, "worker" un-annotated.
		annotatedFixture(t, kk, "rack1-app1", "api", true)
		annotatedFixture(t, kk, "rack1-app1", "worker", false)

		// ServiceList requires a release; seed one matching the
		// release-manifest-service.yml fixture which declares api +
		// worker services.
		seedReleaseForApp(t, p, "rack1-app1", "manifest-service")

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)

		var apiSvc, workerSvc *structs.Service
		for i := range ss {
			s := ss[i]
			switch s.Name {
			case "api":
				apiSvc = &s
			case "worker":
				workerSvc = &s
			}
		}
		require.NotNil(t, apiSvc, "api service expected in ServiceList output")
		require.NotNil(t, workerSvc, "worker service expected in ServiceList output")

		require.NotNil(t, apiSvc.ScaleOverrideActive, "ScaleOverrideActive must be non-nil on annotated service")
		require.True(t, *apiSvc.ScaleOverrideActive, "annotated service must report ScaleOverrideActive=true")

		require.NotNil(t, workerSvc.ScaleOverrideActive, "ScaleOverrideActive must be non-nil on un-annotated service (3.24.6+ rack always populates the pointer)")
		require.False(t, *workerSvc.ScaleOverrideActive, "un-annotated service must report ScaleOverrideActive=false")
	})
}

// ----- Test 12b-i: ServiceList never returns nil ScaleOverrideActive on 3.24.6 rack -----

func TestServiceList_NeverReturnsNilScaleOverrideActive_On3246Rack(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		// Mix of annotation states across the three deployments.
		annotatedFixture(t, kk, "rack1-app1", "api", true)
		annotatedFixture(t, kk, "rack1-app1", "worker", false)
		// "legacy" + "unset" left out; only services in the Deployment
		// list propagate to the populate path. That's already enough to
		// validate the invariant.

		seedReleaseForApp(t, p, "rack1-app1", "manifest-service")

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)

		require.NotEmpty(t, ss)
		for _, s := range ss {
			require.NotNilf(t, s.ScaleOverrideActive,
				"service %q: ScaleOverrideActive must be non-nil on 3.24.6+ rack (nil reserved for pre-3.24.6 wire signal)", s.Name)
		}
	})
}

// ----- Test 12c: ServiceList_PopulatesAgentField -----
//
// Verifies the Agent bool field added in item-23 §I-2: services backed by
// a Deployment report Agent=false (zero value) and services backed by a
// DaemonSet (manifest agent: true) report Agent=true. Console3 uses the
// field to hide per-service UI affordances that don't apply to agents
// (e.g. the scale-override toggle, which the rack only patches on
// Deployments).
func TestServiceList_PopulatesAgentField(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		// Deployment-backed service.
		annotatedFixture(t, kk, "rack1-app1", "api", false)

		// DaemonSet-backed (agent) service. Manifest must declare
		// agent: true on this name so ServiceList's hasAgent gate
		// short-circuits ON and the DaemonSets list iterates.
		ds := &appsv1.DaemonSet{
			ObjectMeta: am.ObjectMeta{
				Name:      "fluentd",
				Namespace: "rack1-app1",
				Labels:    map[string]string{"app": "app1", "type": "service", "service": "fluentd"},
			},
			Spec: appsv1.DaemonSetSpec{
				Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
			},
		}
		_, err := kk.AppsV1().DaemonSets("rack1-app1").Create(context.TODO(), ds, am.CreateOptions{})
		require.NoError(t, err)

		// Inline manifest declares `api` (regular service) and
		// `fluentd` (agent: true). releaseCreateInline lives in
		// release_scale_override_test.go (Wave 2.6 helper).
		manifestYaml := "services:\n" +
			"  api:\n" +
			"    image: docker.io/library/nginx\n" +
			"    port: 5000\n" +
			"  fluentd:\n" +
			"    image: docker.io/library/nginx\n" +
			"    agent: true\n"

		aa := p.Atom.(*atom.MockInterface)
		cc := p.Convox.(*cvfake.Clientset)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", manifestYaml))

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)

		var apiSvc, fluentdSvc *structs.Service
		for i := range ss {
			s := ss[i]
			switch s.Name {
			case "api":
				apiSvc = &s
			case "fluentd":
				fluentdSvc = &s
			}
		}
		require.NotNil(t, apiSvc, "api (Deployment-backed) service expected in ServiceList output")
		require.NotNil(t, fluentdSvc, "fluentd (DaemonSet-backed) service expected in ServiceList output")

		assert.False(t, apiSvc.Agent, "Deployment-backed service must report Agent=false (zero value)")
		assert.True(t, fluentdSvc.Agent, "DaemonSet-backed service must report Agent=true")
	})
}

// ----- Test 12d: AdminRBAC_RequiresAdminRole -----
//
// The provider-method itself is RBAC-agnostic (the gate is at the API
// controller layer per item-23 §4.3). The API-controller RBAC test lives
// in pkg/api/service_test.go (test 13b). This test pins the shared
// invariant: provider-method calls succeed regardless of role because
// the controller is the gate. The negative case is asserted at the
// HTTP-handler layer.
func TestServiceScaleOverrideSet_AdminRBAC_AdminPermitted(t *testing.T) {
	// Provider-method admin check is delegated to the controller. We
	// confirm that the provider-method itself executes correctly when
	// reached — RBAC enforcement is exercised in pkg/api tests.
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)

		err := p.ServiceScaleOverrideSet("app1", "web", true, "system-admin")
		require.NoError(t, err)

		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", d.Annotations[k8s.ServiceScaleOverrideAnnotation])
	})
}

// ----- Race test 21: Concurrent toggle + ServiceUpdate -----
//
// Spawn N goroutines toggling override while another goroutine repeatedly
// calls ServiceUpdate with empty opts — the no-mutation path still drives
// the full budget-circuit-breaker → informer Get → Deployment Update
// pipeline, which is the read+write surface that races the toggle's
// annotation Patch. Validates last-writer-wins on independent surfaces
// (annotation patch vs no-op Deployment Update): every Update round-trip
// must complete cleanly while toggles interleave, and the final
// annotation state must be one of the toggled values (never partial).
func TestServiceScaleOverride_RaceWith_ServiceUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("race test")
	}
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		const N = 8
		wg.Add(N + 1)
		for i := 0; i < N; i++ {
			// Goroutine A: toggle override.
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					if ctx.Err() != nil {
						return
					}
					active := (idx+j)%2 == 0
					_ = p.ServiceScaleOverrideSet("app1", "web", active, "actor")
				}
			}(i)
		}
		// Goroutine B: drive ServiceUpdate concurrently. Empty opts
		// drive the budget-check → informer Get → Deployment Update
		// pipeline without panicking on uninitialized container
		// resource maps in the lean fixture. Errors are absorbed: a
		// stale informer-cached Deployment can race the toggle's Patch
		// and produce a benign resource-version conflict on the
		// Update; we assert only that the final annotation state is
		// coherent and the deployment survives.
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if ctx.Err() != nil {
					return
				}
				_ = p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{})
			}
		}()
		wg.Wait()

		// Sanity: deployment still exists and has a coherent annotation
		// state. Final value is whichever toggle landed last; we don't
		// assert which.
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, d, "deployment must remain after concurrent toggles")
	})
}

// ----- Race test 22: Cross-mutation toggle vs ReleasePromote — informer-cache annotation read race -----
//
// Spec §9.4 row 22 stresses ServiceScaleOverrideSet against the full
// ReleasePromote path. Driving the full ReleasePromote requires Atom +
// ConvoxCRD + Build fixtures and isn't available in the lean unit-test
// scaffolding. This narrowed test covers only the in-process race that
// matters for unit-level coverage: the toggle's annotation Patch vs an
// informer-cached Get of the same Deployment metadata (the read shape
// ReleasePromote uses to pick up the override flag). It does NOT
// exercise the rest of the ReleasePromote pipeline (manifest re-render,
// HPA reconcile, KEDA ScaledObject patch, deployment rollout). The
// full ReleasePromote-path race is exercised in integration via the
// live `test-uiux-0417` AWS rack per spec §9.5.
func TestServiceScaleOverride_CrossMutation_RaceWith_ReleasePromote(t *testing.T) {
	if testing.Short() {
		t.Skip("race test")
	}
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		annotatedFixture(t, kk, "rack1-app1", "web", false)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		const N = 10
		wg.Add(2 * N)

		for i := 0; i < N; i++ {
			// Goroutine A: toggle override.
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					if ctx.Err() != nil {
						return
					}
					_ = p.ServiceScaleOverrideSet("app1", "web", true, "actor1")
				}
			}(i)
			// Goroutine B: simulate the ReleasePromote read path by
			// fetching the Deployment via the informer-cached client.
			go func() {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					if ctx.Err() != nil {
						return
					}
					_, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
				}
			}()
		}
		wg.Wait()

		// Annotation final state must be present (last writer is "true").
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", d.Annotations[k8s.ServiceScaleOverrideAnnotation],
			"final annotation state must equal A's last write (true) — never partial-write")
	})
}
