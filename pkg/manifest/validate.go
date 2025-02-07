package manifest

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

const (
	ValidNameDescription = "must contain only lowercase alphanumeric and dashes"
)

var (
	nameValidator = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
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
		if !nameValidator.MatchString(r.Name) {
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
		if !nameValidator.MatchString(s.Name) {
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

		for i := range s.ConfigMounts {
			cm := &s.ConfigMounts[i]
			if err := cm.Validate(); err != nil {
				errs = append(errs, err)
			}

			if _, has := configMap[cm.Id]; !has {
				errs = append(errs, fmt.Errorf("config id: '%s' not found", cm.Id))
			}
		}

	}
	return errs
}

func (m *Manifest) validateTimers() []error {
	errs := []error{}

	for _, t := range m.Timers {
		if !nameValidator.MatchString(t.Name) {
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
