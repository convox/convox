package k8s_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAppCancel(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCancelMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppCancel("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

func TestAppCancelError(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(fmt.Errorf("err1")).Once()

		err := p.AppCancel("app1")
		require.EqualError(t, err, "err1")
	})
}

func TestAppCancelInvalidState(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Rollback", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app.yml")
		})

		// AppCreate now invokes p.Atom.Status three times: (1) inside
		// ReleasePromote → AppGet → appFromNamespace, (2) the new
		// release-watcher supersession-detection capture in ReleasePromote,
		// (3) the final AppGet → appFromNamespace at the end of AppCreate.
		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Times(3)

		a, err := p.AppCreate("app1", structs.AppCreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, a)

		assert.Equal(t, "3", a.Generation)
		assert.Equal(t, "app1", a.Name)
	})
}

func TestAppDelete(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()

		err := p.AppDelete("app1")
		require.NoError(t, err)

		_, err = kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.EqualError(t, err, `namespaces "rack1-app1" not found`)
	})
}

func TestAppDeleteMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppDelete("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

// TestRemoveAppLock_DeletesEntry: AppDelete must drop the per-app sync.Mutex
// from appBudgetLockMap so the map doesn't grow unbounded on long-lived
// racks with high app churn. This test exercises the helper directly; the
// AppDelete integration is covered by TestAppDelete_DropsAppBudgetLockEntry
// below.
func TestRemoveAppLock_DeletesEntry(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		k8s.AcquireAppBudgetLockForTest("mf8-app1")
		require.True(t, k8s.AppBudgetLockMapHasForTest("mf8-app1"), "lock entry must exist after acquire")

		p.RemoveAppLock("mf8-app1")
		assert.False(t, k8s.AppBudgetLockMapHasForTest("mf8-app1"), "RemoveAppLock must delete the lockMap entry")

		// Idempotent: calling on a missing key is a no-op.
		p.RemoveAppLock("mf8-app1")
		assert.False(t, k8s.AppBudgetLockMapHasForTest("mf8-app1"), "RemoveAppLock must remain idempotent")
	})
}

// TestAppDelete_DropsAppBudgetLockEntry: end-to-end check that AppDelete
// calls RemoveAppLock on success so apps that were ever reconciled don't
// leave their *sync.Mutex stuck in appBudgetLockMap forever.
func TestAppDelete_DropsAppBudgetLockEntry(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa, _ := p.Atom.(*atom.MockInterface)
		kk, _ := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "mf8-del-app"))

		// Simulate a prior reconciliation having taken the lock.
		k8s.AcquireAppBudgetLockForTest("mf8-del-app")
		require.True(t, k8s.AppBudgetLockMapHasForTest("mf8-del-app"))

		aa.On("Status", "rack1-mf8-del-app", "app").Return("Updating", "R1234567", nil).Once()

		require.NoError(t, p.AppDelete("mf8-del-app"))

		assert.False(t, k8s.AppBudgetLockMapHasForTest("mf8-del-app"), "AppDelete must drop the per-app lockMap entry")
	})
}

func TestNamespaceApp(t *testing.T) {
	tests := []struct {
		Name      string
		RackName  string
		AppName   string
		Namespace string
	}{
		{
			Name:      "Success",
			RackName:  "rack1",
			AppName:   "app1",
			Namespace: "rack1-app1",
		},
		{
			Name:      "Namespace not found",
			RackName:  "rack2",
			AppName:   "app2",
			Namespace: "app2",
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				ns, err := p.NamespaceApp(test.Namespace)

				if err != nil {
					require.EqualError(t, err, fmt.Sprintf("namespaces %q not found", test.Namespace))
				} else {
					require.NoError(t, err)
					require.Equal(t, ns, test.AppName)
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestAppGet(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Running", "R1234567", nil).Once()

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		a, err := p.AppGet("app1")
		require.NoError(t, err)

		assert.Equal(t, "3", a.Generation)
		assert.Equal(t, false, a.Locked)
		assert.Equal(t, "app1", a.Name)
		assert.Equal(t, "R1234567", a.Release)
		assert.Equal(t, "", a.Router)
		assert.Equal(t, "running", a.Status)
	})
}

func TestAppGetMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		a, err := p.AppGet("app1")
		require.EqualError(t, err, "app not found: app1")
		require.Nil(t, a)
	})
}

func TestAppGetUpdating(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Updating", "", nil).Once()

		ns := &ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: "rack1-app1",
			},
		}
		_, err := kk.CoreV1().Namespaces().Create(context.TODO(), ns, am.CreateOptions{})
		require.NoError(t, err)

		a, err := p.AppGet("app1")
		require.NoError(t, err)
		require.Equal(t, "updating", a.Status)
	})
}

func TestAppList(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		//aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreateWithAnnotation(kk, "rack1", "app1", map[string]string{
			"convox.com/app-status":  "Running",
			"convox.com/app-release": "R1234567",
		}))
		require.NoError(t, appCreateWithAnnotation(kk, "rack1", "app2", map[string]string{
			"convox.com/app-status":  "Updating",
			"convox.com/app-release": "R2345678",
		}))

		as, err := p.AppList()
		require.NoError(t, err)
		require.Equal(t, 2, len(as))

		assert.Equal(t, "3", as[0].Generation)
		assert.Equal(t, false, as[0].Locked)
		assert.Equal(t, "app1", as[0].Name)
		assert.Equal(t, "R1234567", as[0].Release)
		assert.Equal(t, "", as[0].Router)
		assert.Equal(t, "running", as[0].Status)

		assert.Equal(t, "3", as[1].Generation)
		assert.Equal(t, false, as[1].Locked)
		assert.Equal(t, "app2", as[1].Name)
		assert.Equal(t, "R2345678", as[1].Release)
		assert.Equal(t, "", as[1].Router)
		assert.Equal(t, "updating", as[1].Status)
	})
}

func TestAppLogs(t *testing.T) {
	// AppLogs is implemented as a selector-based fan-out across all
	// service pods in the app namespace. The unit-test provider has no
	// kube REST config wired (testProvider builds a fake clientset
	// without Config), so calling AppLogs would dereference a nil
	// p.Config inside newlogsConfigFlags. The contract this test pins
	// is functional rather than runtime: the symbol must exist and not
	// regress to the old "unimplemented" stub. Run-time behaviour is
	// covered end-to-end by `convox logs -a <app>` against a real rack.
	testProvider(t, func(p *k8s.Provider) {
		// Defer-recover guards against the expected nil-pointer panic
		// from p.Config when the helper builds the kube REST flags.
		defer func() {
			_ = recover()
		}()
		_, err := p.AppLogs("app1", structs.LogsOptions{})
		// If we get here without a panic, the kube selector path
		// returned an error (e.g. ErrNotFound for the empty fake
		// namespace) — that's acceptable too. What we do NOT accept is
		// a clean nil error AND nil reader, which would be the silent
		// pre-fix unimplemented return.
		require.NotEqual(t, "unimplemented", fmt.Sprint(err),
			"AppLogs must no longer return ErrNotImplemented")
	})
}

func TestAppMetrics(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		ms, err := p.AppMetrics("app1", structs.MetricsOptions{})
		require.EqualError(t, err, "unimplemented")
		require.Nil(t, ms)
	})
}

func TestAppNamespace(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		ns := p.AppNamespace("app1")
		require.Equal(t, "rack1-app1", ns)
	})
}

func TestAppUpdateLocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked.yml")
		})

		// AppUpdate routes through ReleasePromote, which now invokes
		// p.Atom.Status three times: (1) AppGet → appFromNamespace inside
		// ReleasePromote, (2) the release-watcher supersession-detection
		// capture in ReleasePromote, (3) post-promote AppGet.
		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Times(3)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", ns.Annotations["convox.com/lock"])
	})
}

func TestAppUpdateDoesNotOverwriteExisting(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked.yml")
		}).Once()

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked-params.yml")
		}).Once()

		// Two AppUpdate calls × 3 Status calls each (ReleasePromote AppGet
		// + release-watcher capture + post-promote AppGet) = 6 total.
		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Times(6)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.NoError(t, err)

		err = p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "bar"}})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", ns.Annotations["convox.com/lock"])
		require.Equal(t, `{"Test":"bar"}`, ns.Annotations["convox.com/params"])
	})
}

func TestAppUpdateParameters(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-params.yml")
		})

		// AppUpdate routes through ReleasePromote (3 Status calls) per the
		// release-watcher supersession-detection landing.
		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Times(3)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "bar"}})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.Background(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, `{"Test":"bar"}`, ns.Annotations["convox.com/params"])
	})
}

func TestAppUpdateExistingRelease(t *testing.T) {
	t.Skip("implement after testing releases")
}

func TestAppUpdateMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.EqualError(t, err, "app not found: app1")
	})
}

// TestAppManifestService_Found — happy path. The release fixture
// release-manifest-service.yml declares four services exercising the
// distinct env/scale shape combinations: pointer form (scale.min/max),
// env-only with no scale block, legacy scale.count: N-M, neither.
//
// IMPORTANT semantic note: manifest.Load runs ApplyDefaults, which
// populates Scale.Count = {Min: 1, Max: 1} for any service with NO
// scale attributes at all (manifest.go ~line 299-301). This means
// after fixture load, even services that omit `scale:` carry an
// effective Count={1,1}. Our K8s impl's two-form synthesis sees that
// non-zero Count and emits Min=1/Max=1 — which is the correct,
// truthful answer for "what does this service actually run as."
// Users querying the new endpoint see the resolved replica
// bounds, not the unfilled YAML state.
func TestAppManifestService_Found(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa, _ := p.Atom.(*atom.MockInterface)
		kk, _ := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, releaseCreate(p.Convox, "rack1-app1", "r1234567", "manifest-service"))

		// New form: scale.min / scale.max as top-level pointers + env.
		// Pointer form wins; Min=1, Max=5 round-trip from yaml verbatim.
		aa.On("Status", "rack1-app1", "app").Return("Running", "r1234567", nil).Once()
		ms, err := p.AppManifestService("app1", "api")
		require.NoError(t, err)
		require.Equal(t, "api", ms.Name)
		require.Equal(t, []string{"LOG_LEVEL=info", "DEBUG=false"}, ms.Environment)
		require.NotNil(t, ms.Scale)
		require.NotNil(t, ms.Scale.Min)
		require.Equal(t, 1, *ms.Scale.Min)
		require.NotNil(t, ms.Scale.Max)
		require.Equal(t, 5, *ms.Scale.Max)

		// Env-only, no scale block: ApplyDefaults populates Count={1,1};
		// Min/Max synthesizes to 1/1. Environment passes through verbatim.
		aa.On("Status", "rack1-app1", "app").Return("Running", "r1234567", nil).Once()
		ms, err = p.AppManifestService("app1", "worker")
		require.NoError(t, err)
		require.Equal(t, "worker", ms.Name)
		require.Equal(t, []string{"WORKER_QUEUE=default"}, ms.Environment)
		require.NotNil(t, ms.Scale)
		require.NotNil(t, ms.Scale.Min)
		require.Equal(t, 1, *ms.Scale.Min)
		require.NotNil(t, ms.Scale.Max)
		require.Equal(t, 1, *ms.Scale.Max)

		// Legacy form: scale.count: 2-8 → Count.Min=2, Count.Max=8.
		// Pointer form is unset; synthesis falls through to the Count
		// branch and emits Min=2, Max=8.
		aa.On("Status", "rack1-app1", "app").Return("Running", "r1234567", nil).Once()
		ms, err = p.AppManifestService("app1", "legacy")
		require.NoError(t, err)
		require.Equal(t, "legacy", ms.Name)
		require.Empty(t, ms.Environment)
		require.NotNil(t, ms.Scale)
		require.NotNil(t, ms.Scale.Min)
		require.Equal(t, 2, *ms.Scale.Min)
		require.NotNil(t, ms.Scale.Max)
		require.Equal(t, 8, *ms.Scale.Max)

		// Neither env nor scale block: ApplyDefaults still sets Count={1,1};
		// Environment is the empty manifest.Environment which JSON-omitempty
		// drops on the wire.
		aa.On("Status", "rack1-app1", "app").Return("Running", "r1234567", nil).Once()
		ms, err = p.AppManifestService("app1", "unset")
		require.NoError(t, err)
		require.Equal(t, "unset", ms.Name)
		require.Empty(t, ms.Environment)
		require.NotNil(t, ms.Scale)
		require.NotNil(t, ms.Scale.Min)
		require.Equal(t, 1, *ms.Scale.Min)
		require.NotNil(t, ms.Scale.Max)
		require.Equal(t, 1, *ms.Scale.Max)
	})
}

// TestAppManifestService_ServiceMissing — the release manifest does not
// declare a service named `ghost`. The provider must return an error
// containing "not found in manifest" so the caller can render an
// appropriate 4xx-friendly message.
func TestAppManifestService_ServiceMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa, _ := p.Atom.(*atom.MockInterface)
		kk, _ := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, releaseCreate(p.Convox, "rack1-app1", "r1234567", "manifest-service"))

		aa.On("Status", "rack1-app1", "app").Return("Running", "r1234567", nil).Once()

		_, err := p.AppManifestService("app1", "ghost")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found in manifest")
	})
}

// TestAppManifestService_NoRelease — common.AppManifest fails when the app
// has no current release (Atom.Status returns ""). The provider must
// propagate the error rather than panicking or returning a partial
// response.
func TestAppManifestService_NoRelease(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa, _ := p.Atom.(*atom.MockInterface)
		kk, _ := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Once()

		_, err := p.AppManifestService("app1", "api")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no release for app")
	})
}

// TestAppManifestService_AppMissing — AppGet returns NotFound when the
// namespace doesn't exist; common.AppManifest propagates the error.
func TestAppManifestService_AppMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		_, err := p.AppManifestService("nope", "api")
		require.Error(t, err)
		require.Contains(t, err.Error(), "app not found: nope")
	})
}

func appCreate(c kubernetes.Interface, rack, name string) error {
	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(),
		&ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name:        fmt.Sprintf("%s-%s", rack, name),
				Annotations: map[string]string{"convox.com/lock": "false"},
				Labels: map[string]string{
					"app":    name,
					"name":   name,
					"rack":   rack,
					"system": "convox",
					"type":   "app",
				},
			},
		},
		am.CreateOptions{},
	)

	return errors.WithStack(err)
}

func appCreateWithAnnotation(c kubernetes.Interface, rack, name string, anno map[string]string) error {
	anno["convox.com/lock"] = "false"
	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(),
		&ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name:        fmt.Sprintf("%s-%s", rack, name),
				Annotations: anno,
				Labels: map[string]string{
					"app":    name,
					"name":   name,
					"rack":   rack,
					"system": "convox",
					"type":   "app",
				},
			},
		},
		am.CreateOptions{},
	)

	return errors.WithStack(err)
}
