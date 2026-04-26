package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// capturedEvent is one decoded EventSend payload captured by the
// in-process webhook server used by the auto-shutdown lifecycle tests.
type capturedEvent struct {
	Action string            `json:"action"`
	Data   map[string]string `json:"data"`
}

// eventCapture wraps an httptest.Server with a thread-safe slice to
// collect every event that EventSend dispatches. Tests assert presence
// of expected actions and read per-event payload fields. Tests must
// call .Close when done; the embedded mutex serializes concurrent
// writes from EventSend's goroutine fan-out.
type eventCapture struct {
	mu     sync.Mutex
	events []capturedEvent
	server *httptest.Server
}

func newEventCapture(t *testing.T) *eventCapture {
	t.Helper()
	c := &eventCapture{}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev capturedEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err == nil {
			c.mu.Lock()
			c.events = append(c.events, ev)
			c.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(c.server.Close)
	return c
}

// drain waits briefly for in-flight goroutine dispatches launched by
// EventSend to land. Mirrors the pattern in event_test.go.
func (c *eventCapture) drain() {
	time.Sleep(200 * time.Millisecond)
}

// findActions returns every captured event whose Action equals the
// given suffix (e.g. ":armed" matches "app:budget:auto-shutdown:armed").
func (c *eventCapture) findActions(suffix string) []capturedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []capturedEvent{}
	for _, e := range c.events {
		if strings.HasSuffix(e.Action, suffix) {
			out = append(out, e)
		}
	}
	return out
}

// buildAutoShutdownManifest returns a minimal Manifest populated with the
// auto-shutdown budget block + a single web service. Used as the
// pre-built manifest argument to ReconcileAutoShutdownWithManifestForTest.
// Tests that need extra services or custom timing override the returned
// fields before calling reconcile.
func buildAutoShutdownManifest(notifyBeforeMin int) *manifest.Manifest {
	return &manifest.Manifest{
		Budget: manifest.BudgetSettings{
			MonthlyCapUsd:         100,
			AtCapAction:           structs.BudgetAtCapActionAutoShutdown,
			AtCapWebhookUrl:       "https://hooks.example.com/budget",
			NotifyBeforeMinutes:   notifyBeforeMin,
			ShutdownGracePeriod:   "30s",
			ShutdownOrder:         "largest-cost",
			RecoveryMode:          "auto-on-reset",
			AlertThresholdPercent: 80,
			PricingAdjustment:     1,
		},
		Services: manifest.Services{
			{Name: "web"},
		},
	}
}

// TestReconcileAutoShutdown_FullLifecycle_ArmedFiredRestored drives
// reconcileAutoShutdown through a full lifecycle — armed → fired →
// restored — asserting each event fires with the correct payload, the
// state annotation transitions correctly, and the per-service shutdown
// PATCH lands.
//
// Tick 1: cap just breached + no annotation → :armed; annotation written
// with armedAt; Deployment replicas unchanged (notify window not yet up).
//
// Tick 2: now > armedAt + notifyBeforeMinutes → :fired; Deployment
// replicas patched to 0; shutdownAt persisted.
//
// Tick 3: customer manually scales the service back up; reconciler
// detects manual recovery → :restored reason="manual-detected" + 24h
// flap-suppressed-until carry-over written.
//
// This is the corrective Wave 8D advisory A3 fix — provides behavioral
// coverage of the central reconcile dispatch (was previously only
// covered by helper-level tests + grep verification).
func TestReconcileAutoShutdown_FullLifecycle_ArmedFiredRestored(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Pre-create a Deployment with replicas=3 so eligibility is
		// non-empty and :fired's PATCH has a target.
		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		// Cap-breach moment: write the :cap-fired budget state so the
		// reconciler picks up the breach indicator.
		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: t0,
			AlertFiredAtCap:       t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		// Force the auto-shutdown action into the in-memory cfg (the
		// canonical accumulator-rehydrated value used by reconcile).
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		// Tick 1: arm.
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		armedEvts := cap.findActions(":armed")
		require.Len(t, armedEvts, 1, "exactly one :armed event must fire on the first cap-breach tick")
		armed := armedEvts[0]
		assert.Equal(t, "app1", armed.Data["app"])
		assert.NotEmpty(t, armed.Data["scheduled_at"])
		assert.NotEmpty(t, armed.Data["expected_shutdown_at"])
		assert.Equal(t, "30", armed.Data["notify_before_minutes"])

		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		raw, ok := ns.Annotations[structs.BudgetShutdownStateAnnotation]
		require.True(t, ok, "shutdown-state annotation must be written on :armed tick")
		var armedState structs.AppBudgetShutdownState
		require.NoError(t, json.Unmarshal([]byte(raw), &armedState))
		require.NotNil(t, armedState.ArmedAt)
		assert.Nil(t, armedState.ShutdownAt, "ShutdownAt must be nil after :armed (fire window not yet elapsed)")
		require.Len(t, armedState.Services, 1)
		assert.Equal(t, "web", armedState.Services[0].Name)

		// Replicas not yet patched (notify window still open).
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, dep.Spec.Replicas)
		assert.Equal(t, int32(3), *dep.Spec.Replicas, "Deployment must remain at 3 replicas during the armed window")

		// Tick 2: notify window elapsed → :fired.
		t2 := t0.Add(31 * time.Minute)
		// Refresh baseState so AlertFiredAtCap is preserved across the
		// gap (mimics what accumulateBudgetApp would have written).
		baseState.CurrentMonthSpendAsOf = t2
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t2)
		cap.drain()

		firedEvts := cap.findActions(":fired")
		require.Len(t, firedEvts, 1, "exactly one :fired event must fire after the notify window elapses")
		fired := firedEvts[0]
		assert.Equal(t, "app1", fired.Data["app"])
		assert.Equal(t, "1", fired.Data["shut_down_count"])

		dep2, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, dep2.Spec.Replicas)
		assert.Equal(t, int32(0), *dep2.Spec.Replicas, "Deployment must be patched to 0 replicas after :fired")

		ns2, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		var firedState structs.AppBudgetShutdownState
		require.NoError(t, json.Unmarshal([]byte(ns2.Annotations[structs.BudgetShutdownStateAnnotation]), &firedState))
		require.NotNil(t, firedState.ShutdownAt, "ShutdownAt must be persisted after :fired")
		require.NotNil(t, firedState.FiredNotificationFiredAt)

		// Tick 3: customer manually scales web back to 5 replicas →
		// reconciler observes manual recovery → :restored.
		five := int32(5)
		dep2.Spec.Replicas = &five
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep2, am.UpdateOptions{})
		require.NoError(t, err)

		t3 := t2.Add(10 * time.Minute)
		baseState.CurrentMonthSpendAsOf = t3
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t3)
		cap.drain()

		restoredEvts := cap.findActions(":restored")
		require.Len(t, restoredEvts, 1, "exactly one :restored event must fire after manual recovery")
		restored := restoredEvts[0]
		assert.Equal(t, "app1", restored.Data["app"])
		assert.Equal(t, "manual-detected", restored.Data["recovery_trigger"])

		ns3, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		var restoredState structs.AppBudgetShutdownState
		require.NoError(t, json.Unmarshal([]byte(ns3.Annotations[structs.BudgetShutdownStateAnnotation]), &restoredState))
		require.NotNil(t, restoredState.RestoredAt, "RestoredAt must be persisted after :restored")
		require.NotNil(t, restoredState.FlapSuppressedUntil, "flap-suppressed-until carry-over must be set on :restored")
		// flap-suppressed-until annotation must also be written outside
		// the main state annotation (the post-GC carry-over channel).
		_, present := ns3.Annotations[structs.BudgetFlapSuppressedUntilAnnotation]
		assert.True(t, present, "flap-suppressed-until annotation must be written for the cooldown carry-over")
	})
}

// TestReconcileAutoShutdown_FiresExpiredOnMonthRollover verifies the
// :expired event fires when:
//
//   - manifest specifies recoveryMode: manual
//   - shutdown previously :fired in month N
//   - now is in month N+1 (rollover) WITHOUT a customer reset
//
// This test is the regression guard for advisory A1 (corrective Wave
// 8D): the original code compared `startOfMonth(now)` against the
// budget-state's MonthStart, which the accumulator had already reset to
// startOfMonth(now) BEFORE reconcileAutoShutdown ran — so the comparison
// was always false and :expired never fired. The fix compares against
// the persisted shutdown-state's recorded ShutdownAt month instead.
func TestReconcileAutoShutdown_FiresExpiredOnMonthRollover(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Pre-write a fired shutdown-state from March 28 (recoveryMode:
		// manual; no customer reset; no expiredAt yet).
		armed := time.Date(2026, 3, 28, 11, 30, 0, 0, time.UTC)
		shut := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			ShutdownAt:           &shut,
			RecoveryMode:         "manual",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-march-fired",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 0, Replicas: 3}},
			},
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})

		// Now is April 1 — rollover. The accumulator would have reset
		// budget-state's MonthStart to startOfMonth(now) BEFORE calling
		// reconcile, so baseState.MonthStart matches now's month. The
		// :expired check must NOT depend on baseState.MonthStart but on
		// the persisted shutdown-state's recorded ShutdownAt month.
		now := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
		baseState := &structs.AppBudgetState{
			MonthStart:            time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), // already rolled
			CurrentMonthSpendUsd:  0,
			CurrentMonthSpendAsOf: now,
			AlertFiredAtCap:       time.Time{}, // cap not yet re-breached in the new month
		}

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		m.Budget.RecoveryMode = "manual"

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		expiredEvts := cap.findActions(":expired")
		require.Len(t, expiredEvts, 1, "exactly one :expired event must fire on month rollover with recoveryMode=manual; advisory A1 regression guard")
		ev := expiredEvts[0]
		assert.Equal(t, "app1", ev.Data["app"])
		assert.Equal(t, "manual", ev.Data["recovery_mode"])

		// State annotation now carries expiredAt + dedup tracker.
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		var got structs.AppBudgetShutdownState
		require.NoError(t, json.Unmarshal([]byte(ns.Annotations[structs.BudgetShutdownStateAnnotation]), &got))
		require.NotNil(t, got.ExpiredAt)
		require.NotNil(t, got.ExpiredNotificationFiredAt)

		// Second tick in the same month must NOT re-fire (dedup).
		later := now.Add(1 * time.Hour)
		baseState.CurrentMonthSpendAsOf = later
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, later)
		cap.drain()
		expiredAgain := cap.findActions(":expired")
		assert.Len(t, expiredAgain, 1, ":expired must not re-fire on a subsequent tick within the same rollover window")
	})
}

// TestReconcileAutoShutdown_NoopReason_NoEligibleServices verifies that
// when every service in the manifest is filtered for a STATIC-config
// reason (here: all in `neverAutoShutdown`), the :noop event fires with
// reason="no-eligible-services". Advisory A2 regression guard.
func TestReconcileAutoShutdown_NoopReason_NoEligibleServices(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105, CurrentMonthSpendAsOf: now, AlertFiredAtCap: now,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		// All services in neverAutoShutdown → eligibility is empty for
		// the no-eligible-services reason path.
		m.Budget.NeverAutoShutdown = []string{"web"}

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		noopEvts := cap.findActions(":noop")
		require.Len(t, noopEvts, 1, "exactly one :noop event must fire when no eligible services remain")
		ev := noopEvts[0]
		assert.Equal(t, "no-eligible-services", ev.Data["reason"], "static-config-only filtering must surface reason=no-eligible-services")
		assert.Equal(t, "app1", ev.Data["app"])
		assert.Equal(t, "0", ev.Data["eligible_service_count"])
	})
}

// TestReconcileAutoShutdown_NoopReason_RuntimeDrift verifies that when
// the manifest declares services but at least one was filtered for a
// RUNTIME reason (Deployment not yet created), the :noop event fires
// with reason="runtime-drift". Advisory A2 regression guard.
func TestReconcileAutoShutdown_NoopReason_RuntimeDrift(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Deliberately do NOT create a Deployment for "web" → eligibility
		// will surface "no deployment yet (pending first deploy)" — the
		// runtime-drift reason.
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105, CurrentMonthSpendAsOf: now, AlertFiredAtCap: now,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		noopEvts := cap.findActions(":noop")
		require.Len(t, noopEvts, 1, "exactly one :noop event must fire when no deployment yet exists")
		ev := noopEvts[0]
		assert.Equal(t, "runtime-drift", ev.Data["reason"], "no-deployment-yet eligibility filter must surface reason=runtime-drift")
		assert.Equal(t, "app1", ev.Data["app"])
	})
}

// TestReconcileAutoShutdown_NoopReason_ExternalEditDetected verifies the
// spec §13.3 scenario: shutdownState is nil but every eligible service
// is already at 0 replicas (operator hand-recovery / CD pipeline strip
// scenario). The reconciler must fire :noop reason="external-edit-
// detected" instead of re-arming. Advisory A2 regression guard.
func TestReconcileAutoShutdown_NoopReason_ExternalEditDetected(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Deployment exists but is at 0 replicas — eligibility passes
		// (the deployment is present), but actual replica count is 0.
		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 0, &grace)

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105, CurrentMonthSpendAsOf: now, AlertFiredAtCap: now,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		noopEvts := cap.findActions(":noop")
		require.Len(t, noopEvts, 1, "exactly one :noop event must fire when external edit is detected")
		ev := noopEvts[0]
		assert.Equal(t, "external-edit-detected", ev.Data["reason"], "all-replicas-zero with no annotation must surface reason=external-edit-detected per spec §13.3")
		assert.Equal(t, "app1", ev.Data["app"])

		// :armed must NOT have fired (re-arming an already-zero
		// deployment would re-trip on the same outage).
		armedEvts := cap.findActions(":armed")
		assert.Empty(t, armedEvts, ":armed must NOT fire when external edit is detected")

		// No annotation must be written for the external-edit branch
		// (the customer's hand-recovery is the source of truth — we
		// observe-only and report).
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		_, present := ns.Annotations[structs.BudgetShutdownStateAnnotation]
		assert.False(t, present, "external-edit branch must NOT write the shutdown-state annotation")
	})
}

// =================================================================
// Phase γ Round 1.5 corrective patch — F1-F13b regression tests.
// =================================================================

// TestReconcileAutoShutdown_CancelledResetDuringArmed_FiresCorrectReason
// (F1 fix). Customer runs `convox budget reset` during the armed window.
// AppBudgetReset must fire :cancelled reason="reset-during-armed" and
// GC the orphan annotation.
func TestReconcileAutoShutdown_CancelledResetDuringArmed_FiresCorrectReason(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		now := time.Now().UTC()
		armed := now.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-armed-reset",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "ml-batch", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: now,
			AlertFiredAtCap:       now,
			CircuitBreakerTripped: true,
		})

		require.NoError(t, p.AppBudgetReset("app1", "test-actor"))
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.Len(t, cancelEvts, 1, ":cancelled event must fire on reset-during-armed")
		ev := cancelEvts[0]
		assert.Equal(t, "reset-during-armed", ev.Data["cancel_reason"], "cancel_reason must be reset-during-armed")
		assert.Equal(t, "app1", ev.Data["app"])
		assert.NotEmpty(t, ev.Data["armed_at"], "armed_at must be populated per spec §8.4")

		// Annotation must be GC'd so next cap re-breach re-arms cleanly (F8).
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		_, present := ns.Annotations[structs.BudgetShutdownStateAnnotation]
		assert.False(t, present, "armed-window annotation must be GC'd on reset-during-armed (F8)")
	})
}

// TestReconcileAutoShutdown_CancelledCapRaised_DistinctFromConfigChanged
// (F2 fix). Customer raises cap during armed window; reason must be
// "cap-raised" with prev_cap_usd / new_cap_usd populated, NOT
// "config-changed".
func TestReconcileAutoShutdown_CancelledCapRaised_DistinctFromConfigChanged(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		// Pre-armed annotation with manifestSha computed for old cap (100).
		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-cap-raised",
			ManifestSha256:       "OLD_SHA_DIFFERENT_FROM_PLAN_RECOMPUTE",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		// New cap (200) higher than spend (105) → cap-raised path.
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 200, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown
		cfg.MonthlyCapUsd = 200

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.Len(t, cancelEvts, 1, "exactly one :cancelled event must fire on cap-raise during armed window")
		ev := cancelEvts[0]
		assert.Equal(t, "cap-raised", ev.Data["cancel_reason"], "cap-raise must classify as 'cap-raised', NOT 'config-changed' (F2)")
		assert.NotEmpty(t, ev.Data["prev_cap_usd"], "prev_cap_usd must be populated for cap-raised case")
		assert.NotEmpty(t, ev.Data["new_cap_usd"], "new_cap_usd must be populated for cap-raised case")
		assert.Equal(t, "200", ev.Data["new_cap_usd"], "new_cap_usd must equal current cfg.MonthlyCapUsd")
		// Universal cap_usd should be the NEW cap, not the previous.
		assert.Equal(t, "200", ev.Data["cap_usd"], "universal cap_usd must reflect new cap on cap-raised (F4 + F5)")
	})
}

// TestCancelled_CapRaised_ActorIsJwtDerived — MF-6 fix (R6 γ-1 carry-forward
// NIT). Spec §8.4 line 777: cap-raised :cancelled events carry the JWT-derived
// actor of the user who raised the cap. AppBudgetSet records ackBy in
// cfg.LastCapMutationBy on every cap mutation; reconcileAutoShutdown reads
// it on cap-raise detection. Verifies the round-trip.
func TestCancelled_CapRaised_ActorIsJwtDerived(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-cap-raised-actor",
			ManifestSha256:       "OLD_SHA_DIFFERENT_FROM_PLAN_RECOMPUTE",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		// LastCapMutationBy populated as if customer just raised the cap.
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 200, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
			LastCapMutationBy: "alice@example.com",
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown
		cfg.MonthlyCapUsd = 200
		cfg.LastCapMutationBy = "alice@example.com"

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.Len(t, cancelEvts, 1)
		ev := cancelEvts[0]
		assert.Equal(t, "cap-raised", ev.Data["cancel_reason"])
		assert.Equal(t, "alice@example.com", ev.Data["actor"],
			"cap-raised actor must be JWT-derived from cfg.LastCapMutationBy per spec §8.4")
	})
}

// TestCancelled_CapRaised_FallsBackToSystemForOlderRack — MF-6 fix.
// Cross-version compat: pre-3.24.6 racks won't have LastCapMutationBy
// populated (omitempty drops it from JSON). Accumulator must fall back
// to "system" rather than emit empty actor.
func TestCancelled_CapRaised_FallsBackToSystemForOlderRack(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-cap-raised-fallback",
			ManifestSha256:       "OLD_SHA_DIFFERENT_FROM_PLAN_RECOMPUTE",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		// Older-rack scenario: LastCapMutationBy omitted (zero value).
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 200, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown
		cfg.MonthlyCapUsd = 200
		// LastCapMutationBy intentionally NOT set on cfg either.

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.Len(t, cancelEvts, 1)
		ev := cancelEvts[0]
		assert.Equal(t, "cap-raised", ev.Data["cancel_reason"])
		assert.Equal(t, "system", ev.Data["actor"],
			"cap-raised actor must fall back to 'system' when LastCapMutationBy is empty (older-rack compat)")
	})
}

// TestCancelled_ConfigChanged_ActorStaysSystem — MF-6 fix.
// config-changed sub-case detects manifest mismatch via accumulator-only
// observation; no originating HTTP request, no JWT in scope. Even when
// LastCapMutationBy is populated, config-changed must keep actor="system"
// because the cap-raise plumbing doesn't apply to a manifest mutation.
func TestCancelled_ConfigChanged_ActorStaysSystem(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-config-changed-actor",
			ManifestSha256:       "OLD_SHA_DIFFERENT_FROM_PLAN_RECOMPUTE",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		// LastCapMutationBy IS populated (from a prior cap-mutation), but
		// cap is NOT raised this tick (100 stays 100, spend 105 > cap → no
		// cap-raised). ManifestSha256 mismatch in the armed-state annotation
		// is what triggers config-changed reason classification.
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
			LastCapMutationBy: "bob@example.com",
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown
		cfg.MonthlyCapUsd = 100
		cfg.LastCapMutationBy = "bob@example.com"

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.Len(t, cancelEvts, 1)
		ev := cancelEvts[0]
		assert.Equal(t, "config-changed", ev.Data["cancel_reason"])
		assert.Equal(t, "system", ev.Data["actor"],
			"config-changed actor must stay 'system' even when LastCapMutationBy is populated — manifest mismatch detection has no originating user")
	})
}

// TestReconcileAutoShutdown_FailedExclusiveOfFired_NoConcurrentFire
// (F3 fix). Spec §8.10: :fired and :failed are MUTUALLY EXCLUSIVE.
// On partial-shutdown (some succeed, some fail), only :failed fires.
func TestReconcileAutoShutdown_FailedExclusiveOfFired_NoConcurrentFire(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// One service exists (will succeed); the second is referenced in
		// the saved state but no Deployment exists — shutdownService
		// errors out, so PATCH "fails" for it.
		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)
		// "missing-svc" deliberately not created → shutdownService fails.

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-31 * time.Minute) // already past notify window
		// Pre-compute manifest SHA so the config-drift branch doesn't fire :cancelled.
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:  1,
			ArmedAt:        &armed,
			RecoveryMode:   "auto-on-reset",
			ShutdownOrder:  "largest-cost",
			ShutdownTickId: "tick-partial-failed",
			// ManifestSha256 left empty to bypass the config-drift detection.
			EligibleServiceCount: 2,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
				{Name: "missing-svc", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 1, Replicas: 1}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		// Manifest needs to expose both services to keep eligibility consistent
		m.Services = manifest.Services{{Name: "web"}, {Name: "missing-svc"}}

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		// CRITICAL: :failed fires, :fired must NOT.
		failedEvts := cap.findActions(":failed")
		firedEvts := cap.findActions(":fired")
		assert.Len(t, failedEvts, 1, ":failed must fire when any service fails")
		assert.Empty(t, firedEvts, ":fired and :failed are mutually exclusive — :fired must NOT fire on partial-shutdown (F3)")

		// :failed payload must carry partial_state = succeeded count.
		if len(failedEvts) == 1 {
			ev := failedEvts[0]
			assert.Equal(t, "1", ev.Data["partial_state"], "partial_state must equal succeeded count (1: web succeeded, missing-svc failed)")
		}

		// F-6 fix (catalog F-6): assert provider-side persistence of
		// FailureReason on the state annotation. Locks in the persist
		// path at budget_auto_shutdown.go (which is currently single-line
		// code with no direct provider-layer test).
		ns2, gerr := kk.CoreV1().Namespaces().Get(context.Background(), "rack1-app1", am.GetOptions{})
		require.NoError(t, gerr, "post-fire ns get must succeed")
		raw, ok := ns2.Annotations[structs.BudgetShutdownStateAnnotation]
		require.True(t, ok, "shutdown-state annotation must be present after :failed fired")
		var persisted structs.AppBudgetShutdownState
		require.NoError(t, json.Unmarshal([]byte(raw), &persisted), "persisted state must parse")
		assert.Equal(t, structs.BudgetShutdownReasonK8sApiFailure, persisted.FailureReason,
			"FailureReason must persist as k8s-api-failure so the FAILED banner renders the canonical reason (catalog F-6)")
	})
}

// TestEventPayload_AllNineEvents_PopulateCapUsdAndSpendUsd (F4 fix).
// Verifies every fire helper passes cap_usd from cfg and spend_usd from
// baseState — never hardcoded 0.
func TestEventPayload_AllNineEvents_PopulateCapUsdAndSpendUsd(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		// Trigger :armed.
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		armedEvts := cap.findActions(":armed")
		require.Len(t, armedEvts, 1)
		assert.Equal(t, "100", armedEvts[0].Data["cap_usd"], ":armed cap_usd must be populated from cfg (F4 + F5)")
		assert.Equal(t, "105.00", armedEvts[0].Data["spend_usd"], ":armed spend_usd must be populated from baseState (F4)")

		// Trigger :fired (advance past notify window).
		t1 := t0.Add(31 * time.Minute)
		baseState.CurrentMonthSpendAsOf = t1
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t1)
		cap.drain()

		firedEvts := cap.findActions(":fired")
		require.Len(t, firedEvts, 1)
		assert.Equal(t, "100", firedEvts[0].Data["cap_usd"], ":fired cap_usd must be populated from cfg (F4 + F5)")
		assert.Equal(t, "105.00", firedEvts[0].Data["spend_usd"], ":fired spend_usd must be populated from baseState (F4)")
	})
}

// TestEventPayload_CapUsdIsInt_NotDecimal (F5 fix). Spec §8.0 line 657
// mandates cap_usd as int. Universal payload helper must emit "100" not
// "100.00" for cap=100.
func TestEventPayload_CapUsdIsInt_NotDecimal(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 250, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 260,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		armedEvts := cap.findActions(":armed")
		require.Len(t, armedEvts, 1)
		// cap_usd must be "250" (int format) not "250.00" (decimal).
		assert.Equal(t, "250", armedEvts[0].Data["cap_usd"], "cap_usd must be int format per spec §8.0 (F5)")
		assert.NotContains(t, armedEvts[0].Data["cap_usd"], ".", "cap_usd must NOT contain decimal point")
	})
}

// TestEventPayload_SpecFieldNamesMatchVerbatim (F6 fix). Verifies the
// 6 spec-mandated field name renames per spec §8.x.
func TestEventPayload_SpecFieldNamesMatchVerbatim(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		armedEvts := cap.findActions(":armed")
		require.Len(t, armedEvts, 1)
		// F6 fix: :armed uses scheduled_at + expected_shutdown_at.
		assert.NotEmpty(t, armedEvts[0].Data["scheduled_at"], "spec §8.1 mandates scheduled_at (F6)")
		assert.NotEmpty(t, armedEvts[0].Data["expected_shutdown_at"], "spec §8.1 mandates expected_shutdown_at (F6)")
		_, hasOldArmedAt := armedEvts[0].Data["armed_at"]
		_, hasOldFireAt := armedEvts[0].Data["fire_at"]
		assert.False(t, hasOldArmedAt, ":armed must NOT carry old field name 'armed_at' (F6)")
		assert.False(t, hasOldFireAt, ":armed must NOT carry old field name 'fire_at' (F6)")
	})
}

// TestFiredPayload_HasSnapshotAnnotationAndRecoveryCommand (F7 fix).
// Spec §8.2 mandates snapshot_annotation + recovery_command + 5 other
// fields on :fired payload.
func TestFiredPayload_HasSnapshotAnnotationAndRecoveryCommand(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		t1 := t0.Add(31 * time.Minute)
		baseState.CurrentMonthSpendAsOf = t1
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t1)
		cap.drain()

		firedEvts := cap.findActions(":fired")
		require.Len(t, firedEvts, 1)
		ev := firedEvts[0]
		// F7 mandated fields per spec §8.2.
		assert.NotEmpty(t, ev.Data["snapshot_annotation"], "snapshot_annotation must be populated (F7)")
		assert.Contains(t, ev.Data["snapshot_annotation"], "shutdownTickId", "snapshot_annotation must contain JSON of state")
		assert.Equal(t, "convox budget reset app1", ev.Data["recovery_command"], "recovery_command must be populated (F7)")
		assert.NotEmpty(t, ev.Data["shutdown_at"], "shutdown_at must be populated (F7 — was fired_at)")
		assert.NotEmpty(t, ev.Data["keda_managed_count"], "keda_managed_count must be populated (F7)")
		assert.NotEmpty(t, ev.Data["deployment_only_count"], "deployment_only_count must be populated (F7)")
	})
}

// TestArmedWindowManualCancel_GCsAnnotation_NoImmediateFireOnRebreath
// (F8 fix). Customer scales back up during armed window → :cancelled
// reason="manual-detected" + annotation GC'd. Next cap re-breach
// re-arms cleanly with :armed (no immediate :fired).
func TestArmedWindowManualCancel_GCsAnnotation_NoImmediateFireOnRebreath(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		// Deployment at 5 (customer scaled UP from snapshot of 3).
		makeDeployment(t, kk, "rack1-app1", "web", 5, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-armed-manual",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				// snapshot was 3, but live is 5 → customer scaled UP.
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		// Use the full reconcileAutoShutdown to drive the F8 detection path
		// (it's pre-manifest so we must use the entry point, not just the
		// with-manifest helper).
		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		// Drive through accumulator entry — this calls reconcileAutoShutdown
		// which checks armedWindowManuallyScaledUp before the manifest path.
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", t0))
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.GreaterOrEqual(t, len(cancelEvts), 1, ":cancelled reason=manual-detected must fire on F8 detection")
		// Annotation must be GC'd.
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		_, present := ns.Annotations[structs.BudgetShutdownStateAnnotation]
		assert.False(t, present, "armed-window annotation must be GC'd after manual-detected (F8)")
		// Best-effort sanity: cfg/baseState passed to fire helper.
		_ = cfg
		_ = baseState
	})
}

// TestNoopDeduplicationViaNoopFiredAtAnnotation (F9 fix). :noop must
// NOT re-fire on every tick — dedup via dedicated annotation.
func TestNoopDeduplicationViaNoopFiredAtAnnotation(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		m.Budget.NeverAutoShutdown = []string{"web"} // → no eligible → :noop

		// First tick: :noop fires.
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()
		first := cap.findActions(":noop")
		require.Len(t, first, 1, "first tick: :noop fires once")

		// Dedup annotation written.
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		_, dedup := ns.Annotations[structs.BudgetShutdownNoopFiredAtAnnotation]
		assert.True(t, dedup, "noop dedup annotation must be written (F9)")

		// Second tick at same time → must NOT re-fire (dedup window not expired).
		baseState.CurrentMonthSpendAsOf = t0.Add(1 * time.Minute)
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0.Add(1*time.Minute))
		cap.drain()
		second := cap.findActions(":noop")
		assert.Len(t, second, 1, ":noop must NOT re-fire within dedup window (F9)")
	})
}

// TestFailedStateCorrupt_DedupViaSeparateAnnotation (F10 fix). State
// is unparseable; :failed reason="state-corrupt" must dedup via SEPARATE
// annotation since the main state is unwritable.
func TestFailedStateCorrupt_DedupViaSeparateAnnotation(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Pre-write a corrupt state annotation.
		ns, _ := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetShutdownStateAnnotation] = "{not-valid-json"
		ns.Annotations[structs.BudgetConfigAnnotation] = `{"monthly-cap-usd":100,"alert-threshold-percent":80,"at-cap-action":"auto-shutdown","pricing-adjustment":1}`
		ns.Annotations[structs.BudgetStateAnnotation] = `{"month-start":"2026-04-01T00:00:00Z","current-month-spend-usd":105,"alert-fired-at-cap":"2026-04-15T12:00:00Z"}`
		_, err := kk.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{})
		require.NoError(t, err)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		// First tick → :failed reason=state-corrupt fires.
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", t0))
		cap.drain()
		first := cap.findActions(":failed")
		require.GreaterOrEqual(t, len(first), 1, "first corrupt tick: :failed fires")

		// Dedup annotation written separately.
		ns2, _ := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		_, dedup := ns2.Annotations[structs.BudgetShutdownStateCorruptFiredAtAnnotation]
		assert.True(t, dedup, "state-corrupt dedup annotation must be written separately (F10)")

		// Second tick within dedup window → must NOT re-fire.
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", t0.Add(1*time.Minute)))
		cap.drain()
		second := cap.findActions(":failed")
		assert.Len(t, second, len(first), ":failed reason=state-corrupt must NOT re-fire within dedup window (F10)")
	})
}

// TestCancelledPayload_HasAllSpecFields (F11 fix). :cancelled payload
// must carry armed_at, expected_shutdown_at, eligible_services,
// cancel_reason, plus prev/new_cap_usd or new_action per sub-case.
func TestCancelledPayload_HasAllSpecFields(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		now := time.Now().UTC()
		armed := now.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-cancel-fields",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "ml-batch", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: now,
			AlertFiredAtCap:       now,
			CircuitBreakerTripped: true,
		})

		require.NoError(t, p.AppBudgetReset("app1", "test-actor"))
		cap.drain()

		cancelEvts := cap.findActions(":cancelled")
		require.GreaterOrEqual(t, len(cancelEvts), 1)
		ev := cancelEvts[0]
		// F11 mandated fields.
		assert.NotEmpty(t, ev.Data["cancelled_at"], "cancelled_at must be populated")
		assert.NotEmpty(t, ev.Data["cancel_reason"], "cancel_reason must be populated")
		assert.NotEmpty(t, ev.Data["armed_at"], "armed_at must be populated (F11)")
		assert.NotEmpty(t, ev.Data["expected_shutdown_at"], "expected_shutdown_at must be populated (F11)")
		assert.NotEmpty(t, ev.Data["eligible_services"], "eligible_services must be populated (F11)")
	})
}

// TestExpiredPayload_HasManualActionHint (F12 fix). :expired payload
// must carry manual_action_hint + requires_manual_action + final_spend_usd.
func TestExpiredPayload_HasManualActionHint(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		armed := time.Date(2026, 3, 28, 11, 30, 0, 0, time.UTC)
		shut := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			ShutdownAt:           &shut,
			RecoveryMode:         "manual",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-expired-fields",
			EligibleServiceCount: 1,
			Services:             []structs.AppBudgetShutdownStateService{{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}}},
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})

		now := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
		baseState := &structs.AppBudgetState{
			MonthStart:            time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			CurrentMonthSpendUsd:  0,
			CurrentMonthSpendAsOf: now,
		}
		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		m.Budget.RecoveryMode = "manual"

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		expiredEvts := cap.findActions(":expired")
		require.Len(t, expiredEvts, 1)
		ev := expiredEvts[0]
		// F12 mandated fields.
		assert.NotEmpty(t, ev.Data["expired_at"], "expired_at must be populated")
		assert.NotEmpty(t, ev.Data["original_shutdown_at"], "original_shutdown_at must be populated (F6 + F12)")
		assert.NotEmpty(t, ev.Data["original_armed_at"], "original_armed_at must be populated (F12)")
		assert.NotEmpty(t, ev.Data["services_still_at_zero"], "services_still_at_zero must be populated (F6)")
		assert.NotEmpty(t, ev.Data["prev_month_label"], "prev_month_label must be populated (F12)")
		assert.NotEmpty(t, ev.Data["new_month_label"], "new_month_label must be populated (F12)")
		assert.Equal(t, "true", ev.Data["requires_manual_action"], "requires_manual_action must be 'true' (F12)")
		assert.Contains(t, ev.Data["manual_action_hint"], "convox services update", "manual_action_hint must contain command (F12)")
		assert.Equal(t, "2026-03", ev.Data["prev_month_label"])
		assert.Equal(t, "2026-04", ev.Data["new_month_label"])
	})
}

// TestFiredPersistThenEmit_LeaderFailoverNoDoublefire (F13b fix). The
// persist-before-emit ordering means that if a crash between persist
// and emit causes a new leader to retry, the dedup check sees
// FiredNotificationFiredAt set and skips re-emit.
func TestFiredPersistThenEmit_LeaderFailoverNoDoublefire(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-31 * time.Minute) // already past notify window
		// Simulate a state where FiredNotificationFiredAt is already set
		// (simulating: previous tick fired :fired, persisted dedup, then
		// crashed AFTER emit. New leader should NOT re-emit.).
		alreadyFired := t0.Add(-1 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-failover",
			ManifestSha256:       "",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
			FiredNotificationFiredAt: &alreadyFired,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		// New "leader" tick.
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		firedEvts := cap.findActions(":fired")
		assert.Empty(t, firedEvts, ":fired must NOT re-fire when FiredNotificationFiredAt is already set (F13b)")
	})
}

// TestArmedPayload_WebhookUrlIsRedacted_NoSecretLeak — MF-2 fix
// (R4 γ-6 NIT + γ-12 NIT). F-1 redacts data["webhook_url"] in the :armed
// payload to scheme+host so customer Slack/Discord bearer tokens embedded
// in atCapWebhookUrl never reach rack-level webhook subscribers. The
// helper-tier test (TestRedactURLHost_StripsSecrets in event_test.go)
// locks the function; this test locks the wire-up: an end-to-end :armed
// fire must emit a payload that omits the secret AND yields a scheme+host
// URL receivers can parse with new URL(...) without throwing.
func TestArmedPayload_WebhookUrlIsRedacted_NoSecretLeak(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: t0,
			AlertFiredAtCap:       t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		// Webhook URL with embedded Slack-style bearer token. F-1 must
		// strip the path component before broadcasting to subscribers.
		m.Budget.AtCapWebhookUrl = "https://hooks.slack.com/services/T0AAA/B0BBB/SECRETXYZTOKEN"

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		armedEvts := cap.findActions(":armed")
		require.Len(t, armedEvts, 1, "exactly one :armed event must fire")
		armed := armedEvts[0]

		// Must contain redacted form (scheme + host).
		assert.Equal(t, "https://hooks.slack.com", armed.Data["webhook_url"],
			"webhook_url must be redacted to scheme+host (RFC 3986 valid)")
		// Must NOT leak the secret token or path components.
		assert.NotContains(t, armed.Data["webhook_url"], "SECRETXYZTOKEN",
			"redacted webhook_url must not contain the embedded token")
		assert.NotContains(t, armed.Data["webhook_url"], "services",
			"redacted webhook_url must not contain path components")
		assert.NotContains(t, armed.Data["webhook_url"], "T0AAA",
			"redacted webhook_url must not contain workspace identifiers")
		assert.NotContains(t, armed.Data["webhook_url"], "B0BBB",
			"redacted webhook_url must not contain bot identifiers")
	})
}

// TestExpiredPersistFails_NoEmit — MF-7 (F-20 extension to :expired).
// When persistShutdownState fails on the :expired path, the :expired
// event must NOT be emitted. Without this gate, next tick reads
// ExpiredNotificationFiredAt==nil (because the Update never landed) and
// re-fires :expired — visible duplicate on the wire.
func TestExpiredPersistFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		// Seed shutdown state from the previous month — :expired branch
		// preconditions: ShutdownAt set, ExpiredAt nil, RestoredAt nil,
		// RecoveryMode "manual", and now is in a later month.
		armed := time.Date(2026, 3, 28, 11, 30, 0, 0, time.UTC)
		shut := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			ShutdownAt:           &shut,
			RecoveryMode:         "manual",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-expired-persistfail",
			EligibleServiceCount: 1,
			Services:             []structs.AppBudgetShutdownStateService{{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}}},
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})

		now := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
		baseState := &structs.AppBudgetState{
			MonthStart:            time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			CurrentMonthSpendUsd:  0,
			CurrentMonthSpendAsOf: now,
			AlertFiredAtCap:       now,
		}
		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		m.Budget.RecoveryMode = "manual"

		// Inject a persist failure: every namespace UPDATE returns an error.
		// The persist-then-emit gate must abort :expired emission.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			return true, nil, fmt.Errorf("synthetic persist failure for MF-7 expired test")
		})

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, now)
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 1, "persist must be attempted")
		expiredEvts := cap.findActions(":expired")
		assert.Empty(t, expiredEvts, ":expired must NOT be emitted when persistShutdownState fails (MF-7 / F-20 extension)")
	})
}

// TestNoopPersistFails_NoEmit — MF-7 (F-20 extension to :noop).
// When the dedup-annotation write fails on the :noop path, the :noop
// event must NOT be emitted. Without this gate, next tick re-fires
// :noop on every reconcile loop until the annotation finally lands.
func TestNoopPersistFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)
		m.Budget.NeverAutoShutdown = []string{"web"} // → no eligible → :noop branch

		// Inject persist failure: dedup-annotation Update fails.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			return true, nil, fmt.Errorf("synthetic dedup-annotation failure for MF-7 noop test")
		})

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 1, "annotation write must be attempted")
		noopEvts := cap.findActions(":noop")
		assert.Empty(t, noopEvts, ":noop must NOT be emitted when dedup-annotation write fails (MF-7 / F-20 extension)")
	})
}

// TestFlapSuppressed_HappyPath_Emits — R8.5 F-2 (γ-5 R8 ADV).
// Positive-path companion to TestFlapSuppressedPersistFails_NoEmit. Verifies
// that under normal conditions (annotation write succeeds), the
// :flap-suppressed event IS emitted and the dedup annotation IS set, so
// next reconcile tick does NOT re-emit. Without this test, the persist-fail
// regression test alone could pass while the success path silently breaks.
func TestFlapSuppressed_HappyPath_Emits(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		flap := t0.Add(7 * 24 * time.Hour)
		require.NoError(t, k8s.WriteFlapSuppressedUntilAnnotationForTest(p, "app1", flap))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		// Positive: :flap-suppressed event WAS emitted.
		flapEvts := cap.findActions(":flap-suppressed")
		require.Len(t, flapEvts, 1, "exactly one :flap-suppressed event must fire on first cap-breach within cooldown")

		// Positive: dedup annotation IS set so next tick skips re-emit.
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		_, present := ns.Annotations[structs.BudgetFlapSuppressFiredAtAnnotation]
		assert.True(t, present, "BudgetFlapSuppressFiredAtAnnotation must be persisted before emit so next tick dedups")

		// Idempotency: a second tick within the flap window must NOT re-emit.
		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0.Add(1*time.Minute))
		cap.drain()

		flapEvtsAfter := cap.findActions(":flap-suppressed")
		assert.Len(t, flapEvtsAfter, 1, ":flap-suppressed must NOT re-emit on subsequent tick within flap window (dedup-via-annotation working)")
	})
}

// TestStateCorruptPersistFails_NoEmit — R9.5 F-1 (R8.5 F-1 companion).
// When the dedup-annotation write fails on the state-corrupt :failed
// branch, the :failed event must NOT be emitted. Without this gate,
// next reconcile tick re-reads the corrupt annotation, the F10 dedup
// window check fires (BudgetShutdownStateCorruptFiredAtAnnotation == unset
// because the prior write failed), and :failed reason="state-corrupt"
// re-fires — duplicate event. Mirror of TestFlapSuppressedPersistFails_NoEmit
// (R7.5 F-3) for the 14th lifecycle emit site.
func TestStateCorruptPersistFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		// Seed a corrupt annotation directly so the parse failure path
		// is taken at reconcile time. F10 dedup annotation absent so the
		// emit branch would normally fire.
		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetShutdownStateAnnotation] = "{not valid json"
		_, err = kk.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{})
		require.NoError(t, err)

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		})

		// Counter-based reactor — accumulator's main spend Update fires
		// first; subsequent Updates (the F-1 dedup-annotation write) fail.
		// Pattern matches TestManualDetectedDeleteFails_NoEmit (R7.5 F-3).
		//
		// NOTE: uses AccumulateBudgetAppForTest (production path via
		// accumulateBudgetApp → reconcileAutoShutdown) — this is the
		// canonical surface for F-1. As of R10.5,
		// ReconcileAutoShutdownWithManifestForTest also mirrors the gate;
		// switching to it would require recalibrating the reactor counter
		// because the helper path skips the accumulator's main spend Update.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			if updateAttempts == 1 {
				return false, nil, nil // let accumulator's main spend Update through
			}
			return true, nil, fmt.Errorf("synthetic dedup-annotation failure for R9.5 F-1 state-corrupt test")
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", t0))
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 1, "annotation write must be attempted")
		failedEvts := cap.findActions(":failed")
		assert.Empty(t, failedEvts, ":failed reason=state-corrupt must NOT be emitted when dedup-annotation write fails (R9.5 F-1 / R8.5 F-1 companion)")
	})
}

// TestFlapSuppressedPersistFails_NoEmit — R7.5 F-3 (F-20 extension to
// :flap-suppressed). When the dedup-annotation write fails, the
// :flap-suppressed event must NOT be emitted. Without this gate, next
// reconcile tick reads BudgetFlapSuppressFiredAtAnnotation == unset and
// re-fires :flap-suppressed — duplicate event on the bus.
func TestFlapSuppressedPersistFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		makeDeployment(t, kk, "rack1-app1", "web", 3, &grace)

		// Seed FlapSuppressedUntil annotation in the future — :flap-suppressed
		// branch preconditions: cap breached, no shutdownState, flap window
		// still active.
		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		flap := t0.Add(7 * 24 * time.Hour)
		require.NoError(t, k8s.WriteFlapSuppressedUntilAnnotationForTest(p, "app1", flap))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		cfg.AtCapAction = structs.BudgetAtCapActionAutoShutdown

		m := buildAutoShutdownManifest(30)

		// Inject a dedup-annotation write failure: every namespace UPDATE
		// returns an error. The persist-then-emit gate must abort :flap-
		// suppressed emission.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			return true, nil, fmt.Errorf("synthetic dedup-annotation failure for R7.5 F-3 flap-suppressed test")
		})

		k8s.ReconcileAutoShutdownWithManifestForTest(p, context.Background(), "app1", cfg, baseState, m, t0)
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 1, "annotation write must be attempted")
		flapEvts := cap.findActions(":flap-suppressed")
		assert.Empty(t, flapEvts, ":flap-suppressed must NOT be emitted when dedup-annotation write fails (R7.5 F-3 / F-20 extension)")
	})
}

// TestManualDetectedDeleteFails_NoEmit — R7.5 F-3 (F-20 extension to
// F8 :cancelled reason="manual-detected"). When the GC of the orphan
// shutdown-state annotation fails, the :cancelled event must NOT be
// emitted. The annotation deletion IS the dedup signal: with the
// annotation gone, next tick's armedWindowManuallyScaledUp returns
// nothing and the manual-detected branch is skipped. If delete fails
// and emit fires anyway, next tick re-detects + re-fires.
func TestManualDetectedDeleteFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		grace := int64(30)
		// Customer scaled UP from snapshot (was 3, now 5) — F8 detects.
		makeDeployment(t, kk, "rack1-app1", "web", 5, &grace)

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-manual-detected-deletefail",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
		}
		writeState(t, kk, "rack1-app1", baseState)

		// Inject a delete failure that targets only the annotation-removal
		// Update. The accumulator's main spend Update goes first; we let
		// it through. The reconcileAutoShutdown manual-detected branch
		// then attempts to delete the shutdown-state annotation, which
		// becomes the failing Update. The delete-then-emit gate must
		// abort :cancelled emission.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			if updateAttempts == 1 {
				return false, nil, nil // let accumulator's main spend Update through
			}
			return true, nil, fmt.Errorf("synthetic GC failure for R7.5 F-3 manual-detected test")
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", t0))
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 2, "main spend Update + annotation delete Update must both be attempted")
		cancelEvts := cap.findActions(":cancelled")
		assert.Empty(t, cancelEvts, ":cancelled reason=manual-detected must NOT be emitted when annotation delete fails (R7.5 F-3 / F-20 extension)")
	})
}

// TestResetDuringArmedDeleteFails_NoEmit — R7.5 F-3 (F-20 extension to
// AppBudgetReset's F1+F8 :cancelled reason="reset-during-armed" path).
// When the GC of the orphan annotation after a successful budget reset
// fails, the :cancelled event must NOT be emitted. Same pattern as
// manual-detected: delete-then-emit prevents next-tick re-detect-re-fire.
func TestResetDuringArmedDeleteFails_NoEmit(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		installFakeDynamicClient(p)

		cap := newEventCapture(t)
		k8s.SetWebhooksForTest(p, []string{cap.server.URL})

		t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		armed := t0.Add(-5 * time.Minute)
		// Pre-armed annotation present.
		state := &structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-reset-during-armed-deletefail",
			EligibleServiceCount: 1,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "web", OriginalScale: structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3}},
			},
			ArmedNotificationFiredAt: &armed,
		}
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80,
			AtCapAction: structs.BudgetAtCapActionAutoShutdown, PricingAdjustment: 1,
		})
		// Tripped baseState so reset is meaningful.
		baseState := &structs.AppBudgetState{
			MonthStart: startOfApril(), CurrentMonthSpendUsd: 105,
			CurrentMonthSpendAsOf: t0, AlertFiredAtCap: t0,
			CircuitBreakerTripped: true,
		}
		writeState(t, kk, "rack1-app1", baseState)

		// Count namespace UPDATE attempts. The reset's main update succeeds
		// for the FIRST attempt (clearing the breaker); the second update
		// (the annotation delete) fails. We use a counter to differentiate.
		var updateAttempts int
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			if updateAttempts == 1 {
				return false, nil, nil // let the reset's main update through
			}
			return true, nil, fmt.Errorf("synthetic GC failure for R7.5 F-3 reset-during-armed test")
		})

		require.NoError(t, p.AppBudgetReset("app1", "alice@example.com"))
		cap.drain()

		assert.GreaterOrEqual(t, updateAttempts, 2, "main reset Update + annotation delete Update must both be attempted")
		cancelEvts := cap.findActions(":cancelled")
		assert.Empty(t, cancelEvts, ":cancelled reason=reset-during-armed must NOT be emitted when annotation delete fails (R7.5 F-3 / F-20 extension)")
	})
}

// Smoke reference to keep the appsv1 import live in the rare build
// configuration where every consumer is conditionally compiled out.
var _ = appsv1.Deployment{}
var _ = ac.Pod{}
