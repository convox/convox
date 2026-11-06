package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func svcWithDeployment(name string, d manifest.ServiceDeployment) manifest.Service {
	return manifest.Service{Name: name, Deployment: d}
}

func TestEffectiveProgressDeadline(t *testing.T) {
	for _, c := range []struct {
		name        string
		service     int
		rackDefault int
		expect      int
	}{
		{"nothing configured stays unset", 0, 0, 0},
		{"rack default applies", 0, 900, 900},
		{"service value wins", 600, 900, 600},
		{"below the floor is clamped up", 0, 5, manifest.ProgressDeadlineFloor},
		{"a negative rack value is clamped up", 0, -1, manifest.ProgressDeadlineFloor},
		{"above the ceiling is clamped down", 99999, 0, manifest.ProgressDeadlineCeiling},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := svcWithDeployment("web", manifest.ServiceDeployment{ProgressDeadline: c.service})
			assert.Equal(t, c.expect, k8s.EffectiveProgressDeadlineForTest(&s, c.rackDefault))
		})
	}
}

func TestEffectiveCrashRestartLimit(t *testing.T) {
	for _, c := range []struct {
		name        string
		service     int
		rackDefault int
		expect      int
	}{
		{"nothing configured is off", 0, 0, 0},
		{"rack default applies", 0, 3, 3},
		{"service value wins", 5, 3, 5},
		{"minus one opts a service out of the rack default", -1, 3, -1},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := svcWithDeployment("web", manifest.ServiceDeployment{CrashRestartLimit: c.service})
			assert.Equal(t, c.expect, k8s.EffectiveCrashRestartLimitForTest(&s, c.rackDefault))
		})
	}
}

func TestFastFailStateForPromote(t *testing.T) {
	t.Run("nothing configured populates nothing", func(t *testing.T) {
		p := &k8s.Provider{}
		deadlines, limits, maxDeadline := k8s.FastFailStateForPromoteForTest(p, manifest.Services{
			svcWithDeployment("web", manifest.ServiceDeployment{}),
		})
		assert.Empty(t, deadlines, "an empty map is what keeps the detector from listing every tick")
		assert.Empty(t, limits)
		assert.Equal(t, 0, maxDeadline)
	})

	t.Run("rack params alone arm both detectors", func(t *testing.T) {
		p := &k8s.Provider{DeployProgressDeadline: 600, DeployCrashRestartLimit: 3}
		deadlines, limits, _ := k8s.FastFailStateForPromoteForTest(p, manifest.Services{
			svcWithDeployment("web", manifest.ServiceDeployment{}),
		})
		assert.Equal(t, map[string]int{"web": 600}, deadlines)
		assert.Equal(t, map[string]int{"web": 3}, limits)
	})

	t.Run("a deadline at or above the shipped default arms nothing", func(t *testing.T) {
		p := &k8s.Provider{}
		deadlines, _, maxDeadline := k8s.FastFailStateForPromoteForTest(p, manifest.Services{
			svcWithDeployment("web", manifest.ServiceDeployment{ProgressDeadline: manifest.DefaultProgressDeadlineSeconds}),
			svcWithDeployment("slow", manifest.ServiceDeployment{ProgressDeadline: 6000}),
		})
		assert.Empty(t, deadlines)
		assert.Equal(t, 6000, maxDeadline, "the Atom has to allow what the Deployment was rendered with")
	})

	t.Run("agents get no deadline entry and do not move the Atom deadline", func(t *testing.T) {
		s := svcWithDeployment("logs", manifest.ServiceDeployment{ProgressDeadline: 21600, CrashRestartLimit: 2})
		s.Agent.Enabled = true
		deadlines, limits, maxDeadline := k8s.FastFailStateForPromoteForTest(&k8s.Provider{}, manifest.Services{s})
		assert.Empty(t, deadlines)
		assert.Equal(t, 0, maxDeadline)
		assert.Equal(t, map[string]int{"logs": 2}, limits, "crash limits do match agent pods")
	})

	t.Run("stateful services get no deadline entry but do move the Atom deadline", func(t *testing.T) {
		s := svcWithDeployment("db", manifest.ServiceDeployment{ProgressDeadline: 21600, CrashRestartLimit: 2})
		s.Stateful = true
		deadlines, limits, maxDeadline := k8s.FastFailStateForPromoteForTest(&k8s.Provider{}, manifest.Services{s})
		assert.Empty(t, deadlines, "no StatefulSet ever carries progressDeadlineSeconds")
		assert.Equal(t, manifest.ProgressDeadlineCeiling, maxDeadline, "the Atom deadline is the only clock they have")
		assert.Equal(t, map[string]int{"db": 2}, limits)
	})

	t.Run("a service level minus one opts out while siblings stay armed", func(t *testing.T) {
		p := &k8s.Provider{DeployCrashRestartLimit: 3}
		_, limits, _ := k8s.FastFailStateForPromoteForTest(p, manifest.Services{
			svcWithDeployment("web", manifest.ServiceDeployment{}),
			svcWithDeployment("worker", manifest.ServiceDeployment{CrashRestartLimit: -1}),
		})
		assert.Equal(t, map[string]int{"web": 3}, limits)
	})
}

func progressingDeployment(service, release, reason string, gen, observed int64, updated time.Time) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:       service,
			Generation: gen,
			Labels:     map[string]string{"service": service, "release": release},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: observed,
			Conditions: []appsv1.DeploymentCondition{{
				Type:           appsv1.DeploymentProgressing,
				Status:         corev1.ConditionFalse,
				Reason:         reason,
				LastUpdateTime: am.NewTime(updated),
			}},
		},
	}
}

func TestDeadlineExceededService(t *testing.T) {
	started := time.Now().UTC()
	opted := map[string]int{"web": 120}

	t.Run("a fresh generation-current verdict bails", func(t *testing.T) {
		deps := []appsv1.Deployment{progressingDeployment("web", "R1", "ProgressDeadlineExceeded", 3, 3, started.Add(time.Minute))}
		assert.Equal(t, "web", k8s.DeadlineExceededServiceForTest(deps, opted, started))
	})

	t.Run("a stale status does not bail", func(t *testing.T) {
		deps := []appsv1.Deployment{progressingDeployment("web", "R1", "ProgressDeadlineExceeded", 4, 3, started.Add(time.Minute))}
		assert.Equal(t, "", k8s.DeadlineExceededServiceForTest(deps, opted, started))
	})

	t.Run("a verdict older than this promote does not bail", func(t *testing.T) {
		deps := []appsv1.Deployment{progressingDeployment("web", "R1", "ProgressDeadlineExceeded", 3, 3, started.Add(-time.Minute))}
		assert.Equal(t, "", k8s.DeadlineExceededServiceForTest(deps, opted, started))
	})

	t.Run("a service that did not opt in does not bail", func(t *testing.T) {
		deps := []appsv1.Deployment{progressingDeployment("worker", "R1", "ProgressDeadlineExceeded", 3, 3, started.Add(time.Minute))}
		assert.Equal(t, "", k8s.DeadlineExceededServiceForTest(deps, opted, started))
	})

	t.Run("another reason does not bail", func(t *testing.T) {
		deps := []appsv1.Deployment{progressingDeployment("web", "R1", "ReplicaSetUpdated", 3, 3, started.Add(time.Minute))}
		assert.Equal(t, "", k8s.DeadlineExceededServiceForTest(deps, opted, started))
	})

	t.Run("a healthy service does not mask a failed one", func(t *testing.T) {
		deps := []appsv1.Deployment{
			progressingDeployment("worker", "R1", "ReplicaSetUpdated", 3, 3, started.Add(time.Minute)),
			progressingDeployment("api", "R1", "ReplicaSetUpdated", 3, 3, started.Add(time.Minute)),
			progressingDeployment("web", "R1", "ProgressDeadlineExceeded", 3, 3, started.Add(time.Minute)),
		}
		assert.Equal(t, "web", k8s.DeadlineExceededServiceForTest(deps, map[string]int{"web": 120, "worker": 120, "api": 120}, started))
	})
}

func podWithRestarts(service string, start *time.Time, terminating bool, initRestarts, mainRestarts []int32) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: am.ObjectMeta{Name: service + "-1", Labels: map[string]string{"service": service}},
	}
	if start != nil {
		pod.Status.StartTime = &am.Time{Time: *start}
	}
	if terminating {
		now := am.Now()
		pod.DeletionTimestamp = &now
	}
	for _, r := range initRestarts {
		pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{RestartCount: r})
	}
	for _, r := range mainRestarts {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{RestartCount: r})
	}
	return pod
}

func TestCrashRestartExceeded(t *testing.T) {
	started := time.Now().UTC()
	fresh := started.Add(time.Minute)
	stale := started.Add(-time.Minute)
	limits := map[string]int{"web": 5}

	t.Run("over the limit bails", func(t *testing.T) {
		svc, restarts, limit := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &fresh, false, nil, []int32{6})}, limits, started)
		assert.Equal(t, "web", svc)
		assert.Equal(t, 6, restarts)
		assert.Equal(t, 5, limit)
	})

	t.Run("exactly at the limit does not bail", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &fresh, false, nil, []int32{5})}, limits, started)
		assert.Equal(t, "", svc)
	})

	t.Run("restarts are the max across containers, never the sum", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &fresh, false, nil, []int32{3, 3})}, limits, started)
		assert.Equal(t, "", svc)
	})

	t.Run("an init container over the limit bails with no main statuses", func(t *testing.T) {
		svc, restarts, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &fresh, false, []int32{7}, nil)}, limits, started)
		assert.Equal(t, "web", svc)
		assert.Equal(t, 7, restarts)
	})

	t.Run("a pod predating this promote does not bail", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &stale, false, nil, []int32{99})}, limits, started)
		assert.Equal(t, "", svc, "an AppUpdate re-promote leaves existing pods and their restarts in place")
	})

	t.Run("a terminating pod does not bail", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", &fresh, true, nil, []int32{99})}, limits, started)
		assert.Equal(t, "", svc)
	})

	t.Run("a pod with no start time is treated as this promote's", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("web", nil, false, nil, []int32{99})}, limits, started)
		assert.Equal(t, "web", svc, "a nil StartTime is treated as this promote's")
	})

	t.Run("a pod with no start time does not hide a sibling over its limit", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest([]corev1.Pod{
			podWithRestarts("worker", nil, false, nil, nil),
			podWithRestarts("web", &fresh, false, nil, []int32{9}),
		}, limits, started)
		assert.Equal(t, "web", svc)
	})

	t.Run("a service with no limit does not bail", func(t *testing.T) {
		svc, _, _ := k8s.CrashRestartExceededForTest(
			[]corev1.Pod{podWithRestarts("worker", &fresh, false, nil, []int32{99})}, limits, started)
		assert.Equal(t, "", svc)
	})
}

func TestIsRollbackStatus(t *testing.T) {
	for _, s := range []string{"Cancelled", "Deadline", "Error", "Rollback", "Reverted", "Failure"} {
		assert.True(t, k8s.IsRollbackStatusForTest(s), s)
	}
	for _, s := range []string{"Running", "Success", "Pending", "Updating", ""} {
		assert.False(t, k8s.IsRollbackStatusForTest(s), s)
	}
}

// Driven from the producer rather than a copy of the override list, so renaming
// a mapper message fails here instead of silently detaching the bail reason.
func TestBailReasonOverridesError(t *testing.T) {
	for _, atomStatus := range []string{"Failure", "Reverted", "Cancelled", "Deadline", "Error", "Rollback"} {
		_, errMsg, terminal := k8s.MapAppStatusToWatchResultForTest(atomStatus)
		require.True(t, terminal, atomStatus)
		assert.True(t, k8s.BailReasonOverridesErrorForTest(errMsg), "%s maps to %q", atomStatus, errMsg)
	}
	for _, s := range []string{"watcher-panic: boom", "superseded-by-newer-promote", "watcher-timeout", ""} {
		assert.False(t, k8s.BailReasonOverridesErrorForTest(s), s)
	}
}

// The Atom's rollback swaps CurrentVersion to the previous release before the
// controller writes the status, so the app-release mirror names the prior
// release for the whole rollback family. Without the PriorRelease guard every
// failed deploy would report itself as superseded.
func TestReleasePromoteWatcher_OwnRollbackIsNotSupersession(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(100 * time.Millisecond)()

	for atomStatus, expect := range map[string]string{
		"Rollback": "rollback: Rollback",
		"Reverted": "rollout-failed: Reverted",
		"Deadline": "deadline-exceeded",
	} {
		t.Run(atomStatus, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				app := "appRollback" + atomStatus
				seedAppNamespaceWithStatus(t, p, app, atomStatus, "RPRIOR")

				state := structs.ReleasePromoteWatchState{
					SchemaVersion: 1,
					ReleaseID:     "RNEW",
					AtomVersion:   "RNEW",
					PriorRelease:  "RPRIOR",
					StartedAt:     time.Now().UTC().Add(-10 * time.Second),
					ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
					Actor:         "alice@example.com",
				}

				events := captureReleaseWatcherEvents(t, p, func() {
					runWatcherWithAnnotation(t, p, app, &state)
				}, 1, 2*time.Second)

				assert.Nil(t, findEventByAction(events, "app:promote:cancelled"),
					"a rollback of our own release is not a supersession")
				ev := findEventByAction(events, "app:promote:errored")
				require.NotNil(t, ev, "expected app:promote:errored; got %v", events)
				data, _ := ev["data"].(map[string]any)
				assert.Equal(t, expect, data["message"])
			})
		})
	}
}

func TestReleasePromoteWatcher_ThirdReleaseStillSupersedes(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appThird", "Updating", "RTHIRD")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "RNEW",
			AtomVersion:   "RNEW",
			PriorRelease:  "RPRIOR",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, "appThird", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:cancelled")
		require.NotNil(t, ev, "expected app:promote:cancelled; got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "superseded-by-newer-promote", data["message"])
	})
}

// A mirror that never names this promote must not let the previous rollout's
// status through, and must still terminate rather than spinning forever.
func TestReleasePromoteWatcher_MirrorNeverCatchesUp(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(200 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appLag", "Running", "RPRIOR")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "RNEW",
			AtomVersion:   "RNEW",
			PriorRelease:  "RPRIOR",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(50 * time.Millisecond),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, "appLag", &state)
		}, 1, 3*time.Second)

		assert.Nil(t, findEventByAction(events, "app:promote:completed"),
			"the previous rollout's Running must not be reported as this promote's success")
		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "watcher must still terminate; got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"])
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest("appLag", "RNEW"),
			"a latch that never opens must not leak the inflight slot")
	})
}

func TestEmitReleasePromoteResult_BailReason(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		state := structs.ReleasePromoteWatchState{SchemaVersion: 1, ReleaseID: "R1", Actor: "alice@example.com"}
		reason := "crash-restart-limit: service=web restarts=6 limit=5"

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.EmitReleasePromoteResultForTest(p, "appBail", &state, "error", "cancelled", reason)
		}, 1, time.Second)
		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, reason, data["message"], "the generic Atom status must be replaced by the bail reason")

		events = captureReleaseWatcherEvents(t, p, func() {
			k8s.EmitReleasePromoteResultForTest(p, "appBail", &state, "success", "", reason)
		}, 1, time.Second)
		ev = findEventByAction(events, "app:promote:completed")
		require.NotNil(t, ev)
		data, _ = ev["data"].(map[string]any)
		assert.Nil(t, data["message"], "a bail reason must never ride a successful promote")

		events = captureReleaseWatcherEvents(t, p, func() {
			k8s.EmitReleasePromoteResultForTest(p, "appBail", &state, "error", "watcher-panic: boom", reason)
		}, 1, time.Second)
		ev = findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev)
		data, _ = ev["data"].(map[string]any)
		assert.Equal(t, "watcher-panic: boom", data["message"], "a panic message is more specific than the bail reason")
	})
}

func seedFailedDeployment(t *testing.T, p *k8s.Provider, app, service, release string, startedAt time.Time) {
	t.Helper()
	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)
	d := progressingDeployment(service, release, "ProgressDeadlineExceeded", 1, 1, startedAt.Add(time.Second))
	d.Labels["app"] = app
	d.Labels["type"] = "service"
	d.Namespace = fmt.Sprintf("%s-%s", p.Name, app)
	_, err := kk.AppsV1().Deployments(d.Namespace).Create(context.TODO(), &d, am.CreateOptions{})
	require.NoError(t, err)
}

func TestReleasePromoteWatcher_DeadlineBailCancelsOnce(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(200 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		app := "appDeadlineBail"
		seedAppNamespaceWithStatus(t, p, app, "Updating", "RNEW")

		ns := fmt.Sprintf("%s-%s", p.Name, app)
		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)
		aa.On("Cancel", ns, "app").Return(nil).Once().Run(func(mock.Arguments) {
			body := []byte(`{"metadata":{"annotations":{"convox.com/app-status":"Cancelled"}}}`)
			_, perr := kk.CoreV1().Namespaces().Patch(context.TODO(), ns, types.MergePatchType, body, am.PatchOptions{})
			require.NoError(t, perr)
		})

		started := time.Now().UTC()
		seedFailedDeployment(t, p, app, "web", "RNEW", started)

		state := structs.ReleasePromoteWatchState{
			SchemaVersion:     1,
			ReleaseID:         "RNEW",
			AtomVersion:       "RNEW",
			StartedAt:         started,
			ExpiresAt:         started.Add(300 * time.Millisecond),
			Actor:             "alice@example.com",
			ProgressDeadlines: map[string]int{"web": 120},
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, app, &state)
		}, 1, 3*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "progress-deadline-exceeded: service=web", data["message"])
		aa.AssertNumberOfCalls(t, "Cancel", 1)
	})
}

func TestReleasePromoteWatcher_KillSwitchDisablesDetectionButStillEmits(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(200 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		app := "appKillSwitch"
		seedAppNamespaceWithStatus(t, p, app, "Updating", "RNEW")
		p.FeatureGates = map[string]bool{options.FeatureGateDeployFastFailDisable: true}

		started := time.Now().UTC()
		seedFailedDeployment(t, p, app, "web", "RNEW", started)

		state := structs.ReleasePromoteWatchState{
			SchemaVersion:     1,
			ReleaseID:         "RNEW",
			AtomVersion:       "RNEW",
			StartedAt:         started,
			ExpiresAt:         started.Add(150 * time.Millisecond),
			Actor:             "alice@example.com",
			ProgressDeadlines: map[string]int{"web": 120},
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, app, &state)
		}, 1, 3*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "the gate must never skip the terminal emit; got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"], "no bail, so the watcher runs out its own clock")

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		require.NoError(t, err)
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"the gate must never skip the annotation cleanup either")
	})
}

func renderFastFailService(t *testing.T, rackDeadline int, serviceYaml string) string {
	t.Helper()
	var out []byte
	testProvider(t, func(p *k8s.Provider) {
		p.DeployProgressDeadline = rackDeadline
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		yml := "services:\n" + serviceYaml
		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)
		cc, _ := p.Convox.(*cvfake.Clientset)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", yml))

		m, err := manifest.Load([]byte(yml), structs.Environment{})
		require.NoError(t, err)

		data, err := k8s.ReleaseTemplateServicesForTest(p,
			&structs.App{Name: "app1", Release: "release1"}, structs.Environment{},
			&structs.Release{Id: "release1", App: "app1"}, m.Services, structs.ReleasePromoteOptions{})
		require.NoError(t, err)
		out = data
	})
	return string(out)
}

func TestRenderProgressDeadlineSeconds(t *testing.T) {
	web := "  web:\n    image: docker.io/library/nginx\n    port: 5000\n"

	t.Run("unconfigured renders no field at all", func(t *testing.T) {
		assert.NotContains(t, renderFastFailService(t, 0, web), "progressDeadlineSeconds",
			"an unconfigured rack must render byte-identically to before this change")
	})

	t.Run("the rack default renders", func(t *testing.T) {
		assert.Contains(t, renderFastFailService(t, 600, web), "progressDeadlineSeconds: 600")
	})

	t.Run("the service value wins", func(t *testing.T) {
		out := renderFastFailService(t, 600, web+"    deployment:\n      progressDeadline: 900\n")
		assert.Contains(t, out, "progressDeadlineSeconds: 900")
	})

	t.Run("an out of range value is clamped rather than rendered verbatim", func(t *testing.T) {
		out := renderFastFailService(t, -5, web)
		assert.Contains(t, out, fmt.Sprintf("progressDeadlineSeconds: %d", manifest.ProgressDeadlineFloor))
	})

	t.Run("an agent DaemonSet carries no deadline", func(t *testing.T) {
		out := renderFastFailService(t, 600, "  logs:\n    image: docker.io/library/nginx\n    agent: true\n")
		assert.Contains(t, out, "kind: DaemonSet")
		assert.NotContains(t, out, "progressDeadlineSeconds")
	})

	t.Run("a stateful StatefulSet carries no deadline", func(t *testing.T) {
		out := renderFastFailService(t, 600, "  db:\n    image: docker.io/library/nginx\n    stateful: true\n    port: 5000\n")
		assert.Contains(t, out, "kind: StatefulSet")
		assert.NotContains(t, out, "progressDeadlineSeconds")
	})
}

// The Deployment keeps its rendered deadline until the next full promote, so the
// Atom has to allow at least as long, but only when the caller named no timeout:
// AppUpdate passes 30 for what is only a metadata change.
func TestReleasePromote_AtomTimeoutCoversLongestDeadline(t *testing.T) {
	for _, c := range []struct {
		name    string
		yaml    string
		timeout *int
		expect  int32
	}{
		{"unconfigured keeps the shipped default", "", nil, manifest.DefaultProgressDeadlineSeconds},
		{"a longer deadline raises it", "    deployment:\n      progressDeadline: 6000\n", nil, 6000},
		{"a shorter deadline does not lower it", "    deployment:\n      progressDeadline: 120\n", nil, manifest.DefaultProgressDeadlineSeconds},
		{"an explicit caller timeout wins", "    deployment:\n      progressDeadline: 6000\n", options.Int(30), 30},
	} {
		t.Run(c.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*fake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))

				yml := "services:\n  web:\n    image: docker.io/library/nginx\n    port: 5000\n" + c.yaml
				cc, _ := p.Convox.(*cvfake.Clientset)
				require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", yml))
				require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

				aa, ok := p.Atom.(*atom.MockInterface)
				require.True(t, ok)
				aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

				var got int32
				aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
					cfg, ok := args.Get(2).(*atom.ApplyConfig)
					require.True(t, ok)
					got = cfg.Timeout
				})

				require.NoError(t, p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{Timeout: c.timeout}))
				assert.Equal(t, c.expect, got)
			})
		})
	}
}

// A transient failure to claim the bail must not latch, or a single apiserver
// blip permanently disables detection for that rollout.
func TestReleasePromoteWatcher_BailClaimRetriesAfterTransientError(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(300 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		app := "appBailRetry"
		seedAppNamespaceWithStatus(t, p, app, "Updating", "RNEW")

		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)
		aa.On("Cancel", fmt.Sprintf("%s-%s", p.Name, app), "app").Return(nil).Once()

		started := time.Now().UTC()
		seedFailedDeployment(t, p, app, "web", "RNEW", started)

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		var patches int32
		kk.PrependReactor("patch", "namespaces", func(a ktesting.Action) (bool, runtime.Object, error) {
			pa, ok := a.(ktesting.PatchAction)
			if !ok || pa.GetPatchType() != types.JSONPatchType {
				return false, nil, nil
			}
			// fail only the first bail claim; the delete patch runs after it
			if atomic.AddInt32(&patches, 1) == 1 {
				return true, nil, fmt.Errorf("etcdserver: request timed out")
			}
			return false, nil, nil
		})

		state := structs.ReleasePromoteWatchState{
			SchemaVersion:     1,
			ReleaseID:         "RNEW",
			AtomVersion:       "RNEW",
			StartedAt:         started,
			ExpiresAt:         started.Add(400 * time.Millisecond),
			Actor:             "alice@example.com",
			ProgressDeadlines: map[string]int{"web": 120},
		}

		captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, app, &state)
		}, 1, 3*time.Second)

		aa.AssertNumberOfCalls(t, "Cancel", 1)
		assert.Greater(t, atomic.LoadInt32(&patches), int32(1), "the failed claim must be retried")
	})
}

// A rack that has configured nothing must render byte-identically to before this
// change, which is only true while the rack default resolves to unset.
func TestRenderProgressDeadlineSecondsShippedDefault(t *testing.T) {
	var deadline int
	testProvider(t, func(p *k8s.Provider) {
		deadline = p.DeployProgressDeadline
	})
	assert.Equal(t, 0, deadline, "an api pod with no DEPLOY_PROGRESS_DEADLINE must resolve to unset")

	out := renderFastFailService(t, deadline, "  web:\n    image: docker.io/library/nginx\n    port: 5000\n")
	assert.NotContains(t, out, "progressDeadlineSeconds")
}

// The pod template renders the upper-cased release id while the watch state
// carries the raw one. Get that wrong and crash detection silently never fires.
func TestReleasePromoteWatcher_CrashRestartBailMatchesUpperCasedPods(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(200 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		app := "appCrashBail"
		ns := fmt.Sprintf("%s-%s", p.Name, app)
		seedAppNamespaceWithStatus(t, p, app, "Updating", "release2")

		started := time.Now().UTC()
		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)
		// mirror what the Atom really does, so the terminal mapping runs
		aa.On("Cancel", ns, "app").Return(nil).Once().Run(func(mock.Arguments) {
			body := []byte(`{"metadata":{"annotations":{"convox.com/app-status":"Cancelled"}}}`)
			_, perr := kk.CoreV1().Namespaces().Patch(context.TODO(), ns, types.MergePatchType, body, am.PatchOptions{})
			require.NoError(t, perr)
		})
		_, err := kk.CoreV1().Pods(ns).Create(context.TODO(), &corev1.Pod{
			ObjectMeta: am.ObjectMeta{
				Name:      "web-abc",
				Namespace: ns,
				Labels: map[string]string{
					"system": "convox", "rack": p.Name, "app": app,
					"name": "web", "service": "web", "type": "service",
					"release": "RELEASE2",
				},
			},
			Status: corev1.PodStatus{
				StartTime:         &am.Time{Time: started.Add(time.Second)},
				ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 4}},
			},
		}, am.CreateOptions{})
		require.NoError(t, err)

		state := structs.ReleasePromoteWatchState{
			SchemaVersion:      1,
			ReleaseID:          "release2",
			AtomVersion:        "release2",
			StartedAt:          started,
			ExpiresAt:          started.Add(300 * time.Millisecond),
			Actor:              "alice@example.com",
			CrashRestartLimits: map[string]int{"web": 3},
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			runWatcherWithAnnotation(t, p, app, &state)
		}, 1, 3*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Contains(t, data["message"], "crash-restart-limit: service=web restarts=4 limit=3")
		aa.AssertNumberOfCalls(t, "Cancel", 1)
	})
}

// The GC runs on a background context, so a tenant app's namespace has to be
// rebuilt from the tid label or every delete NotFounds and the GC goes silent.
func TestScanReleasePromoteAnnotations_TenantNamespace(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		app, tid := "appTenant", "t123"
		ns := fmt.Sprintf("%s-%s-%s", p.Name, tid, app)

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "RTEN",
			AtomVersion:   "RTEN",
			StartedAt:     time.Now().UTC().Add(-time.Hour),
			ExpiresAt:     time.Now().UTC().Add(-time.Hour),
			Actor:         "alice@example.com",
		}
		raw, err := json.Marshal(&state)
		require.NoError(t, err)

		_, err = kk.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: ns,
				Annotations: map[string]string{
					structs.ReleasePromoteWatchAnnotation: string(raw),
					"convox.com/app-status":               "Running",
					"convox.com/app-release":              "RTEN",
				},
				Labels: map[string]string{
					"app": app, "name": app, "rack": p.Name,
					"system": "convox", "type": "app", "tid": tid,
				},
			},
		}, am.CreateOptions{})
		require.NoError(t, err)

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 2*time.Second)

		require.NotNil(t, findEventByAction(events, "app:promote:completed"), "got %v", events)

		got, err := kk.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
		require.NoError(t, err)
		assert.Empty(t, got.Annotations[structs.ReleasePromoteWatchAnnotation],
			"the annotation must be reaped from the tenant namespace, not a rebuilt one")
	})
}
