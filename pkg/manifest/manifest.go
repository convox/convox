package manifest

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/options"
	yaml "gopkg.in/yaml.v2"
)

var (
	DefaultCpu        = 250
	DefaultMem        = 512
	ReservedLabelKeys = map[string]bool{
		"system":  true,
		"rack":    true,
		"app":     true,
		"name":    true,
		"service": true,
		"release": true,
		"type":    true,
	}
)

type Manifest struct {
	AppSettings AppSettings `yaml:"appSettings,omitempty"`
	Balancers   Balancers   `yaml:"balancers,omitempty"`
	Configs     AppConfigs  `yaml:"configs,omitempty"`
	Environment Environment `yaml:"environment,omitempty"`
	Labels      Labels      `yaml:"labels,omitempty"`
	Params      Params      `yaml:"params,omitempty"`
	Resources   Resources   `yaml:"resources,omitempty"`
	Services    Services    `yaml:"services,omitempty"`
	Timers      Timers      `yaml:"timers,omitempty"`

	attributes map[string]bool
	env        map[string]string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Load(data []byte, env map[string]string) (*Manifest, error) {
	var m Manifest

	p, err := interpolate(data, env)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(p, &m); err != nil {
		return nil, err
	}

	m.attributes, err = yamlAttributes(p)
	if err != nil {
		return nil, err
	}

	m.env = map[string]string{}

	for k, v := range env {
		m.env[k] = v
	}

	if err := m.ApplyCompatibility(); err != nil {
		return nil, err
	}

	if err := m.ApplyDefaults(); err != nil {
		return nil, err
	}

	if err := m.CombineEnv(); err != nil {
		return nil, err
	}

	if err := m.CombineLabels(); err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *Manifest) Agents() []string {
	a := []string{}

	for _, s := range m.Services {
		if s.Agent.Enabled {
			a = append(a, s.Name)
		}
	}

	return a
}

func (m *Manifest) ApplyCompatibility() error {
	for i := range m.Timers {
		parts := strings.Fields(m.Timers[i].Schedule)

		// v3 uses only 5 fields for schedules
		if len(parts) > 5 {
			parts = parts[0:5]
		}

		// replace ? with * from v2 aws syntax
		for j := range parts {
			if strings.TrimSpace(parts[j]) == "?" {
				parts[j] = "*"
			}
		}

		m.Timers[i].Schedule = strings.Join(parts, " ")
	}

	return nil
}

func (m *Manifest) ApplyDefaults() error {

	for i, s := range m.Services {
		if s.Build.Path == "" && s.Image == "" {
			m.Services[i].Build.Path = "."
		}

		if m.Services[i].Build.Path != "" && s.Build.Manifest == "" {
			m.Services[i].Build.Manifest = "Dockerfile"
		}

		if !m.AttributeExists(fmt.Sprintf("services.%s.deployment.maximum", s.Name)) {
			if s.Agent.Enabled || s.Singleton {
				m.Services[i].Deployment.Maximum = 100
			} else {
				m.Services[i].Deployment.Maximum = 200
			}
		}

		if !m.AttributeExists(fmt.Sprintf("services.%s.deployment.minimum", s.Name)) {
			if s.Agent.Enabled || s.Singleton {
				m.Services[i].Deployment.Minimum = 0
			} else {
				m.Services[i].Deployment.Minimum = 50
			}
		}

		if s.Drain == 0 {
			m.Services[i].Drain = 30
		}

		if s.Health.Path == "" {
			m.Services[i].Health.Path = "/"
		}

		if s.Health.Interval == 0 {
			m.Services[i].Health.Interval = 5
		}

		if s.Health.Grace == 0 {
			m.Services[i].Health.Grace = m.Services[i].Health.Interval
		}

		if s.Health.Timeout == 0 {
			m.Services[i].Health.Timeout = m.Services[i].Health.Interval - 1
		}

		s.Liveness.Path = strings.TrimSpace(s.Liveness.Path)
		if s.Liveness.Path != "" {
			if s.Liveness.Grace == 0 {
				m.Services[i].Liveness.Grace = 10
			}
			if s.Liveness.Interval == 0 {
				m.Services[i].Liveness.Interval = 5
			}
			if s.Liveness.Timeout == 0 {
				m.Services[i].Liveness.Timeout = 5
			}
			if s.Liveness.SuccessThreshold == 0 {
				m.Services[i].Liveness.SuccessThreshold = 1
			}
			if s.Liveness.FailureThreshold == 0 {
				m.Services[i].Liveness.FailureThreshold = 3
			}
		}

		if !m.AttributeExists(fmt.Sprintf("services.%s.init", s.Name)) {
			m.Services[i].Init = true
		}

		if s.Port.Port > 0 && s.Port.Scheme == "" {
			m.Services[i].Port.Scheme = "http"
		}

		sp := fmt.Sprintf("services.%s.scale", s.Name)

		// if no scale attributes set
		if len(m.AttributesByPrefix(sp)) == 0 {
			m.Services[i].Scale.Count = ServiceScaleCount{Min: 1, Max: 1}
		}

		// if no explicit count attribute set yet has multiple scale attributes other than count
		if !m.AttributeExists(fmt.Sprintf("%s.count", sp)) && len(m.AttributesByPrefix(sp)) > 1 {
			m.Services[i].Scale.Count = ServiceScaleCount{Min: 1, Max: 1}
		}

		if m.Services[i].Scale.Gpu.Count == 0 {
			if m.Services[i].Scale.Cpu == 0 {
				m.Services[i].Scale.Cpu = DefaultCpu
				if m.Services[i].Scale.Limit.Cpu > 0 {
					m.Services[i].Scale.Cpu = m.Services[i].Scale.Limit.Cpu
				}
			}

			if m.Services[i].Scale.Memory == 0 {
				m.Services[i].Scale.Memory = DefaultMem
				if m.Services[i].Scale.Limit.Memory > 0 {
					m.Services[i].Scale.Memory = m.Services[i].Scale.Limit.Memory
				}
			}
		}

		if options.GetFeatureGates()[options.FeatureGateAppLimitRequired] {
			m.Services[i].Scale.Cpu = options.CoalesceInt(m.Services[i].Scale.Cpu, options.CoalesceInt(m.Services[i].Scale.Limit.Cpu, DefaultCpu))
			m.Services[i].Scale.Memory = options.CoalesceInt(m.Services[i].Scale.Memory, options.CoalesceInt(m.Services[i].Scale.Limit.Memory, DefaultMem))

			m.Services[i].Scale.Limit.Cpu = options.CoalesceInt(m.Services[i].Scale.Limit.Cpu, m.Services[i].Scale.Cpu)
			m.Services[i].Scale.Limit.Memory = options.CoalesceInt(m.Services[i].Scale.Limit.Memory, m.Services[i].Scale.Memory)
		}

		if m.Services[i].Scale.Limit.Cpu > 0 && m.Services[i].Scale.Limit.Cpu < m.Services[i].Scale.Cpu {
			return fmt.Errorf("cpu limit can not be less cpu request")
		}
		if m.Services[i].Scale.Limit.Memory > 0 && m.Services[i].Scale.Limit.Memory < m.Services[i].Scale.Memory {
			return fmt.Errorf("memory limit can not be less memory request")
		}

		if m.Services[i].Scale.Gpu.Count > 0 && m.Services[i].Scale.Gpu.Vendor == "" {
			m.Services[i].Scale.Gpu.Vendor = "nvidia"
		}

		if !m.AttributeExists(fmt.Sprintf("services.%s.termination.grace", s.Name)) {
			m.Services[i].Termination.Grace = 30
		}

		if s.Timeout == 0 {
			m.Services[i].Timeout = 60
		}

		if !m.AttributeExists(fmt.Sprintf("services.%s.tls.redirect", s.Name)) {
			m.Services[i].Tls.Redirect = true
		}
	}

	for i := range m.Resources {
		if m.Resources[i].Options == nil {
			m.Resources[i].Options = map[string]string{}
		}
		if options.GetFeatureGates()[options.FeatureGateAppLimitRequired] && m.Resources[i].IsContainerizedResource() {
			// set default options
			if m.Resources[i].Options["cpu"] == "" {
				m.Resources[i].Options["cpu"] = "250" // 0.25 vCPU
			} else {
				cpu, _ := strconv.Atoi(m.Resources[i].Options["cpu"])
				if cpu < 100 || cpu > 10000 {
					return fmt.Errorf("resource cpu must be between 100 and 10000 millicpu")
				}
			}
			if m.Resources[i].Options["mem"] == "" {
				m.Resources[i].Options["mem"] = "512" // 512 MB
			} else {
				mem, _ := strconv.Atoi(m.Resources[i].Options["mem"])
				if mem < 256 || mem > 10240 {
					return fmt.Errorf("resource mem must be between 256 and 10240 megabyte")
				}
			}
		}
	}

	return nil
}

func (m *Manifest) Attributes() []string {
	attrs := []string{}

	for k := range m.attributes {
		attrs = append(attrs, k)
	}

	sort.Strings(attrs)

	return attrs
}

func (m *Manifest) AttributesByPrefix(prefix string) []string {
	attrs := []string{}

	for _, a := range m.Attributes() {
		if strings.HasPrefix(a, prefix) {
			attrs = append(attrs, a)
		}
	}

	return attrs
}

func (m *Manifest) AttributeExists(name string) bool {
	return m.attributes[name]
}

func (m *Manifest) Env() map[string]string {
	return m.env
}

// CombineEnv calculates the final environment of each service
// and filters m.env to the union of all service env vars
// defined in the manifest
func (m *Manifest) CombineEnv() error {
	for i, s := range m.Services {
		me := make([]string, len(m.Environment))
		copy(me, m.Environment)
		m.Services[i].Environment = append(me, s.Environment...)
	}

	keys := map[string]bool{}

	for _, s := range m.Services {
		env, err := m.ServiceEnvironment(s.Name)
		if err != nil {
			return err
		}

		for k := range env {
			keys[k] = true
		}
	}

	for k := range m.env {
		if !keys[k] {
			delete(m.env, k)
		}
	}

	return nil
}

func (m *Manifest) CombineLabels() error {
	for k := range m.Labels {
		if _, has := ReservedLabelKeys[k]; has {
			return fmt.Errorf("reserved label key '%s' is not allowed", k)
		}
	}

	for i := range m.Services {
		used := map[string]bool{}
		for k := range m.Services[i].Labels {
			used[k] = true
			if _, has := ReservedLabelKeys[k]; has {
				return fmt.Errorf("reserved label key '%s' is not allowed", k)
			}
		}

		for k, v := range m.Labels {
			if !used[k] {
				if m.Services[i].Labels == nil {
					m.Services[i].Labels = Labels{}
				}
				m.Services[i].Labels[k] = v
			}
		}
	}

	return nil
}

func (m *Manifest) Resource(name string) (*Resource, error) {
	for _, r := range m.Resources {
		if r.Name == name {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no such resource: %s", name)
}

func (m *Manifest) Service(name string) (*Service, error) {
	for _, s := range m.Services {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("no such service: %s", name)
}

func (m *Manifest) ServiceEnvironment(service string) (map[string]string, error) {
	s, err := m.Service(service)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}

	missing := []string{}

	for _, e := range s.Environment {
		parts := strings.SplitN(e, "=", 2)

		switch len(parts) {
		case 1:
			if parts[0] == "*" {
				for k, v := range m.env {
					env[k] = v
				}
			} else {
				v, ok := m.env[parts[0]]
				if !ok {
					missing = append(missing, parts[0])
				}
				env[parts[0]] = v
			}
		case 2:
			v, ok := m.env[parts[0]]
			if ok {
				env[parts[0]] = v
			} else {
				env[parts[0]] = parts[1]
			}
		default:
			return nil, fmt.Errorf("invalid environment declaration: %s", e)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)

		return nil, fmt.Errorf("required env: %s", strings.Join(missing, ", "))
	}

	return env, nil
}

// used only for tests
func (m *Manifest) SetAttributes(attrs []string) {
	m.attributes = map[string]bool{}

	for _, a := range attrs {
		m.attributes[a] = true
	}
}

// used only for tests
func (m *Manifest) SetEnv(env map[string]string) {
	m.env = env
}

func (m *Manifest) Validate() error {
	if errs := m.validate(); len(errs) > 0 {
		messages := []string{}

		for _, err := range errs {
			messages = append(messages, err.Error())
		}

		return fmt.Errorf("validation errors:\n%s", strings.Join(messages, "\n"))
	}

	return nil
}
