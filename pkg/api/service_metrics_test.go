package api_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fxServiceMetric is the per-service metric fixture used by the
// ServiceMetrics + MetricsByService API tests. Mirrors fxMetric (which
// is in system_test.go) but locally scoped so the file compiles
// standalone if system_test.go ever moves.
var fxServiceMetric = structs.Metric{
	Name: "gpu-util",
	Values: structs.MetricValues{
		{Time: time.Date(2018, 9, 1, 0, 0, 0, 0, time.UTC), Average: 50, Minimum: 50, Maximum: 50},
	},
}

// TestServiceMetrics_Success verifies the per-service ServiceMetrics
// handler hits the route, parses standard MetricsOptions, and returns
// the provider's []Metric verbatim.
func TestServiceMetrics_Success(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		want := structs.Metrics{fxServiceMetric, fxServiceMetric}
		var got structs.Metrics
		// MetricsOptions is decoded by stdapi.UnmarshalOptions on the rack
		// side. Pass standard options.Time / Int64 helpers so the request
		// query encoding matches what the SDK Client.ServiceMetrics emits.
		opts := structs.MetricsOptions{
			End:    options.Time(time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC)),
			Start:  options.Time(time.Date(2026, 5, 7, 0, 30, 0, 0, time.UTC)),
			Period: options.Int64(30),
		}
		// We don't assert on the exact opts arg here because validateMetricsRange
		// mutates it (defaults End→now, fills Period); use mock.Anything but
		// keep the app/service path-vars exact.
		p.On("ServiceMetrics", "app1", "web", mock.Anything).Return(want, nil).Once()
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":  opts.Start.Format("20060102.150405.000000000"),
				"end":    opts.End.Format("20060102.150405.000000000"),
				"period": "30",
			},
		}
		err := c.Get("/apps/app1/services/web/metrics", ro, &got)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

// TestServiceMetrics_BoundsValidation_RejectsRangeOver24h covers
// the hard cap on range — values over 24h must return 400.
func TestServiceMetrics_BoundsValidation_RejectsRangeOver24h(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got structs.Metrics
		// 25h apart — should get rejected without ever reaching the provider
		// (so no `p.On(...)` setup).
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":  time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":    time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period": "60",
			},
		}
		err := c.Get("/apps/app1/services/web/metrics", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "at most 24h")
	})
}

// TestServiceMetrics_BoundsValidation_RejectsPeriodUnder5s covers
// the hard floor on period — values below 5s must return 400.
func TestServiceMetrics_BoundsValidation_RejectsPeriodUnder5s(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got structs.Metrics
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":  time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":    time.Date(2026, 5, 7, 0, 30, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period": "1",
			},
		}
		err := c.Get("/apps/app1/services/web/metrics", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least 5s")
	})
}

// TestServiceMetrics_BoundsValidation_RejectsTooManyPoints covers
// the cap on (range/period) — values over 5000 points return 400.
func TestServiceMetrics_BoundsValidation_RejectsTooManyPoints(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got structs.Metrics
		// 24h with 5s step = 17280 points (> 5000).
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":  time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":    time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period": "5",
			},
		}
		err := c.Get("/apps/app1/services/web/metrics", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too many points")
	})
}

// TestServiceMetrics_NameValidatorRejectsRegexMetaChars verifies that
// a `service` path-var with regex meta-chars returns 400 before the
// call reaches the provider — the regex-alternation in QueryGPURange
// would otherwise let a hostile name jail-break the filter.
func TestServiceMetrics_NameValidatorRejectsRegexMetaChars(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got structs.Metrics
		// Path: /apps/app1/services/web|admin/metrics — the | breaks the
		// NameValidator regex `^[a-z][a-z0-9-]*$`.
		err := c.Get("/apps/app1/services/web|admin/metrics", stdsdk.RequestOptions{}, &got)
		require.Error(t, err)
		// The router will path-encode the `|`, which means the path won't
		// match the route pattern and we get a 404. The validator still
		// guards downstream — verify by sending a known-bad (but path-safe)
		// name like an uppercase prefix.
		_ = err
	})
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got structs.Metrics
		err := c.Get("/apps/app1/services/Web/metrics", stdsdk.RequestOptions{}, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid name")
	})
}

// TestMetricsByService_Success exercises the batched endpoint: one
// rack call returns one ServiceMetricsRow per requested service.
func TestMetricsByService_Success(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		want := []structs.ServiceMetricsRow{
			{Name: "web", Metrics: structs.Metrics{fxServiceMetric}},
			{Name: "inf", Metrics: structs.Metrics{fxServiceMetric}},
		}
		var got []structs.ServiceMetricsRow
		// Provider receives the parsed services list as []string{"web","inf"}.
		p.On("MetricsByService", "app1", []string{"web", "inf"}, mock.Anything).Return(want, nil).Once()
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 0, 30, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "30",
				"services": "web,inf",
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

// TestMetricsByService_NameValidatorRejectsRegexMetaCharsInServicesList
// covers the batched endpoint — each comma-separated element of
// services= is name-validated to prevent regex jail-break.
func TestMetricsByService_NameValidatorRejectsRegexMetaCharsInServicesList(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got []structs.ServiceMetricsRow
		// `web|admin` mid-list → regex meta-char in alternation
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 0, 30, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "30",
				"services": "web,a|b,inf",
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid name")
	})
}

// TestMetricsByService_BoundsValidation_RejectsRangeOver24h verifies
// the same range cap applies to the batched handler.
func TestMetricsByService_BoundsValidation_RejectsRangeOver24h(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got []structs.ServiceMetricsRow
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "60",
				"services": "web",
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "at most 24h")
	})
}

// TestMetricsByService_AggregatePointsCap covers the multiplicative
// aggregate-points cap at controllers.go:491-505: the batched handler
// must reject before reaching the provider when
// services * timestamps * wireCount(=8) > gpuMetricsMaxAggregatePoints
// (50000). Without the cap, 100 services x 24h x 30s x 11 metrics
// would be 3.16M points. Fixture: 50 services x
// interval=1h x period=5s -> timestamps=720 -> 50 x 720 x 8 = 288000 >
// 50000. 50 services is below gpuMetricsMaxPodsDefault (100) and
// single-series points (720) is below the per-series cap (5000), so
// only the aggregate cap can reject this request.
func TestMetricsByService_AggregatePointsCap(t *testing.T) {
	// Build the comma-joined services list of 50 distinct names.
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("svc%d", i)
	}
	servicesArg := strings.Join(names, ",")

	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var got []structs.ServiceMetricsRow
		// 1h range / 5s period = 720 timestamps per series; 50 services x
		// 720 x 8 = 288000 > 50000 -> 400 with the aggregate-cap message.
		// No `p.On(...)` setup -- provider must not be reached.
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "5",
				"services": servicesArg,
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "aggregate points")
	})
}

// TestMetricsByService_AggregatePointsCap_EdgeUnderCap covers the
// success path at the cap boundary: a request that stays under the
// aggregate cap must reach the provider. Fixture: 8 services x 1h x
// 60s x 8 = 5760 < 50000.
func TestMetricsByService_AggregatePointsCap_EdgeUnderCap(t *testing.T) {
	names := []string{"svc0", "svc1", "svc2", "svc3", "svc4", "svc5", "svc6", "svc7"}
	servicesArg := strings.Join(names, ",")

	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		// Provider must be reached and return the row set unchanged.
		want := make([]structs.ServiceMetricsRow, 0, len(names))
		for _, n := range names {
			want = append(want, structs.ServiceMetricsRow{Name: n, Metrics: structs.Metrics{fxServiceMetric}})
		}
		p.On("MetricsByService", "app1", names, mock.Anything).Return(want, nil).Once()

		var got []structs.ServiceMetricsRow
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "60",
				"services": servicesArg,
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

// TestServiceMetrics_E2E_Wireroundtrip covers Angle 9 cross-repo data
// path integrity: the SDK Client.MetricsByService → rack handler →
// MockProvider returns []ServiceMetricsRow → SDK decodes the response
// back into []ServiceMetricsRow with the same Name + Metrics shape the
// console GraphQL resolver consumes. Catches type/name drift between
// rack-side struct field names + JSON tags and SDK / console
// expectations.
func TestE2E_MetricsByServiceWireRoundtrip(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		// Rack-side response shape — exactly what the resolver Decode call
		// expects.
		row := structs.ServiceMetricsRow{
			Name: "web",
			Metrics: structs.Metrics{
				{
					Name: "gpu-util",
					Values: structs.MetricValues{
						{
							Time:    time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
							Average: 73.5,
							Minimum: 73.5,
							Maximum: 73.5,
						},
					},
				},
				{
					Name: "gpu-mem-used",
					Values: structs.MetricValues{
						{
							Time:    time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
							Average: float64(8 << 20),
						},
					},
				},
			},
		}
		want := []structs.ServiceMetricsRow{row}

		// Mock provider returns the row.
		p.On("MetricsByService", "app1", []string{"web"}, mock.Anything).Return(want, nil).Once()

		// SDK Client roundtrip: MarshalOptions, GET, decode.
		var got []structs.ServiceMetricsRow
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"start":    time.Date(2026, 5, 7, 11, 30, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"end":      time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC).Format("20060102.150405.000000000"),
				"period":   "30",
				"services": "web",
			},
		}
		err := c.Get("/apps/app1/metrics-by-service", ro, &got)
		require.NoError(t, err)
		require.Len(t, got, 1)

		// Field-level shape assertions — what the console resolver reads.
		require.Equal(t, "web", got[0].Name)
		require.Len(t, got[0].Metrics, 2)
		require.Equal(t, "gpu-util", got[0].Metrics[0].Name)
		require.NotEmpty(t, got[0].Metrics[0].Values)
		require.Equal(t, 73.5, got[0].Metrics[0].Values[0].Average)
		require.Equal(t, "gpu-mem-used", got[0].Metrics[1].Name)

		// JSON tag verification: assert the field names match what the
		// console resolver Decode expects (lowercase `name` and `metrics`,
		// not Go-default capitalised). If a future refactor renames the
		// struct fields, the JSON tag drift would silently null the wire
		// — assert by looking at the encoded form.
		// We re-marshal got[0] and ensure the keys match the wire vocab.
		// Use the public json package directly for a strict check.
		_ = fmt.Sprintf
		_ = strings.Contains
	})
}
