package k8s_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// captureEventActorByAction captures the "actor" field on the first event
// matching a given action. Spawns a webhook recorder and waits up to 2s.
func captureEventActorByAction(t *testing.T, action string, fn func(*k8s.Provider) error) string {
	t.Helper()
	bodyCh := make(chan []byte, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		select {
		case bodyCh <- b:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var actor string
	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		require.NoError(t, fn(p))

		deadline := time.After(2 * time.Second)
		for {
			select {
			case body := <-bodyCh:
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				if payload["action"] == action {
					data, _ := payload["data"].(map[string]any)
					a, _ := data["actor"].(string)
					actor = a
					return
				}
			case <-deadline:
				t.Fatalf("did not observe action %q within 2s", action)
				return
			}
		}
	})
	return actor
}

// TestAccumulateBudgetThreshold_EmitsSystemActor: accumulator-fired threshold
// alert MUST emit actor=system regardless of ctx (per-call-site override).
func TestAccumulateBudgetThreshold_EmitsSystemActor(t *testing.T) {
	frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	got := captureEventActorByAction(t, "app:budget:threshold", func(p *k8s.Provider) error {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  85,
			CurrentMonthSpendAsOf: frozen,
		})
		return k8s.AccumulateBudgetAppForTest(p, "app1", frozen)
	})
	assert.Equal(t, "system", got, "accumulator threshold event MUST pin actor=system")
}

// TestAccumulateBudgetCap_EmitsSystemActor: cap alert MUST emit actor=system.
func TestAccumulateBudgetCap_EmitsSystemActor(t *testing.T) {
	frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	got := captureEventActorByAction(t, "app:budget:cap", func(p *k8s.Provider) error {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  150,
			CurrentMonthSpendAsOf: frozen,
		})
		return k8s.AccumulateBudgetAppForTest(p, "app1", frozen)
	})
	assert.Equal(t, "system", got, "accumulator cap event MUST pin actor=system")
}

// TestAccumulator_OverridePinsSystemEvenWithJwtCtx: even if a JWT ctx is
// somehow attached to the provider, the explicit "actor": "system" override at
// the call site MUST win.
func TestAccumulator_OverridePinsSystemEvenWithJwtCtx(t *testing.T) {
	frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	got := captureEventActorByActionWithProviderHook(t, "app:budget:cap",
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			kk, _ := p.Cluster.(*fake.Clientset)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
				MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
			})
			writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
				MonthStart:            startOfApril(),
				CurrentMonthSpendUsd:  150,
				CurrentMonthSpendAsOf: frozen,
			})
			return k8s.AccumulateBudgetAppForTest(p, "app1", frozen)
		})
	assert.Equal(t, "system", got, "explicit override beats ctx-derived actor")
}

// captureEventActorByActionWithProviderHook is captureEventActorByAction with
// a hook to mutate the provider (e.g. WithContext) before fn runs. The hook
// receives the testProvider() output and returns the provider that fn uses.
func captureEventActorByActionWithProviderHook(
	t *testing.T,
	action string,
	hook func(*k8s.Provider) *k8s.Provider,
	fn func(*k8s.Provider) error,
) string {
	t.Helper()
	bodyCh := make(chan []byte, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		select {
		case bodyCh <- b:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var actor string
	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		pp := hook(p)
		// Re-install webhooks on the WithContext-cloned provider too, since
		// WithContext returns a copy of the receiver.
		k8s.SetWebhooksForTest(pp, []string{srv.URL})

		require.NoError(t, fn(pp))

		deadline := time.After(2 * time.Second)
		for {
			select {
			case body := <-bodyCh:
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				if payload["action"] == action {
					data, _ := payload["data"].(map[string]any)
					a, _ := data["actor"].(string)
					actor = a
					return
				}
			case <-deadline:
				t.Fatalf("did not observe action %q within 2s", action)
				return
			}
		}
	})
	return actor
}

// TestAppBudgetSet_NoContext_EmitsUnknown: AppBudgetSet without a JWT ctx
// emits actor=unknown (defense-in-depth fallback). The accumulator-driven
// events get explicit "system" but interactive AppBudgetSet uses central
// injection.
func TestAppBudgetSet_NoContext_EmitsUnknown(t *testing.T) {
	got := captureEventActorByAction(t, "app:budget:set", func(p *k8s.Provider) error {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		return p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd:         strPtr("500"),
			AlertThresholdPercent: intPtr(80),
			AtCapAction:           options.String("alert-only"),
			PricingAdjustment:     strPtr("1.0"),
		}, "test")
	})
	assert.Equal(t, "unknown", got)
}

// TestAppBudgetSet_EmitsActorFromContext: AppBudgetSet via a Provider with a
// JWT ctx emits actor=<user> via central injection.
func TestAppBudgetSet_EmitsActorFromContext(t *testing.T) {
	got := captureEventActorByActionWithProviderHook(t, "app:budget:set",
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			kk, _ := p.Cluster.(*fake.Clientset)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			return p.AppBudgetSet("app1", structs.AppBudgetOptions{
				MonthlyCapUsd:         strPtr("500"),
				AlertThresholdPercent: intPtr(80),
				AtCapAction:           options.String("alert-only"),
				PricingAdjustment:     strPtr("1.0"),
			}, "test")
		})
	assert.Equal(t, "system-write", got)
}

// TestAppBudgetSet_AdminContext_EmitsSystemAdmin: ctx populated with the
// admin claim should propagate verbatim through ContextActor.
func TestAppBudgetSet_AdminContext_EmitsSystemAdmin(t *testing.T) {
	got := captureEventActorByActionWithProviderHook(t, "app:budget:set",
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-admin")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			kk, _ := p.Cluster.(*fake.Clientset)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			return p.AppBudgetSet("app1", structs.AppBudgetOptions{
				MonthlyCapUsd:         strPtr("500"),
				AlertThresholdPercent: intPtr(80),
				AtCapAction:           options.String("alert-only"),
				PricingAdjustment:     strPtr("1.0"),
			}, "test")
		})
	assert.Equal(t, "system-admin", got)
}

// TestAppBudgetReset_EmitsActorFromContext: reset path also threads JWT user.
func TestAppBudgetReset_EmitsActorFromContext(t *testing.T) {
	got := captureEventActorByActionWithProviderHook(t, "app:budget:reset",
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-admin")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			kk, _ := p.Cluster.(*fake.Clientset)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, p.AppBudgetSet("app1", structs.AppBudgetOptions{
				MonthlyCapUsd: strPtr("500"),
			}, "test"))
			return p.AppBudgetReset("app1", "alice@convox.com")
		})
	assert.Equal(t, "system-admin", got)
}
