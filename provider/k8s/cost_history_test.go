package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type historyDay struct {
	SpendUsd float64            `json:"spend-usd"`
	Services map[string]float64 `json:"services"`
}

func dayOf(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func seedHistory(t *testing.T, kk *fake.Clientset, ns, data string) {
	t.Helper()
	_, err := kk.CoreV1().ConfigMaps(ns).Create(context.TODO(), &ac.ConfigMap{
		ObjectMeta: am.ObjectMeta{Name: "convox-cost-history", Namespace: ns},
		Data:       map[string]string{"history.json": data},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func seedHistoryDays(t *testing.T, kk *fake.Clientset, ns string, days map[string]historyDay) {
	t.Helper()
	data, err := json.Marshal(days)
	require.NoError(t, err)
	seedHistory(t, kk, ns, string(data))
}

func readHistory(t *testing.T, kk *fake.Clientset, ns string) map[string]historyDay {
	t.Helper()
	cm, err := kk.CoreV1().ConfigMaps(ns).Get(context.TODO(), "convox-cost-history", am.GetOptions{})
	require.NoError(t, err)
	days := map[string]historyDay{}
	require.NoError(t, json.Unmarshal([]byte(cm.Data["history.json"]), &days))
	return days
}

func historyTotal(days map[string]historyDay) float64 {
	var total float64
	for _, d := range days {
		total += d.SpendUsd
	}
	return total
}

func historyApp(t *testing.T, kk *fake.Clientset, lastTick time.Time) {
	t.Helper()
	require.NoError(t, appCreate(kk, "rack1", "app1"))
	writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
		MonthStart:            startOfApril(),
		CurrentMonthSpendAsOf: lastTick,
	})
}

func stateSpend(t *testing.T, p *k8s.Provider) float64 {
	t.Helper()
	_, state, err := p.AppBudgetGet("app1")
	require.NoError(t, err)
	require.NotNil(t, state)
	return state.CurrentMonthSpendUsd
}

func TestCostHistory_TickAddsToToday(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})
		servicePodFixture(t, kk, "rack1-app1", "p2", "node2", "m5.large", map[string]string{"service": "api"})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		cm, err := kk.CoreV1().ConfigMaps("rack1-app1").Get(context.TODO(), "convox-cost-history", am.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"system": "convox", "rack": "rack1", "app": "app1", "type": "cost-history"}, cm.Labels)

		days := readHistory(t, kk, "rack1-app1")
		require.Len(t, days, 1)
		assert.InDelta(t, 0.192, days["2026-04-15"].SpendUsd, 1e-6)
		assert.InDelta(t, 0.096, days["2026-04-15"].Services["web"], 1e-6)
		assert.InDelta(t, 0.096, days["2026-04-15"].Services["api"], 1e-6)

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))
		assert.Equal(t, days, readHistory(t, kk, "rack1-app1"), "a zero delta must not change history")
	})
}

func TestCostHistory_FirstTickWritesNothing(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)))

		_, err := kk.CoreV1().ConfigMaps("rack1-app1").Get(context.TODO(), "convox-cost-history", am.GetOptions{})
		assert.True(t, ae.IsNotFound(err), "got %v", err)
	})
}

func TestCostHistory_MidnightSplit(t *testing.T) {
	for _, tc := range []struct {
		window        time.Duration
		before, after float64
	}{
		{60 * time.Minute, 0.048, 0.048},
		{45 * time.Minute, 0.024, 0.048},
	} {
		t.Run(tc.window.String(), func(t *testing.T) {
			t.Setenv("COST_TRACKING_ENABLE", "true")
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*fake.Clientset)
				now := time.Date(2026, 4, 16, 0, 30, 0, 0, time.UTC)
				historyApp(t, kk, now.Add(-tc.window))
				servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

				require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

				days := readHistory(t, kk, "rack1-app1")
				require.Len(t, days, 2)
				assert.InDelta(t, tc.before, days["2026-04-15"].SpendUsd, 1e-6)
				assert.InDelta(t, tc.before, days["2026-04-15"].Services["web"], 1e-6)
				assert.InDelta(t, tc.after, days["2026-04-16"].SpendUsd, 1e-6)
				assert.InDelta(t, tc.after, days["2026-04-16"].Services["web"], 1e-6)
			})
		})
	}
}

func TestCostHistory_ChargedWindowLastHour(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 16, 0, 30, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-3*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		days := readHistory(t, kk, "rack1-app1")
		require.Len(t, days, 2, "only the last hour's two days")
		assert.InDelta(t, stateSpend(t, p), historyTotal(days), 1e-6)
		assert.InDelta(t, days["2026-04-15"].SpendUsd, days["2026-04-16"].SpendUsd, 1e-6)
	})
}

func TestCostHistory_StateConflictCountsOnce(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

		conflicted := false
		kk.PrependReactor("update", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if conflicted {
				return false, nil, nil
			}
			conflicted = true
			return true, nil, ae.NewConflict(schema.GroupResource{Resource: "namespaces"}, "rack1-app1", fmt.Errorf("conflict"))
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))
		require.True(t, conflicted)

		days := readHistory(t, kk, "rack1-app1")
		assert.InDelta(t, 0.096, stateSpend(t, p), 1e-6)
		assert.InDelta(t, 0.096, historyTotal(days), 1e-6)
	})
}

func TestCostHistory_WriteFailureLeavesEnforcement(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 0.05, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

		kk.PrependReactor("create", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("etcd unavailable")
		})

		out := captureStdout(t)
		err := k8s.AccumulateBudgetAppForTest(p, "app1", now)
		logs := out()
		require.NoError(t, err)

		assert.Contains(t, logs, "at=alert kind=cap app=app1")
		assert.Contains(t, logs, `ns=budget_accumulator at=history_write app=app1 error="etcd unavailable"`)

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		assert.False(t, state.AlertFiredAtCap.IsZero())
		assert.InDelta(t, 0.096, state.CurrentMonthSpendUsd, 1e-6)
	})
}

func TestCostHistory_CreateRaceRetriesAsUpdate(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{"2026-04-14": {SpendUsd: 1, Services: map[string]float64{"web": 1}}})

		missed := false
		kk.PrependReactor("get", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if missed {
				return false, nil, nil
			}
			missed = true
			return true, nil, ae.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "convox-cost-history")
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		days := readHistory(t, kk, "rack1-app1")
		assert.InDelta(t, 1, days["2026-04-14"].SpendUsd, 1e-6)
		assert.InDelta(t, 0.096, days["2026-04-15"].SpendUsd, 1e-6)
	})
}

func TestCostHistory_PrunesOutsideRetention(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	t.Setenv("COST_TRACKING_HISTORY_DAYS", "31")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{
			dayOf(now.AddDate(0, 0, -31)): {SpendUsd: 1},
			dayOf(now.AddDate(0, 0, -30)): {SpendUsd: 2},
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		days := readHistory(t, kk, "rack1-app1")
		assert.NotContains(t, days, dayOf(now.AddDate(0, 0, -31)))
		assert.Contains(t, days, dayOf(now.AddDate(0, 0, -30)))
		assert.Contains(t, days, "2026-04-15")
	})
}

func TestCostHistory_SizeLimitTrimsOldest(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	t.Setenv("COST_TRACKING_HISTORY_DAYS", "400")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})

		services := map[string]float64{}
		for i := 0; i < 50; i++ {
			services[fmt.Sprintf("%s%02d", strings.Repeat("s", 61), i)] = 0.123456
		}
		seeded := map[string]historyDay{}
		for i := 0; i < 400; i++ {
			seeded[dayOf(now.AddDate(0, 0, -i))] = historyDay{SpendUsd: 6.1728, Services: services}
		}
		seedHistoryDays(t, kk, "rack1-app1", seeded)

		out := captureStdout(t)
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))
		logs := out()

		cm, err := kk.CoreV1().ConfigMaps("rack1-app1").Get(context.TODO(), "convox-cost-history", am.GetOptions{})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(cm.Data["history.json"]), 900*1024)
		assert.Contains(t, logs, "ns=budget_accumulator at=history_trimmed app=app1 days=")

		days := readHistory(t, kk, "rack1-app1")
		assert.Contains(t, days, "2026-04-15")
		assert.NotContains(t, days, dayOf(now.AddDate(0, 0, -399)), "oldest days go first")
	})
}

func TestCostHistory_FoldsBeyondFiftyServices(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		for i := 0; i < 50; i++ {
			servicePodFixture(t, kk, "rack1-app1", fmt.Sprintf("p%02d", i), fmt.Sprintf("node%02d", i), "m5.large", map[string]string{"service": fmt.Sprintf("s%02d", i)})
		}
		servicePodFixture(t, kk, "rack1-app1", "cheap", "cheap-node", "t3.micro", map[string]string{"service": "aaa"})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		day := readHistory(t, kk, "rack1-app1")["2026-04-15"]
		assert.Len(t, day.Services, 51)
		assert.NotContains(t, day.Services, "aaa", "the lowest-spend service folds")
		assert.Greater(t, day.Services["_other"], 0.0)
		var sum float64
		for _, v := range day.Services {
			sum += v
		}
		assert.InDelta(t, day.SpendUsd, sum, 1e-5)
	})
}

func TestCostHistory_NamedServiceStaysNamed(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		services := map[string]float64{"_build": 1}
		for i := 0; i < 50; i++ {
			services[fmt.Sprintf("z%02d", i)] = 1
		}
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{"2026-04-15": {SpendUsd: 51, Services: services}})
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "z07"})
		servicePodFixture(t, kk, "rack1-app1", "p2", "node2", "m5.large", map[string]string{"service": "new"})
		servicePodFixture(t, kk, "rack1-app1", "p3", "node3", "m5.large", map[string]string{"service-type": "build"})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		day := readHistory(t, kk, "rack1-app1")["2026-04-15"]
		assert.InDelta(t, 1.096, day.Services["z07"], 1e-6)
		assert.InDelta(t, 1.096, day.Services["_build"], 1e-6)
		assert.InDelta(t, 0.096, day.Services["_other"], 1e-6)
		assert.NotContains(t, day.Services, "new")
	})
}

func TestCostHistory_UnparseableIsReplaced(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		historyApp(t, kk, now.Add(-1*time.Hour))
		servicePodFixture(t, kk, "rack1-app1", "p1", "node1", "m5.large", map[string]string{"service": "web"})
		seedHistory(t, kk, "rack1-app1", "not json")

		out := captureStdout(t)
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))
		assert.Contains(t, out(), "ns=budget_accumulator at=history_parse app=app1 error=")

		days := readHistory(t, kk, "rack1-app1")
		assert.InDelta(t, 0.096, days["2026-04-15"].SpendUsd, 1e-6)
	})
}

func TestCostHistory_ResetAndClearLeaveHistory(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		historyApp(t, kk, time.Now().UTC())
		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only"})
		seeded := map[string]historyDay{dayOf(time.Now()): {SpendUsd: 3, Services: map[string]float64{"web": 3}}}
		seedHistoryDays(t, kk, "rack1-app1", seeded)

		require.NoError(t, p.AppBudgetReset("app1", "test"))
		require.NoError(t, p.AppBudgetResetWithOptions("app1", "test", structs.AppBudgetResetOptions{ResetPeriod: true}))
		require.NoError(t, p.AppBudgetClear("app1", "test"))

		assert.Equal(t, seeded, readHistory(t, kk, "rack1-app1"))
	})
}

func TestAppCostWithOptions_NoOptionsIsMonthToDate(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		historyApp(t, kk, time.Now().UTC())
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{dayOf(time.Now()): {SpendUsd: 3}})

		mtd, err := p.AppCost("app1")
		require.NoError(t, err)
		got, err := p.AppCostWithOptions("app1", structs.AppCostOptions{})
		require.NoError(t, err)
		assert.Equal(t, mtd, got)
		assert.Empty(t, got.RangeStart)
	})
}

func TestAppCostWithOptions_SumsRange(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Now().UTC()
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:               time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
			CurrentMonthSpendUsd:     10,
			CurrentMonthSpendAsOf:    now,
			WarningCount:             2,
			PerServiceSpendUsd:       map[string]float64{"web": 4, "api": 6},
			PerServiceInstanceType:   map[string]string{"web": "m5.large", "api": "c5.large"},
			PerServiceSpendByVariant: map[string]map[string]float64{"web": {"m5.large:on-demand": 4}},
		})
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{
			dayOf(now.AddDate(0, 0, -62)): {SpendUsd: 100, Services: map[string]float64{"web": 100}},
			dayOf(now.AddDate(0, 0, -2)):  {SpendUsd: 3, Services: map[string]float64{"web": 1, "api": 2}},
			dayOf(now.AddDate(0, 0, -1)):  {SpendUsd: 0.5, Services: map[string]float64{"web": 0.5}},
			dayOf(now):                    {SpendUsd: 0.25, Services: map[string]float64{"web": 0.25}},
		})

		mtd, err := p.AppCost("app1")
		require.NoError(t, err)
		require.NotEmpty(t, mtd.VariantBreakdown)

		got, err := p.AppCostWithOptions("app1", structs.AppCostOptions{
			Start: options.String(dayOf(now.AddDate(0, 0, -2))),
			End:   options.String(dayOf(now.AddDate(0, 0, -1))),
		})
		require.NoError(t, err)

		assert.InDelta(t, 3.5, got.SpendUsd, 1e-9)
		assert.Equal(t, []structs.ServiceCostLine{
			{Service: "api", SpendUsd: 2, InstanceType: "c5.large"},
			{Service: "web", SpendUsd: 1.5, InstanceType: "m5.large"},
		}, got.Breakdown)
		assert.Nil(t, got.VariantBreakdown)
		assert.Equal(t, dayOf(now.AddDate(0, 0, -2)), got.RangeStart)
		assert.Equal(t, dayOf(now.AddDate(0, 0, -1)), got.RangeEnd)
		assert.Equal(t, dayOf(now.AddDate(0, 0, -2)), got.HistoryStart, "a day outside retention is not history")
		assert.Equal(t, mtd.App, got.App)
		assert.Equal(t, mtd.AsOf, got.AsOf)
		assert.Equal(t, mtd.MonthStart, got.MonthStart)
		assert.Equal(t, mtd.WarningCount, got.WarningCount)
		assert.Equal(t, mtd.TrackingEnabled, got.TrackingEnabled)
		assert.Equal(t, mtd.PricingTableVersion, got.PricingTableVersion)

		got, err = p.AppCostWithOptions("app1", structs.AppCostOptions{Start: options.String(dayOf(now.AddDate(0, 0, -70)))})
		require.NoError(t, err)
		assert.InDelta(t, 3.75, got.SpendUsd, 1e-9, "days outside retention are not summed")
	})
}

func TestAppCostWithOptions_RangeDefaults(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		now := time.Now().UTC()
		today := dayOf(now)
		first := dayOf(now.AddDate(0, 0, -5))
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, appCreate(kk, "rack1", "app2"))
		seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{
			first: {SpendUsd: 1, Services: map[string]float64{"web": 1}},
			today: {SpendUsd: 2, Services: map[string]float64{"web": 2}},
		})

		got, err := p.AppCostWithOptions("app1", structs.AppCostOptions{Start: options.String(first)})
		require.NoError(t, err)
		assert.Equal(t, today, got.RangeEnd)
		assert.InDelta(t, 3, got.SpendUsd, 1e-9)

		got, err = p.AppCostWithOptions("app1", structs.AppCostOptions{End: options.String(today)})
		require.NoError(t, err)
		assert.Equal(t, first, got.RangeStart)
		assert.Equal(t, first, got.HistoryStart)

		before := dayOf(now.AddDate(0, 0, -10))
		got, err = p.AppCostWithOptions("app1", structs.AppCostOptions{End: options.String(before)})
		require.NoError(t, err)
		assert.Equal(t, before, got.RangeStart)
		assert.Equal(t, before, got.RangeEnd)
		assert.Zero(t, got.SpendUsd)

		future := dayOf(now.AddDate(0, 0, 3))
		got, err = p.AppCostWithOptions("app1", structs.AppCostOptions{Start: options.String(future)})
		require.NoError(t, err)
		assert.Equal(t, future, got.RangeStart)
		assert.Equal(t, future, got.RangeEnd)
		assert.Zero(t, got.SpendUsd)

		got, err = p.AppCostWithOptions("app2", structs.AppCostOptions{End: options.String(today)})
		require.NoError(t, err)
		assert.Equal(t, today, got.RangeStart)
		assert.Equal(t, today, got.RangeEnd)
		assert.Empty(t, got.HistoryStart)
		data, err := json.Marshal(got)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"breakdown":[]`)
	})
}

func TestAppCostWithOptions_UnparseableHistoryReadsEmpty(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedHistory(t, kk, "rack1-app1", "not json")

		got, err := p.AppCostWithOptions("app1", structs.AppCostOptions{End: options.String(dayOf(time.Now()))})
		require.NoError(t, err)
		assert.Empty(t, got.HistoryStart)
		assert.Zero(t, got.SpendUsd)
	})
}

func TestAppCostWithOptions_RetentionEnv(t *testing.T) {
	for _, tc := range []struct {
		env  string
		days int
	}{
		{"", 62},
		{"abc", 62},
		{"30", 62},
		{"401", 400},
		{"90", 90},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("COST_TRACKING_ENABLE", "true")
			t.Setenv("COST_TRACKING_HISTORY_DAYS", tc.env)
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*fake.Clientset)
				now := time.Now().UTC()
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				seedHistoryDays(t, kk, "rack1-app1", map[string]historyDay{
					dayOf(now.AddDate(0, 0, -tc.days)):     {SpendUsd: 1},
					dayOf(now.AddDate(0, 0, 1-tc.days)):    {SpendUsd: 1},
					dayOf(now.AddDate(0, 0, 1-tc.days-30)): {SpendUsd: 1},
				})

				got, err := p.AppCostWithOptions("app1", structs.AppCostOptions{End: options.String(dayOf(now))})
				require.NoError(t, err)
				assert.Equal(t, dayOf(now.AddDate(0, 0, 1-tc.days)), got.HistoryStart)
			})
		})
	}
}

func TestAppCostWithOptions_BadInput(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		for _, tc := range []struct {
			opts structs.AppCostOptions
			msg  string
		}{
			{structs.AppCostOptions{Start: options.String("2026-13-01")}, "start must be YYYY-MM-DD"},
			{structs.AppCostOptions{End: options.String("10/01/2026")}, "end must be YYYY-MM-DD"},
			{structs.AppCostOptions{Start: options.String("2026-10-02"), End: options.String("2026-10-01")}, "start must not be after end"},
		} {
			_, err := p.AppCostWithOptions("app1", tc.opts)
			require.Error(t, err)
			assert.Equal(t, tc.msg, err.Error())
			var he *structs.HttpError
			require.True(t, errors.As(err, &he))
			assert.Equal(t, 400, he.Code())
		}
	})
}
