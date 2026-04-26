package manifest

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	ValidNameDescription = "must contain only lowercase alphanumeric and dashes"
)

var (
	NameValidator         = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	prometheusMetricNameR = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
)

func (m *Manifest) validate() []error {
	errs := []error{}

	for i := range m.Configs {
		if err := m.Configs[i].Validate(); err != nil {
			errs = append(errs, err)
			break
		}
	}

	errs = append(errs, m.validateBalancers()...)
	errs = append(errs, m.validateBudget()...)
	errs = append(errs, m.validateEnv()...)
	errs = append(errs, m.validateResources()...)
	errs = append(errs, m.validateServices()...)
	errs = append(errs, m.validateTimers()...)

	return errs
}

func (m *Manifest) validateBalancers() []error {
	errs := []error{}

	for _, b := range m.Balancers {
		if len(b.Ports) == 0 {
			errs = append(errs, fmt.Errorf("balancer %s has no ports", b.Name))
		}

		if b.Service == "" {
			errs = append(errs, fmt.Errorf("balancer %s has blank service", b.Name))
		} else {
			serviceFound := false

			for _, s := range m.Services {
				if s.Name == b.Service {
					serviceFound = true
					break
				}
			}

			if !serviceFound {
				errs = append(errs, fmt.Errorf("balancer %s refers to unknown service %s", b.Name, b.Service))
			}
		}

		for _, w := range b.Whitelist {
			if _, _, err := net.ParseCIDR(w); err != nil {
				errs = append(errs, fmt.Errorf("balancer %s whitelist %s is not a valid cidr range", b.Name, w))
			}
		}
	}

	return errs
}

func (m *Manifest) validateEnv() []error {
	errs := []error{}

	for _, s := range m.Services {
		if _, err := m.ServiceEnvironment(s.Name); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (m *Manifest) validateResources() []error {
	errs := []error{}

	for _, r := range m.Resources {
		if !NameValidator.MatchString(r.Name) {
			errs = append(errs, fmt.Errorf("resource name %s invalid, %s", r.Name, ValidNameDescription))
		}

		if strings.TrimSpace(r.Type) == "" {
			errs = append(errs, fmt.Errorf("resource %q has blank type", r.Name))
		}
	}

	return errs
}

func (m *Manifest) validateServices() []error {
	errs := []error{}

	configMap := map[string]struct{}{}
	for i := range m.Configs {
		configMap[m.Configs[i].Id] = struct{}{}
	}

	for _, s := range m.Services {
		if !NameValidator.MatchString(s.Name) {
			errs = append(errs, fmt.Errorf("service name %s invalid, %s", s.Name, ValidNameDescription))
		}

		if s.Deployment.Minimum < 0 {
			errs = append(errs, fmt.Errorf("service %s deployment minimum can not be less than 0", s.Name))
		}

		if s.Deployment.Minimum > 100 {
			errs = append(errs, fmt.Errorf("service %s deployment minimum can not be greater than 100", s.Name))
		}

		if s.Deployment.Maximum < 100 {
			errs = append(errs, fmt.Errorf("service %s deployment maximum can not be less than 100", s.Name))
		}

		if s.Deployment.Maximum > 200 {
			errs = append(errs, fmt.Errorf("service %s deployment maximum can not be greater than 200", s.Name))
		}

		if s.Internal && s.InternalRouter {
			errs = append(errs, fmt.Errorf("service %s can not have both internal and internalRouter set as true", s.Name))
		}

		for _, r := range s.ResourcesName() {
			if _, err := m.Resource(r); err != nil {
				if strings.HasPrefix(err.Error(), "no such resource") {
					errs = append(errs, fmt.Errorf("service %s references a resource that does not exist: %s", s.Name, r))
				}
			}
		}

		for i := range s.VolumeOptions {
			if err := s.VolumeOptions[i].Validate(); err != nil {
				errs = append(errs, err)
			}
		}

		if err := s.SecurityContext.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("service %s: %s", s.Name, err))
		}

		seenRegistries := map[string]int{}
		for i := range s.ImagePullSecrets {
			if err := s.ImagePullSecrets[i].Validate(); err != nil {
				errs = append(errs, fmt.Errorf("service %s imagePullSecrets[%d]: %s", s.Name, i, err))
				continue
			}
			reg := strings.ToLower(s.ImagePullSecrets[i].Registry)
			if prev, has := seenRegistries[reg]; has {
				errs = append(errs, fmt.Errorf("service %s imagePullSecrets[%d]: duplicate registry %q (also declared at index %d); each registry may be declared at most once per service", s.Name, i, s.ImagePullSecrets[i].Registry, prev))
				continue
			}
			seenRegistries[reg] = i
		}

		for i := range s.ConfigMounts {
			cm := &s.ConfigMounts[i]
			if err := cm.Validate(); err != nil {
				errs = append(errs, err)
			}

			if _, has := configMap[cm.Id]; !has {
				errs = append(errs, fmt.Errorf("config id: '%s' not found", cm.Id))
			}
		}

		errs = append(errs, validateServiceScale(&s)...)
	}
	return errs
}

func validateServiceScale(s *Service) []error {
	var errs []error

	if s.Agent.Enabled && (s.Scale.Autoscale.IsEnabled() || s.Scale.IsKedaEnabled()) {
		errs = append(errs, fmt.Errorf("service %s: agent services render as DaemonSet and cannot use scale.autoscale or scale.keda", s.Name))
	}

	// Validate against EFFECTIVE Count (populated by ApplyDefaults from both
	// the legacy scale.count shape and the new scale.min/scale.max pointer
	// shape). Checking pointer-only would let legacy scale.count: 1-5 callers
	// bypass every autoscale-aware rule.
	effMin := s.Scale.Count.Min
	effMax := s.Scale.Count.Max

	if effMin < 0 {
		errs = append(errs, fmt.Errorf("service %s: scale.min must be >= 0", s.Name))
	}
	if effMax < 0 {
		errs = append(errs, fmt.Errorf("service %s: scale.max must be >= 0", s.Name))
	}
	if effMax < effMin {
		errs = append(errs, fmt.Errorf("service %s: scale.max must be >= scale.min", s.Name))
	}

	a := s.Scale.Autoscale
	if !a.IsEnabled() {
		return errs
	}

	if effMax < 1 {
		errs = append(errs, fmt.Errorf("service %s: scale.max must be >= 1 when autoscale is enabled", s.Name))
	}
	if effMax == effMin && effMax >= 1 {
		errs = append(errs, fmt.Errorf("service %s: scale.max must be > scale.min when autoscale is enabled (ScaledObject would be a no-op)", s.Name))
	}

	if a.Cpu != nil && invalidPercent(a.Cpu.Threshold) {
		errs = append(errs, fmt.Errorf("service %s: scale.autoscale.cpu.threshold must be between 1 and 100", s.Name))
	}
	if a.Memory != nil && invalidPercent(a.Memory.Threshold) {
		errs = append(errs, fmt.Errorf("service %s: scale.autoscale.memory.threshold must be between 1 and 100", s.Name))
	}
	if a.GpuUtilization != nil {
		if invalidPercent(a.GpuUtilization.Threshold) {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.gpuUtilization.threshold must be > 0 and <= 100", s.Name))
		}
		if s.Scale.Gpu.Count == 0 {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.gpuUtilization requires scale.gpu.count >= 1", s.Name))
		}
	}
	if a.QueueDepth != nil {
		if math.IsNaN(a.QueueDepth.Threshold) || a.QueueDepth.Threshold <= 0 {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.queueDepth.threshold must be > 0", s.Name))
		}
	}

	errs = append(errs, validateAutoscaleMode(s.Name, "cpu", a.Cpu)...)
	errs = append(errs, validateAutoscaleMode(s.Name, "memory", a.Memory)...)
	errs = append(errs, validateAutoscaleMode(s.Name, "gpuUtilization", a.GpuUtilization)...)
	errs = append(errs, validateAutoscaleMode(s.Name, "queueDepth", a.QueueDepth)...)

	for i, trig := range a.Custom {
		if trig.Name != "" && !NameValidator.MatchString(trig.Name) {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.custom[%d].name %q invalid, %s", s.Name, i, trig.Name, ValidNameDescription))
		}
		if strings.HasPrefix(strings.ToLower(trig.Name), "convox-") {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.custom[%d].name: %q uses reserved prefix 'convox-'", s.Name, i, trig.Name))
		}
		if trig.AuthenticationRef != nil && trig.AuthenticationRef.Kind == "ClusterTriggerAuthentication" {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.custom[%d].authenticationRef.kind: ClusterTriggerAuthentication is not permitted", s.Name, i))
		}
	}

	if effMin == 0 && !autoscaleCanScaleToZero(a) {
		errs = append(errs, fmt.Errorf(
			"service %s: scale.min: 0 combined with only always-active autoscale triggers (cpu/memory/cron) will never scale to zero; "+
				"KEDA's cpu/memory/cron scalers are always active. Add scale.autoscale.queueDepth, scale.autoscale.gpuUtilization, "+
				"or a scale.autoscale.custom[] prometheus/external trigger, or raise scale.min to 1.",
			s.Name,
		))
	}

	return errs
}

func invalidPercent(v float64) bool {
	return math.IsNaN(v) || v <= 0 || v > 100
}

// autoscaleCanScaleToZero returns true when the autoscale spec contains at
// least one trigger type that KEDA's scale_handler will let drop to zero.
// KEDA's cpu, memory, and cron scalers are always-active (IsActive=true
// unconditionally), so a service configured with only those never scales to
// zero regardless of scale.min. Returns false when the config is made up
// entirely of always-active triggers.
func autoscaleCanScaleToZero(a *ServiceAutoscale) bool {
	if a == nil {
		return false
	}
	if a.GpuUtilization != nil || a.QueueDepth != nil {
		return true
	}
	for i := range a.Custom {
		t := strings.ToLower(a.Custom[i].Type)
		if t != "cpu" && t != "memory" && t != "cron" {
			return true
		}
	}
	return false
}

func validateAutoscaleMode(service, field string, t *AutoscaleMode) []error {
	if t == nil {
		return nil
	}
	var errs []error
	if t.PrometheusUrl != "" {
		if err := validatePrometheusURL(t.PrometheusUrl); err != nil {
			errs = append(errs, fmt.Errorf("service %s: scale.autoscale.%s.prometheusUrl %s", service, field, err))
		}
	}
	if t.MetricName != "" && !prometheusMetricNameR.MatchString(t.MetricName) {
		errs = append(errs, fmt.Errorf("service %s: scale.autoscale.%s.metricName %q must match %s", service, field, t.MetricName, prometheusMetricNameR.String()))
	}
	return errs
}

func validatePrometheusURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("is not a valid URL: %s", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must use http or https scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host")
	}
	return nil
}

func (m *Manifest) validateTimers() []error {
	errs := []error{}

	for _, t := range m.Timers {
		if !NameValidator.MatchString(t.Name) {
			errs = append(errs, fmt.Errorf("timer name %s invalid, %s", t.Name, ValidNameDescription))
		}

		if _, err := m.Service(t.Service); err != nil {
			if strings.HasPrefix(err.Error(), "no such service") {
				errs = append(errs, fmt.Errorf("timer %s references a service that does not exist: %s", t.Name, t.Service))
			}
		}

		if strings.Contains(t.Schedule, "?") {
			errs = append(errs, fmt.Errorf("timer %s invalid, schedule cannot contain ?", t.Name))
		}
	}

	return errs
}

// validateBudget enforces the 11 cross-field rules from Set G v2 spec
// §3.1. 10 rules are HARD-FAIL; rule 3 is WARN-only (printed to
// stderr, parse succeeds).
func (m *Manifest) validateBudget() []error {
	b := m.Budget
	errs := []error{}

	// Skip the entire pass when no budget block is configured. This
	// preserves backward compatibility -- manifests that never declare
	// budget see zero behavior change.
	if b.AtCapAction == "" && b.MonthlyCapUsd == 0 && b.AtCapWebhookUrl == "" &&
		len(b.NeverAutoShutdown) == 0 && b.ShutdownOrder == "" &&
		b.NotifyBeforeMinutes == 0 && b.ShutdownGracePeriod == "" &&
		b.RecoveryMode == "" {
		return errs
	}

	if b.AtCapAction != "" {
		switch b.AtCapAction {
		case "alert-only", "block-new-deploys", "auto-shutdown":
		default:
			errs = append(errs, fmt.Errorf(
				"budget.atCapAction must be one of %q, %q, %q; got %q",
				"alert-only", "block-new-deploys", "auto-shutdown", b.AtCapAction))
		}
	}

	autoShutdown := b.AtCapAction == "auto-shutdown"

	// Rule 1: refuse auto-shutdown without atCapWebhookUrl
	if autoShutdown && strings.TrimSpace(b.AtCapWebhookUrl) == "" {
		errs = append(errs, fmt.Errorf(
			"budget.atCapAction %q requires budget.atCapWebhookUrl to be set; auto-shutdown silently killing services without notification is rejected by design",
			"auto-shutdown"))
	}
	// Rule 2: refuse auto-shutdown without monthlyCapUsd > 0
	if autoShutdown && !(b.MonthlyCapUsd > 0) {
		errs = append(errs, fmt.Errorf(
			"budget.atCapAction %q requires budget.monthlyCapUsd > 0; cannot shut down on cap breach without a cap",
			"auto-shutdown"))
	}

	// Rule 3 (WARN only): unknown service in neverAutoShutdown
	if len(b.NeverAutoShutdown) > 0 {
		known := map[string]bool{}
		for i := range m.Services {
			known[m.Services[i].Name] = true
		}
		for _, name := range b.NeverAutoShutdown {
			if !known[name] {
				fmt.Fprintf(os.Stderr,
					"WARNING: budget.neverAutoShutdown contains %q which is not a service in this manifest; ignoring at runtime\n",
					name)
			}
		}
	}

	// Rule 4: notifyBeforeMinutes range
	if b.NotifyBeforeMinutes != 0 {
		if b.NotifyBeforeMinutes < 5 || b.NotifyBeforeMinutes > 1440 {
			errs = append(errs, fmt.Errorf(
				"budget.notifyBeforeMinutes must be between 5 and 1440; got %d",
				b.NotifyBeforeMinutes))
		}
	}

	// Rule 5/6: shutdownGracePeriod parse + range
	if strings.TrimSpace(b.ShutdownGracePeriod) != "" {
		d, err := time.ParseDuration(b.ShutdownGracePeriod)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"budget.shutdownGracePeriod must be a Go duration string (e.g. %q, %q); got %q",
				"5m", "30s", b.ShutdownGracePeriod))
		} else {
			if d < 0 || d > time.Hour {
				errs = append(errs, fmt.Errorf(
					"budget.shutdownGracePeriod must be between 0s and 1h; got %s",
					b.ShutdownGracePeriod))
			}
		}
	}

	// Rule 7/7a: shutdownOrder enum
	if b.ShutdownOrder != "" {
		switch b.ShutdownOrder {
		case "largest-cost", "newest":
		case "priority-annotation":
			errs = append(errs, fmt.Errorf(
				"budget.shutdownOrder %q reserved for 3.24.7; use %q or %q",
				"priority-annotation", "largest-cost", "newest"))
		default:
			errs = append(errs, fmt.Errorf(
				"budget.shutdownOrder must be one of %q, %q; got %q",
				"largest-cost", "newest", b.ShutdownOrder))
		}
	}

	// Rule 8/9: recoveryMode enum
	if b.RecoveryMode != "" {
		switch b.RecoveryMode {
		case "auto-on-reset", "manual":
		case "scheduled":
			errs = append(errs, fmt.Errorf(
				"budget.recoveryMode %q reserved for 3.25.0; use %q or %q",
				"scheduled", "auto-on-reset", "manual"))
		default:
			errs = append(errs, fmt.Errorf(
				"budget.recoveryMode must be one of %q, %q; got %q",
				"auto-on-reset", "manual", b.RecoveryMode))
		}
	}

	if autoShutdown {
		// Rule 10a: timer-only (no services)
		if len(m.Services) == 0 {
			errs = append(errs, fmt.Errorf(
				"budget.atCapAction %q requires at least one service in the services block; timer-only apps cannot be auto-shut. Use %q or remove auto-shutdown.",
				"auto-shutdown", "block-new-deploys"))
		} else {
			// Rule 10: every service exempt. Exempt entries that do not
			// match a service name (rule 3) are silently skipped at
			// runtime, so they do not count toward "all eligible
			// excluded" here.
			exempt := map[string]bool{}
			knownService := map[string]bool{}
			for i := range m.Services {
				knownService[m.Services[i].Name] = true
			}
			for _, s := range b.NeverAutoShutdown {
				if knownService[s] {
					exempt[s] = true
				}
			}
			anyEligible := false
			for i := range m.Services {
				if !exempt[m.Services[i].Name] {
					anyEligible = true
					break
				}
			}
			if !anyEligible {
				errs = append(errs, fmt.Errorf(
					"budget.atCapAction %q with all services listed in neverAutoShutdown leaves no services eligible; either remove a service from neverAutoShutdown or use atCapAction %q",
					"auto-shutdown", "block-new-deploys"))
			}
		}
	}

	return errs
}
