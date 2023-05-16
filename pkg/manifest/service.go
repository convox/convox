package manifest

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type Service struct {
	Name string `yaml:"-"`

	Agent          ServiceAgent          `yaml:"agent,omitempty"`
	Annotations    ServiceAnnotations    `yaml:"annotations,omitempty"`
	Build          ServiceBuild          `yaml:"build,omitempty"`
	Certificate    Certificate           `yaml:"certificate,omitempty"`
	Command        string                `yaml:"command,omitempty"`
	Deployment     ServiceDeployment     `yaml:"deployment,omitempty"`
	Domains        ServiceDomains        `yaml:"domain,omitempty"`
	Drain          int                   `yaml:"drain,omitempty"`
	Environment    Environment           `yaml:"environment,omitempty"`
	Health         ServiceHealth         `yaml:"health,omitempty"`
	Image          string                `yaml:"image,omitempty"`
	Init           bool                  `yaml:"init,omitempty"`
	Internal       bool                  `yaml:"internal,omitempty"`
	InternalRouter bool                  `yaml:"internalRouter,omitempty"`
	Lifecycle      ServiceLifecycle      `yaml:"lifecycle,omitempty"`
	Port           ServicePortScheme     `yaml:"port,omitempty"`
	Ports          []ServicePortProtocol `yaml:"ports,omitempty"`
	Privileged     bool                  `yaml:"privileged,omitempty"`
	Resources      []string              `yaml:"resources,omitempty"`
	Scale          ServiceScale          `yaml:"scale,omitempty"`
	Singleton      bool                  `yaml:"singleton,omitempty"`
	Sticky         bool                  `yaml:"sticky,omitempty"`
	Termination    ServiceTermination    `yaml:"termination,omitempty"`
	Test           string                `yaml:"test,omitempty"`
	Timeout        int                   `yaml:"timeout,omitempty"`
	Tls            ServiceTls            `yaml:"tls,omitempty"`
	Volumes        []string              `yaml:"volumes,omitempty"`
	Whitelist      string                `yaml:"whitelist,omitempty"`
}

type Services []Service

type Certificate struct {
	Duration string `yaml:"duration,omitempty"`
}

type ServiceAgent struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

type ServiceAnnotations []string

type ServiceBuild struct {
	Args     []string `yaml:"args,omitempty"`
	Manifest string   `yaml:"manifest,omitempty"`
	Path     string   `yaml:"path,omitempty"`
}

type ServiceDeployment struct {
	Maximum int `yaml:"maximum,omitempty"`
	Minimum int `yaml:"minimum,omitempty"`
}

type ServiceDomains []string

type ServiceHealth struct {
	Grace    int
	Interval int
	Path     string
	Timeout  int
}

type ServiceLifecycle struct {
	PreStop   string `yaml:"preStop,omitempty"`
	PostStart string `yaml:"postStart,omitempty"`
}

type ServicePortProtocol struct {
	Port     int    `yaml:"port,omitempty"`
	Protocol string `yaml:"protocol,omitempty"`
}

type ServicePortScheme struct {
	Port   int    `yaml:"port,omitempty"`
	Scheme string `yaml:"scheme,omitempty"`
}

type ServiceScale struct {
	Count   ServiceScaleCount
	Cpu     int
	Gpu     ServiceScaleGpu `yaml:"gpu,omitempty"`
	Memory  int
	Targets ServiceScaleTargets `yaml:"targets,omitempty"`
}

type ServiceScaleCount struct {
	Min int
	Max int
}
type ServiceScaleExternalMetric struct {
	AverageValue *float64          `yaml:"averageValue,omitempty"`
	MatchLabels  map[string]string `yaml:"matchLabels,omitempty"`
	Name         string            `yaml:"name"`
	Value        *float64          `yaml:"value,omitempty"`
}

type ServiceScaleExternalMetrics []ServiceScaleExternalMetric

type ServiceScaleGpu struct {
	Count  int
	Vendor string
}

type ServiceScaleMetric struct {
	Aggregate  string
	Dimensions map[string]string
	Namespace  string
	Name       string
	Value      float64
}

type ServiceScaleMetrics []ServiceScaleMetric

type ServiceScaleTargets struct {
	Cpu      int
	Custom   ServiceScaleMetrics
	External ServiceScaleExternalMetrics
	Memory   int
	Requests int
}

type ServiceTermination struct {
	Grace int `yaml:"grace,omitempty"`
}

type ServiceTls struct {
	Redirect bool
}

// skipcq
func (s Service) BuildHash(key string) string {
	return fmt.Sprintf("%x", sha256.Sum224([]byte(fmt.Sprintf("key=%q build[path=%q, manifest=%q, args=%v] image=%q", key, s.Build.Path, s.Build.Manifest, s.Build.Args, s.Image))))
}

// skipcq
func (s Service) Domain() string {
	if len(s.Domains) < 1 {
		return ""
	}

	return s.Domains[0]
}

// skipcq
func (s Service) EnvironmentDefaults() map[string]string {
	defaults := map[string]string{}

	for _, e := range s.Environment {
		switch parts := strings.Split(e, "="); len(parts) {
		case 2:
			defaults[parts[0]] = parts[1]
		}
	}

	return defaults
}

// skipcq
func (s Service) EnvironmentKeys() string {
	kh := map[string]bool{}

	for _, e := range s.Environment {
		kh[strings.Split(e, "=")[0]] = true
	}

	keys := []string{}

	for k := range kh {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return strings.Join(keys, ",")
}

// skipcq
func (s Service) GetName() string {
	return s.Name
}

// skipcq
func (s Service) Autoscale() bool {
	if s.Agent.Enabled {
		return false
	}

	switch {
	case s.Scale.Count.Min == s.Scale.Count.Max:
		return false
	case s.Scale.Targets.Cpu > 0:
		return true
	case len(s.Scale.Targets.Custom) > 0:
		return true
	case s.Scale.Targets.Memory > 0:
		return true
	case s.Scale.Targets.Requests > 0:
		return true
	}

	return false
}

type ServiceResource struct {
	Name string
	Env  string
}

func (sr ServiceResource) GetConfigMapKey() string {
	parts := strings.Split(sr.Env, "_")
	key := parts[len(parts)-1]

	for _, en := range AdditionalEnvNames {
		if key == en {
			return key
		}
	}

	return DEFAULT_RESOURCE_ENV_NAME
}

// skipcq
func (s Service) AnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range s.Annotations {
		parts := strings.SplitN(a, "=", 2)
		annotations[parts[0]] = parts[1]
	}

	return annotations
}

// skipcq
func (s Service) ResourceMap() []ServiceResource {
	srs := []ServiceResource{}

	for _, r := range s.Resources {
		parts := strings.SplitN(r, ":", 2)

		switch len(parts) {
		case 1:
			envs := Resource{Name: parts[0]}.LoadEnv()
			for _, e := range envs {
				srs = append(srs, ServiceResource{Name: parts[0], Env: e})
			}
		case 2:
			srs = append(srs, ServiceResource{Name: parts[0], Env: strings.TrimSpace(parts[1])})
		}
	}

	return srs
}

// skipcq
func (s Service) ResourcesName() []string {
	srs := []string{}

	for _, r := range s.Resources {
		parts := strings.SplitN(r, ":", 2)
		srs = append(srs, parts[0])
	}

	return srs
}

func (ss Services) External() Services {
	return ss.Filter(func(s Service) bool {
		return !s.Internal && !s.InternalRouter
	})
}

func (ss Services) Filter(fn func(s Service) bool) Services {
	fss := Services{}

	// skipcq
	for _, s := range ss {
		if fn(s) {
			fss = append(fss, s)
		}
	}

	return fss
}

func (ss Services) InternalRouter() Services {
	return ss.Filter(func(s Service) bool {
		return s.InternalRouter
	})
}

func (ss Services) Routable() Services {
	return ss.Filter(func(s Service) bool {
		return s.Port.Port > 0
	})
}
