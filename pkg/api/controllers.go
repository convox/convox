package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
)

func (s *Server) AppCancel(c *stdapi.Context) error {
	if err := s.hook("AppCancelValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	err := s.provider(c).WithContext(contextFrom(c)).AppCancel(name)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppCreate(c *stdapi.Context) error {
	if err := s.hook("AppCreateValidate", c); err != nil {
		return err
	}

	name := c.Value("name")

	var opts structs.AppCreateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).AppCreate(name, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) AppConfigGet(c *stdapi.Context) error {
	name := c.Var("name")
	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppConfigGet(app, name)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) AppConfigList(c *stdapi.Context) error {
	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppConfigList(app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) AppConfigSet(c *stdapi.Context) error {
	app := c.Var("app")
	name := c.Var("name")

	valaue64 := c.Value("value")

	err := s.provider(c).WithContext(contextFrom(c)).AppConfigSet(app, name, valaue64)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppDelete(c *stdapi.Context) error {
	if err := s.hook("AppDeleteValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	err := s.provider(c).WithContext(contextFrom(c)).AppDelete(name)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppDiagnose(c *stdapi.Context) error {
	app := c.Var("app")

	var opts structs.AppDiagnoseOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).AppDiagnose(app, opts)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) AppGet(c *stdapi.Context) error {
	if err := s.hook("AppGetValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppGet(name)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) AppList(c *stdapi.Context) error {
	if err := s.hook("AppListValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).AppList()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) AppLogs(c *stdapi.Context) error {
	if err := s.hook("AppLogsValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	var opts structs.LogsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).AppLogs(name, opts)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) AppManifestService(c *stdapi.Context) error {
	if err := s.hook("AppManifestServiceValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	service := c.Var("service")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppManifestService(app, service)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) AppMetrics(c *stdapi.Context) error {
	if err := s.hook("AppMetricsValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	var opts structs.MetricsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).AppMetrics(name, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

// gpuMetricsConcurrencyDefault is the default concurrency cap when the
// `gpu_metrics_max_concurrent` rack param / GPU_METRICS_MAX_CONCURRENT
// env var is unset. The TF variable defaults are mirrored here so a
// rack that never set the param still has a sane bound. When set, the
// value is read at request time so a `convox rack params set
// gpu_metrics_max_concurrent=20` followed by a chart refresh observes
// the new cap without an api pod restart.
const (
	gpuMetricsConcurrencyDefault = 10
	gpuMetricsConcurrencyMax     = 50
	gpuMetricsMaxPodsDefault     = 100
	gpuMetricsMaxPodsMax         = 500
	gpuMetricsMaxRange           = 24 * time.Hour
	gpuMetricsMinPeriod          = 5 * time.Second
	gpuMetricsMaxPointsPerSeries = 5000
	gpuMetricsMaxAggregatePoints = 50000

	maxJwtDurationHours = 8760
)

// gpuMetricsSemMu guards the lazy-init of the singleton semaphore — the
// first request reads the env var and creates the semaphore at that
// cap. Subsequent param-change-then-refresh loops keep the original
// semaphore (cap is not hot-reloadable post-init; documented in the
// param spec). A future enhancement could swap the semaphore on param
// change but the failure mode is contained: an operator who lowers the
// cap then needs the api pod to restart for the new lower cap to take
// effect (raising the cap mid-flight is also constrained to the next
// pod restart). For 3.24.6 this is acceptable.
var (
	gpuMetricsSemMu sync.Mutex
	gpuMetricsSem   chan struct{}
)

// gpuMetricsAcquireSem returns true if a slot was acquired (caller is
// responsible for calling release on completion). Returns false when
// the semaphore is at capacity — caller fails fast with 503.
func gpuMetricsAcquireSem() bool {
	sem := gpuMetricsGetSem()
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func gpuMetricsReleaseSem() {
	sem := gpuMetricsGetSem()
	select {
	case <-sem:
	default:
	}
}

func gpuMetricsGetSem() chan struct{} {
	gpuMetricsSemMu.Lock()
	defer gpuMetricsSemMu.Unlock()
	if gpuMetricsSem == nil {
		concurrency := gpuMetricsConcurrencyDefault
		if s := os.Getenv("GPU_METRICS_MAX_CONCURRENT"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				if n > gpuMetricsConcurrencyMax {
					n = gpuMetricsConcurrencyMax
				}
				concurrency = n
			}
		}
		gpuMetricsSem = make(chan struct{}, concurrency)
	}
	return gpuMetricsSem
}

// gpuMetricsResetSemForTest is a test-only helper to drop and re-init
// the semaphore (so a test can lower the cap via env var and observe
// the new bound). Not exported in the wire — only available to the
// pkg/api in-package tests.
func gpuMetricsResetSemForTest() {
	gpuMetricsSemMu.Lock()
	defer gpuMetricsSemMu.Unlock()
	gpuMetricsSem = nil
}

// gpuMetricsMaxPods reads the operator-configured pod cap from the
// GPU_METRICS_MAX_PODS env var (set by terraform from the
// gpu_metrics_max_pods rack param). Falls back to the default when
// unset; clamps to [1, gpuMetricsMaxPodsMax] when set out-of-range.
func gpuMetricsMaxPods() int {
	n := gpuMetricsMaxPodsDefault
	if s := os.Getenv("GPU_METRICS_MAX_PODS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			if v > gpuMetricsMaxPodsMax {
				v = gpuMetricsMaxPodsMax
			}
			n = v
		}
	}
	return n
}

// validateMetricsRange enforces the bounds for time-range metric
// queries: End-Start ≤ 24h, Period ≥ 5s, max points per series ≤
// 5000. End defaults to time.Now() (so the server clock — not the
// caller's clock — is the authoritative window edge) when nil.
// Mutates `opts` so handlers pass the normalized window through to
// the provider.
func validateMetricsRange(opts *structs.MetricsOptions) error {
	now := time.Now()
	if opts.End == nil {
		end := now
		opts.End = &end
	}
	if opts.Start == nil {
		// Default 30 minutes — matches the most-common dropdown value.
		start := opts.End.Add(-30 * time.Minute)
		opts.Start = &start
	}
	if !opts.End.After(*opts.Start) {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: end must be after start")
	}
	rng := opts.End.Sub(*opts.Start)
	if rng > gpuMetricsMaxRange {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: end-start must be at most 24h (got %s)", rng)
	}
	period := time.Duration(0)
	if opts.Period != nil && *opts.Period > 0 {
		period = time.Duration(*opts.Period) * time.Second
	} else {
		defP := int64(30) // 30s default step
		opts.Period = &defP
		period = 30 * time.Second
	}
	if period < gpuMetricsMinPeriod {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: period must be at least 5s (got %s)", period)
	}
	points := int64(rng / period)
	if points > gpuMetricsMaxPointsPerSeries {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: too many points per series (%d > %d); widen period or shrink range", points, gpuMetricsMaxPointsPerSeries)
	}
	return nil
}

// validateAppName runs the manifest.NameValidator regex against an
// `app` path var or `services` list element. Returns a 400 stdapi
// error when invalid, suitable for direct return from a handler.
// Centralised so SSRF / regex meta-char rejection lives in one place.
func validateAppName(name string) error {
	if !manifest.NameValidator.MatchString(name) {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: invalid name %q (must match %s)", name, manifest.NameValidator.String())
	}
	return nil
}

// ServiceMetrics returns time-range GPU metric series for a single
// service in the app. Validates app + service against
// manifest.NameValidator, enforces 24h / 5s / 5000-point bounds,
// acquires the gpu_metrics_max_concurrent semaphore (fail-fast 503
// when full), and forwards to the provider.
//
// New in 3.24.6. Pre-3.24.6 racks return 404 from the route — the
// console resolver catches that and renders the "Time-range data
// requires rack 3.24.6+" banner.
func (s *Server) ServiceMetrics(c *stdapi.Context) error {
	if err := s.hook("ServiceMetricsValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	if err := validateAppName(app); err != nil {
		return err
	}
	service := c.Var("service")
	if err := validateAppName(service); err != nil {
		return err
	}

	var opts structs.MetricsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}
	if err := validateMetricsRange(&opts); err != nil {
		return err
	}

	if !gpuMetricsAcquireSem() {
		return stdapi.Errorf(http.StatusServiceUnavailable, "metrics: server busy, retry shortly")
	}
	defer gpuMetricsReleaseSem()

	v, err := s.provider(c).WithContext(contextFrom(c)).ServiceMetrics(app, service, opts)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

// MetricsByService is the batched per-app companion to ServiceMetrics.
// One rack call → one Prom QueryRange per metric (regex alternation on
// services) → one ServiceMetricsRow per requested service in the
// response. Bounds + name validation + concurrency cap are shared with
// ServiceMetrics; an additional aggregate-points cap (services ×
// timestamps × metrics ≤ 50000) protects against the multiplicative
// blowup a single huge request could cause.
//
// `services` query param is comma-separated. Each element is name-
// validated (rejecting regex meta-chars at the boundary). Caller
// MUST sanitise; the provider trusts what we forward.
func (s *Server) MetricsByService(c *stdapi.Context) error {
	if err := s.hook("MetricsByServiceValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	if err := validateAppName(app); err != nil {
		return err
	}

	var opts structs.MetricsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}
	if err := validateMetricsRange(&opts); err != nil {
		return err
	}

	// services= comes through as a comma-joined string; split and
	// validate each element. UnmarshalOptions doesn't help here because
	// the field shape is custom (we want a query string `services=a,b,c`
	// not a repeated key).
	raw := c.Request().URL.Query().Get("services")
	services := []string{}
	if raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if err := validateAppName(s); err != nil {
				return err
			}
			services = append(services, s)
		}
	}

	if len(services) > gpuMetricsMaxPods() {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: too many services (%d > %d); reduce services list", len(services), gpuMetricsMaxPods())
	}

	// Aggregate-points cap (services × timestamps × metrics ≤ 50000)
	// protects against the multiplicative blowup. Wire-name count is
	// fixed at 8 GPU metrics today; keep the multiplier dynamic so
	// future additions don't silently grow the cap.
	wireCount := 8
	period := *opts.Period
	range_ := opts.End.Sub(*opts.Start)
	timestamps := int64(range_) / int64(time.Duration(period)*time.Second)
	if timestamps < 1 {
		timestamps = 1
	}
	aggregate := int64(len(services)) * timestamps * int64(wireCount)
	if aggregate > int64(gpuMetricsMaxAggregatePoints) {
		return stdapi.Errorf(http.StatusBadRequest, "metrics: aggregate points (%d) exceed cap (%d); reduce services / range", aggregate, gpuMetricsMaxAggregatePoints)
	}

	if !gpuMetricsAcquireSem() {
		return stdapi.Errorf(http.StatusServiceUnavailable, "metrics: server busy, retry shortly")
	}
	defer gpuMetricsReleaseSem()

	v, err := s.provider(c).WithContext(contextFrom(c)).MetricsByService(app, services, opts)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) AppUpdate(c *stdapi.Context) error {
	if err := s.hook("AppUpdateValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	var opts structs.AppUpdateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).AppUpdate(name, opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppBudgetGet(c *stdapi.Context) error {
	if err := s.hook("AppBudgetGetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	cfg, state, err := s.provider(c).WithContext(contextFrom(c)).AppBudgetGet(app)
	if err != nil {
		return err
	}

	return c.RenderJSON(map[string]interface{}{"config": cfg, "state": state})
}

func (s *Server) AppBudgetSet(c *stdapi.Context) error {
	if err := s.hook("AppBudgetSetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	ackBy := resolveAckByOverride(c, app)

	var opts structs.AppBudgetOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	// Admin gate: setting a monthly cap, at-cap action, or pricing adjustment
	// is a privileged operation. Threshold-only changes remain rw-gated via the
	// default Authorize middleware. This mirrors the Console GraphQL admin check
	// and closes the rack-direct path for non-admin callers.
	if opts.MonthlyCapUsd != nil || opts.AtCapAction != nil || opts.PricingAdjustment != nil {
		if !CanAdmin(c) {
			return stdapi.Errorf(http.StatusForbidden, "AppBudgetSet: admin role required to set budget cap")
		}
	}

	if err := s.provider(c).WithContext(contextFrom(c)).AppBudgetSet(app, opts, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppBudgetClear(c *stdapi.Context) error {
	if err := s.hook("AppBudgetClearValidate", c); err != nil {
		return err
	}

	// Admin gate: removing the budget config wipes the cap, threshold, and
	// at-cap action that an admin set. Without this gate, a non-admin caller
	// could defeat an admin-set cap by Clear+Set (the threshold-only re-set
	// has no admin gate). Mirrors the AppBudgetSet cap-mutation guard so the
	// cap lifecycle is admin-only end to end. Basic-auth callers (rack
	// password) continue to pass via SetAdminRole at authenticate-time.
	if !CanAdmin(c) {
		return stdapi.Errorf(http.StatusForbidden, "AppBudgetClear: admin role required to remove budget config")
	}

	app := c.Var("app")

	ackBy := resolveAckByOverride(c, app)

	if err := s.provider(c).WithContext(contextFrom(c)).AppBudgetClear(app, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) AppBudgetReset(c *stdapi.Context) error {
	if err := s.hook("AppBudgetResetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	ackBy := resolveAckByOverride(c, app)

	// --force-clear-cooldown is the Admin-gated escape hatch that
	// ALSO drops the 24h flap-prevention cooldown. The plain reset
	// path is CanWrite (rw) — sufficient role for the routine
	// ACTIVE→recover flow that GUI Reset buttons drive. CanAdmin is
	// enforced ONLY when force_clear_cooldown=true (the cooldown-
	// bypass path is the only one that requires elevated role).
	// User tooling parses the 403 body for migration guidance — the
	// "requires Admin role; current role is 'w'" substring is preserved.
	//
	// Both paths route to AppBudgetResetWithOptions so that plain reset
	// (post-:fired) calls restoreFromAnnotation to restart shutdown
	// services from the persisted replica counts. The flag is additive —
	// it triggers the cooldown-annotation deletion in addition to the
	// shared restore-replicas path. This matches the documented behavior
	// in docs/reference/cli/budget-reset.md (canonical post-:fired
	// recovery path) and docs/management/budget-caps.md.
	forceClear := c.Value("force_clear_cooldown") == "true"
	if forceClear && !CanAdmin(c) {
		return stdapi.Errorf(http.StatusForbidden, "AppBudgetReset --force-clear-cooldown requires Admin role; current role is 'w'. Contact rack admin or use Admin token.")
	}
	resetPeriod := c.Value("reset_period") == "true"
	opts := structs.AppBudgetResetOptions{
		ForceClearCooldown: forceClear,
		ResetPeriod:        resetPeriod,
	}
	if err := s.provider(c).WithContext(contextFrom(c)).AppBudgetResetWithOptions(app, ackBy, opts); err != nil {
		return err
	}

	return c.RenderOK()
}

// AppBudgetShutdownStateGet returns the shutdown-state annotation for
// an app. Used by the CLI banner renderer to drive the post-fired
// status display. Read-only path; standard CanRead gate.
func (s *Server) AppBudgetShutdownStateGet(c *stdapi.Context) error {
	if err := s.hook("AppBudgetShutdownStateGetValidate", c); err != nil {
		return err
	}
	app := c.Var("app")
	v, err := s.provider(c).WithContext(contextFrom(c)).AppBudgetShutdownStateGet(app)
	if err != nil {
		return err
	}
	return c.RenderJSON(v)
}

// AppBudgetSimulate runs a dry-run shutdown simulation that previews
// the planned scale-down without mutating any service. Read-only
// effect; CanWrite gate via the default Authorize middleware (POST
// → write-role check).
func (s *Server) AppBudgetSimulate(c *stdapi.Context) error {
	if err := s.hook("AppBudgetSimulateValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppBudgetSimulate(app)
	if err != nil {
		return err
	}
	return c.RenderJSON(v)
}

// AppBudgetDismissRecovery dismisses the sticky recovery banner.
// Idempotent. CanWrite gate via the default Authorize middleware.
// Renders an AppBudgetDismissRecoveryResult JSON body so the CLI can
// surface the 3-case status: dismissed / already-dismissed / no-banner.
func (s *Server) AppBudgetDismissRecovery(c *stdapi.Context) error {
	if err := s.hook("AppBudgetDismissRecoveryValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	ackBy := resolveAckByOverride(c, app)

	v, err := s.provider(c).WithContext(contextFrom(c)).AppBudgetDismissRecoveryWithResult(app, ackBy)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) AppCost(c *stdapi.Context) error {
	if err := s.hook("AppCostValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).AppCost(app)
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) BalancerList(c *stdapi.Context) error {
	if err := s.hook("BalancerListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).BalancerList(app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) BuildCreate(c *stdapi.Context) error {
	if err := s.hook("BuildCreateValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	url := c.Value("url")

	var opts structs.BuildCreateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildCreate(app, url, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) BuildExport(c *stdapi.Context) error {
	if err := s.hook("BuildExportValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")
	w := c

	err := s.provider(c).WithContext(contextFrom(c)).BuildExport(app, id, w)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) BuildGet(c *stdapi.Context) error {
	if err := s.hook("BuildGetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildGet(app, id)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) BuildImport(c *stdapi.Context) error {
	if err := s.hook("BuildImportValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	r := c

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildImport(app, r)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) BuildImportImage(c *stdapi.Context) error {
	if err := s.hook("BuildImportImageValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")
	image := c.Value("image")

	if image == "" {
		return structs.ErrUnprocessable("image param required")
	}

	var opts structs.BuildImportImageOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	if err := s.provider(c).WithContext(contextFrom(c)).BuildImportImage(app, id, image, opts); err != nil {
		return err
	}

	c.Response().WriteHeader(http.StatusAccepted)
	return nil
}

func (s *Server) BuildList(c *stdapi.Context) error {
	if err := s.hook("BuildListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	var opts structs.BuildListOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildList(app, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) BuildLogs(c *stdapi.Context) error {
	if err := s.hook("BuildLogsValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")

	var opts structs.LogsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildLogs(app, id, opts)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) BuildUpdate(c *stdapi.Context) error {
	if err := s.hook("BuildUpdateValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")

	var opts structs.BuildUpdateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).BuildUpdate(app, id, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) CapacityGet(c *stdapi.Context) error {
	if err := s.hook("CapacityGetValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).CapacityGet()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) CertificateApply(c *stdapi.Context) error {
	if err := s.hook("CertificateApplyValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	service := c.Var("service")
	id := c.Value("id")

	port, cerr := strconv.Atoi(c.Var("port"))
	if cerr != nil {
		return cerr
	}

	err := s.provider(c).WithContext(contextFrom(c)).CertificateApply(app, service, port, id)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) CertificateCreate(c *stdapi.Context) error {
	if err := s.hook("CertificateCreateValidate", c); err != nil {
		return err
	}

	pub := c.Value("pub")
	key := c.Value("key")

	var opts structs.CertificateCreateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).CertificateCreate(pub, key, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) CertificateDelete(c *stdapi.Context) error {
	if err := s.hook("CertificateDeleteValidate", c); err != nil {
		return err
	}

	id := c.Var("id")

	err := s.provider(c).WithContext(contextFrom(c)).CertificateDelete(id)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) CertificateGenerate(c *stdapi.Context) error {
	if err := s.hook("CertificateGenerateValidate", c); err != nil {
		return err
	}

	domains := strings.Split(c.Value("domains"), ",")

	var opts structs.CertificateGenerateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).CertificateGenerate(domains, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) CertificateList(c *stdapi.Context) error {
	if err := s.hook("CertificateListValidate", c); err != nil {
		return err
	}

	var opts structs.CertificateListOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).CertificateList(opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) LetsEncryptConfigGet(c *stdapi.Context) error {
	v, err := s.provider(c).WithContext(contextFrom(c)).LetsEncryptConfigGet()
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) LetsEncryptConfigApply(c *stdapi.Context) error {
	config := structs.LetsEncryptConfig{}

	if err := stdapi.UnmarshalOptions(c.Request(), &config); err != nil {
		return err
	}

	if err := json.NewDecoder(c).Decode(&config); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).LetsEncryptConfigApply(config)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) EventSend(c *stdapi.Context) error {
	if err := s.hook("EventSendValidate", c); err != nil {
		return err
	}

	action := c.Value("action")

	var opts structs.EventSendOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).EventSend(action, opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) FilesDelete(c *stdapi.Context) error {
	if err := s.hook("FilesDeleteValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")
	files := strings.Split(c.Value("files"), ",")

	err := s.provider(c).WithContext(contextFrom(c)).FilesDelete(app, pid, files)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) FilesDownload(c *stdapi.Context) error {
	if err := s.hook("FilesDownloadValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")
	file := c.Value("file")

	v, err := s.provider(c).WithContext(contextFrom(c)).FilesDownload(app, pid, file)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) FilesUpload(c *stdapi.Context) error {
	if err := s.hook("FilesUploadValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")
	r := c

	var opts structs.FileTransterOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).FilesUpload(app, pid, r, opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (*Server) Initialize(_ *stdapi.Context) error {
	return stdapi.Errorf(404, "not available via api")
}

func (s *Server) InstanceKeyroll(c *stdapi.Context) error {
	if err := s.hook("InstanceKeyrollValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).InstanceKeyroll()
	if err != nil {
		return err
	}

	return c.RenderJSON(v)
}

func (s *Server) InstanceList(c *stdapi.Context) error {
	if err := s.hook("InstanceListValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).InstanceList()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) InstanceShell(c *stdapi.Context) error {
	if err := s.hook("InstanceShellValidate", c); err != nil {
		return err
	}

	id := c.Var("id")

	var opts structs.InstanceShellOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).InstanceShell(id, stdsdk.NewAdapterWs(c.Websocket()), opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return renderStatusCode(c, v)
}

func (s *Server) InstanceTerminate(c *stdapi.Context) error {
	if err := s.hook("InstanceTerminateValidate", c); err != nil {
		return err
	}

	id := c.Var("id")

	err := s.provider(c).WithContext(contextFrom(c)).InstanceTerminate(id)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func sanitizeObjectKey(key string) (string, error) {
	cleaned := path.Clean("/" + key)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." || strings.TrimSpace(cleaned) == "" {
		return "", stdapi.Errorf(http.StatusBadRequest, "invalid object key")
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return "", stdapi.Errorf(http.StatusBadRequest, "invalid object key")
		}
	}
	return cleaned, nil
}

func (s *Server) ObjectDelete(c *stdapi.Context) error {
	if err := s.hook("ObjectDeleteValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	key, kerr := sanitizeObjectKey(c.Var("key"))
	if kerr != nil {
		return kerr
	}

	err := s.provider(c).WithContext(contextFrom(c)).ObjectDelete(app, key)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) ObjectExists(c *stdapi.Context) error {
	if err := s.hook("ObjectExistsValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	key, kerr := sanitizeObjectKey(c.Var("key"))
	if kerr != nil {
		return kerr
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ObjectExists(app, key)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ObjectFetch(c *stdapi.Context) error {
	if err := s.hook("ObjectFetchValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	key, kerr := sanitizeObjectKey(c.Var("key"))
	if kerr != nil {
		return kerr
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ObjectFetch(app, key)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) ObjectList(c *stdapi.Context) error {
	if err := s.hook("ObjectListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	prefix := c.Value("prefix")
	for _, seg := range strings.Split(prefix, "/") {
		if seg == ".." {
			return stdapi.Errorf(http.StatusBadRequest, "invalid prefix")
		}
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ObjectList(app, prefix)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ObjectStore(c *stdapi.Context) error {
	if err := s.hook("ObjectStoreValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	key := c.Var("key")
	if key != "" {
		var kerr error
		key, kerr = sanitizeObjectKey(key)
		if kerr != nil {
			return kerr
		}
	}
	r := c

	var opts structs.ObjectStoreOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ObjectStore(app, key, r, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ProcessExec(c *stdapi.Context) error {
	if err := s.hook("ProcessExecValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")
	command := c.Value("command")

	var opts structs.ProcessExecOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ProcessExec(app, pid, command, stdsdk.NewAdapterWs(c.Websocket()), opts)
	if err != nil {
		renderStatusCode(c, v)
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return renderStatusCode(c, v)
}

func (s *Server) ProcessGet(c *stdapi.Context) error {
	if err := s.hook("ProcessGetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")

	v, err := s.provider(c).WithContext(contextFrom(c)).ProcessGet(app, pid)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ProcessList(c *stdapi.Context) error {
	if err := s.hook("ProcessListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	var opts structs.ProcessListOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ProcessList(app, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ProcessLogs(c *stdapi.Context) error {
	if err := s.hook("ProcessLogsValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")

	var opts structs.LogsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ProcessLogs(app, pid, opts)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) ProcessRun(c *stdapi.Context) error {
	if err := s.hook("ProcessRunValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	service := c.Var("service")

	var opts structs.ProcessRunOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ProcessRun(app, service, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ProcessStop(c *stdapi.Context) error {
	if err := s.hook("ProcessStopValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	pid := c.Var("pid")

	err := s.provider(c).WithContext(contextFrom(c)).ProcessStop(app, pid)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) Proxy(c *stdapi.Context) error {
	if err := s.hook("ProxyValidate", c); err != nil {
		return err
	}

	host := c.Var("host")

	port, cerr := strconv.Atoi(c.Var("port"))
	if cerr != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid proxy port")
	}

	var opts structs.ProxyOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).Proxy(host, port, stdsdk.NewAdapterWs(c.Websocket()), opts)
	if err != nil {
		return err
	}

	return nil
}

// validProxyHost matches cluster-internal DNS names only:
//
//	{svc}.{app}.{ns}.local                    — internal services
//	{svc}.{ns}.svc.cluster.local              — K8s service DNS
var validProxyHost = regexp.MustCompile(
	`^[a-z0-9]([a-z0-9-]*[a-z0-9])?` +
		`(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+` +
		`\.(local|svc\.cluster\.local)$`)

var bareProxySuffix = map[string]bool{
	"local": true, "svc.cluster.local": true,
	"cluster.local": true,
}

func isAllowedProxyHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if bareProxySuffix[h] {
		return false
	}
	return validProxyHost.MatchString(h)
}

func (s *Server) ProxyHttpService(c *stdapi.Context) error {
	host := strings.TrimSpace(c.Header("X-Host"))
	if !isAllowedProxyHost(host) {
		return stdapi.Errorf(http.StatusBadRequest, "invalid proxy host")
	}

	port, cerr := strconv.Atoi(c.Header("X-Port"))
	if cerr != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid proxy port")
	}

	path := c.Var("path")

	u, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid host: %s", err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)

	req := c.Request()

	req.Host = u.Hostname()
	req.URL.Path = fmt.Sprintf("/%s", path)
	req.URL.RawQuery = c.Request().URL.RawQuery
	req.Header.Del("Authorization")

	rp.ServeHTTP(c.Response(), req)

	return nil
}

func (s *Server) RegistryAdd(c *stdapi.Context) error {
	if err := s.hook("RegistryAddValidate", c); err != nil {
		return err
	}

	server := c.Value("server")
	username := c.Value("username")
	password := c.Value("password")

	v, err := s.provider(c).WithContext(contextFrom(c)).RegistryAdd(server, username, password)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) RegistryList(c *stdapi.Context) error {
	if err := s.hook("RegistryListValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).RegistryList()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) RegistryProxy(c *stdapi.Context) error {
	if err := s.hook("RegistryProxyValidate", c); err != nil {
		return err
	}

	ctx := c

	err := s.provider(c).WithContext(contextFrom(c)).RegistryProxy(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) RegistryRemove(c *stdapi.Context) error {
	if err := s.hook("RegistryRemoveValidate", c); err != nil {
		return err
	}

	server := c.Var("server")

	err := s.provider(c).WithContext(contextFrom(c)).RegistryRemove(server)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) ReleaseCreate(c *stdapi.Context) error {
	if err := s.hook("ReleaseCreateValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	var opts structs.ReleaseCreateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ReleaseCreate(app, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ReleaseGet(c *stdapi.Context) error {
	if err := s.hook("ReleaseGetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")

	v, err := s.provider(c).WithContext(contextFrom(c)).ReleaseGet(app, id)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ReleaseList(c *stdapi.Context) error {
	if err := s.hook("ReleaseListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	var opts structs.ReleaseListOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ReleaseList(app, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ReleasePromote(c *stdapi.Context) error {
	if err := s.hook("ReleasePromoteValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	id := c.Var("id")

	var opts structs.ReleasePromoteOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).ReleasePromote(app, id, opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) ResourceConsole(c *stdapi.Context) error {
	if err := s.hook("ResourceConsoleValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")
	rw := c

	var opts structs.ResourceConsoleOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).ResourceConsole(app, name, rw, opts)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) ResourceExport(c *stdapi.Context) error {
	if err := s.hook("ResourceExportValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")

	v, err := s.provider(c).WithContext(contextFrom(c)).ResourceExport(app, name)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) ResourceGet(c *stdapi.Context) error {
	if err := s.hook("ResourceGetValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")

	v, err := s.provider(c).WithContext(contextFrom(c)).ResourceGet(app, name)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ResourceImport(c *stdapi.Context) error {
	if err := s.hook("ResourceImportValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")
	r := c

	err := s.provider(c).WithContext(contextFrom(c)).ResourceImport(app, name, r)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) ResourceList(c *stdapi.Context) error {
	if err := s.hook("ResourceListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).ResourceList(app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ServiceList(c *stdapi.Context) error {
	if err := s.hook("ServiceListValidate", c); err != nil {
		return err
	}

	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).ServiceList(app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) ServiceLogs(c *stdapi.Context) error {
	app := c.Var("app")
	name := c.Var("service")

	var opts structs.LogsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).ServiceLogs(app, name, opts)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) ServiceRestart(c *stdapi.Context) error {
	if err := s.hook("ServiceRestartValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")

	err := s.provider(c).WithContext(contextFrom(c)).ServiceRestart(app, name)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

// ServiceScaleOverrideSet toggles the per-service scale-override annotation.
// Read+Write-RBAC-gated: scale override is an operational service control
// (alongside deploy, rollback, env edit) and gates on the same per-app Write
// permission those controls use. Console3 enforces the per-app Write check
// at api/resolver/mutation.go:5744-5750; this rack-side gate matches.
// Defense-in-depth: the default Authorize middleware already enforces
// CanWrite for non-GET methods; this explicit check produces an
// endpoint-named error and acts as a backstop if middleware is bypassed.
// The provider call uses ack_by-derived actor identity for the audit event
// (mirrors AppBudgetSet pattern).
func (s *Server) ServiceScaleOverrideSet(c *stdapi.Context) error {
	if err := s.hook("ServiceScaleOverrideSetValidate", c); err != nil {
		return err
	}

	if !CanWrite(c) {
		return stdapi.Errorf(http.StatusForbidden, "ServiceScaleOverrideSet requires Read+Write role; current role does not include 'w'. Contact rack admin or use a Read+Write or Admin token.")
	}

	app := c.Var("app")
	service := c.Var("service")

	ackBy := resolveAckByOverride(c, app)

	activeStr := strings.TrimSpace(formValue(c, "active"))
	if activeStr == "" {
		return stdapi.Errorf(http.StatusBadRequest, "active form-param is required (\"true\" or \"false\")")
	}
	active, err := strconv.ParseBool(activeStr)
	if err != nil {
		return stdapi.Errorf(http.StatusBadRequest, "active must be \"true\" or \"false\": %v", err)
	}

	if err := s.provider(c).WithContext(contextFrom(c)).ServiceScaleOverrideSet(app, service, active, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) ServiceUpdate(c *stdapi.Context) error {
	if err := s.hook("ServiceUpdateValidate", c); err != nil {
		return err
	}

	app := c.Var("app")
	name := c.Var("name")

	var opts structs.ServiceUpdateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).ServiceUpdate(app, name, opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

// ServiceTriggersEnable materializes a Console-driven autoscaler on the
// named service. Same Read+Write RBAC gate as ServiceScaleOverrideSet
// (both surfaces are operational per-app writes). The opts form-param
// carries the JSON-encoded ServiceTriggersOptions; ack_by carries actor
// identity for the audit event.
func (s *Server) ServiceTriggersEnable(c *stdapi.Context) error {
	if err := s.hook("ServiceTriggersEnableValidate", c); err != nil {
		return err
	}
	if !CanWrite(c) {
		return stdapi.Errorf(http.StatusForbidden, "ServiceTriggersEnable requires Read+Write role; current role does not include 'w'. Contact rack admin or use a Read+Write or Admin token.")
	}

	app := c.Var("app")
	service := c.Var("service")

	var opts structs.ServiceTriggersOptions
	if err := json.Unmarshal([]byte(formValue(c, "opts")), &opts); err != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid opts payload: %s", err.Error())
	}

	ackBy := resolveAckByOverride(c, app)

	if err := s.provider(c).WithContext(contextFrom(c)).ServiceTriggersEnable(app, service, opts, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

// ServiceTriggersDisable clears the Console-driven autoscale override on
// the named service. Same RBAC gate as ServiceTriggersEnable.
func (s *Server) ServiceTriggersDisable(c *stdapi.Context) error {
	if err := s.hook("ServiceTriggersDisableValidate", c); err != nil {
		return err
	}
	if !CanWrite(c) {
		return stdapi.Errorf(http.StatusForbidden, "ServiceTriggersDisable requires Read+Write role; current role does not include 'w'.")
	}

	app := c.Var("app")
	service := c.Var("service")
	ackBy := resolveAckByOverride(c, app)

	if err := s.provider(c).WithContext(contextFrom(c)).ServiceTriggersDisable(app, service, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

// ServiceTriggersThresholdSet updates one trigger's threshold value on
// the CRD owned by an active Console-driven override.
func (s *Server) ServiceTriggersThresholdSet(c *stdapi.Context) error {
	if err := s.hook("ServiceTriggersThresholdSetValidate", c); err != nil {
		return err
	}
	if !CanWrite(c) {
		return stdapi.Errorf(http.StatusForbidden, "ServiceTriggersThresholdSet requires Read+Write role; current role does not include 'w'.")
	}

	app := c.Var("app")
	service := c.Var("service")
	triggerType := strings.TrimSpace(formValue(c, "type"))
	thresholdStr := strings.TrimSpace(formValue(c, "threshold"))
	if thresholdStr == "" {
		return stdapi.Errorf(http.StatusBadRequest, "threshold form-param is required")
	}
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid threshold: %s", err.Error())
	}

	ackBy := resolveAckByOverride(c, app)

	if err := s.provider(c).WithContext(contextFrom(c)).ServiceTriggersThresholdSet(app, service, triggerType, threshold, ackBy); err != nil {
		return err
	}

	return c.RenderOK()
}

func (*Server) Start(_ *stdapi.Context) error {
	return stdapi.Errorf(404, "not available via api")
}

func (s *Server) SystemGet(c *stdapi.Context) error {
	if err := s.hook("SystemGetValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemGet()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (*Server) SystemInstall(_ *stdapi.Context) error {
	return stdapi.Errorf(404, "not available via api")
}

func (s *Server) SystemLogs(c *stdapi.Context) error {
	if err := s.hook("SystemLogsValidate", c); err != nil {
		return err
	}

	var opts structs.LogsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemLogs(opts)
	if err != nil {
		return err
	}

	if c, ok := interface{}(v).(io.Closer); ok {
		defer c.Close()
	}

	if _, err := io.Copy(c, v); err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return nil
}

func (s *Server) SystemMetrics(c *stdapi.Context) error {
	if err := s.hook("SystemMetricsValidate", c); err != nil {
		return err
	}

	var opts structs.MetricsOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemMetrics(opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemJwtSignKeyRotate(c *stdapi.Context) error {
	if !CanAdmin(c) {
		return stdapi.Errorf(http.StatusForbidden, "admin role required")
	}

	_, err := s.provider(c).WithContext(contextFrom(c)).SystemJwtSignKeyRotate()
	if err != nil {
		return err
	}
	return c.RenderOK()
}

func (s *Server) SystemJwtToken(c *stdapi.Context) error {
	role := c.Value("role")
	durationInHour, err := strconv.Atoi(c.Value("durationInHour"))
	if err != nil {
		return stdapi.Errorf(http.StatusBadRequest, "invalid duration")
	}
	if durationInHour < 1 || durationInHour > maxJwtDurationHours {
		return stdapi.Errorf(http.StatusBadRequest, "duration must be between 1 and %d hours", maxJwtDurationHours)
	}

	var tk string

	switch role {
	case "read":
		tk, err = s.JwtMngr.ReadToken(time.Hour * time.Duration(durationInHour))
		if err != nil {
			return err
		}
	case "write":
		tk, err = s.JwtMngr.WriteToken(time.Hour * time.Duration(durationInHour))
		if err != nil {
			return err
		}
	case "admin":
		if !CanAdmin(c) {
			return stdapi.Errorf(http.StatusForbidden, "admin role required to mint admin tokens")
		}
		tk, err = s.JwtMngr.AdminToken(time.Hour * time.Duration(durationInHour))
		if err != nil {
			return err
		}
	default:
		return stdapi.Errorf(http.StatusBadRequest, "invalid role: must be read, write, or admin")
	}

	return c.RenderJSON(structs.SystemJwt{
		Token: tk,
	})
}

func (s *Server) SystemProcesses(c *stdapi.Context) error {
	if err := s.hook("SystemProcessesValidate", c); err != nil {
		return err
	}

	var opts structs.SystemProcessesOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemProcesses(opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemReadAccess(c *stdapi.Context) error {
	if err := s.hook("SystemProcessesValidate", c); err != nil {
		return err
	}

	var opts structs.SystemProcessesOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemProcesses(opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemReleases(c *stdapi.Context) error {
	if err := s.hook("SystemReleasesValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemReleases()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceCreate(c *stdapi.Context) error {
	if err := s.hook("SystemResourceCreateValidate", c); err != nil {
		return err
	}

	kind := c.Value("kind")

	var opts structs.ResourceCreateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceCreate(kind, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceDelete(c *stdapi.Context) error {
	if err := s.hook("SystemResourceDeleteValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	err := s.provider(c).WithContext(contextFrom(c)).SystemResourceDelete(name)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) SystemResourceGet(c *stdapi.Context) error {
	if err := s.hook("SystemResourceGetValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceGet(name)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceLink(c *stdapi.Context) error {
	if err := s.hook("SystemResourceLinkValidate", c); err != nil {
		return err
	}

	name := c.Var("name")
	app := c.Value("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceLink(name, app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceList(c *stdapi.Context) error {
	if err := s.hook("SystemResourceListValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceList()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceTypes(c *stdapi.Context) error {
	if err := s.hook("SystemResourceTypesValidate", c); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceTypes()
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceUnlink(c *stdapi.Context) error {
	if err := s.hook("SystemResourceUnlinkValidate", c); err != nil {
		return err
	}

	name := c.Var("name")
	app := c.Var("app")

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceUnlink(name, app)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (s *Server) SystemResourceUpdate(c *stdapi.Context) error {
	if err := s.hook("SystemResourceUpdateValidate", c); err != nil {
		return err
	}

	name := c.Var("name")

	var opts structs.ResourceUpdateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	v, err := s.provider(c).WithContext(contextFrom(c)).SystemResourceUpdate(name, opts)
	if err != nil {
		return err
	}

	if vs, ok := interface{}(v).(Sortable); ok {
		sort.Slice(v, vs.Less)
	}

	return c.RenderJSON(v)
}

func (*Server) SystemUninstall(_ *stdapi.Context) error {
	return stdapi.Errorf(404, "not available via api")
}

func (s *Server) SystemUpdate(c *stdapi.Context) error {
	if err := s.hook("SystemUpdateValidate", c); err != nil {
		return err
	}

	var opts structs.SystemUpdateOptions
	if err := stdapi.UnmarshalOptions(c.Request(), &opts); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).SystemUpdate(opts)
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (s *Server) KarpenterCleanup(c *stdapi.Context) error {
	if err := s.hook("KarpenterCleanupValidate", c); err != nil {
		return err
	}

	err := s.provider(c).WithContext(contextFrom(c)).KarpenterCleanup()
	if err != nil {
		return err
	}

	return c.RenderOK()
}

func (*Server) Workers(_ *stdapi.Context) error {
	return stdapi.Errorf(404, "not available via api")
}

func (s *Server) CertificateRenew(c *stdapi.Context) error {
	if err := s.hook("CertificateRenewValidate", c); err != nil {
		return err
	}

	id := c.Var("id")

	err := s.provider(c).WithContext(contextFrom(c)).CertificateRenew(id)
	if err != nil {
		return err
	}

	return c.RenderOK()
}
