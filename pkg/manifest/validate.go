package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	ValidNameDescription = "must contain only lowercase alphanumeric and dashes"
)

var (
	nameValidator = regexp.MustCompile(`^[a-z]{1}[a-z0-9-]*$`)
)

func (m *Manifest) Validate() []error {
	errs := []error{}

	errs = append(errs, m.validateEnv()...)
	errs = append(errs, m.validateResources()...)
	errs = append(errs, m.validateServices()...)
	errs = append(errs, m.validateTimers()...)

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

	for _, s := range m.Services {
		if !nameValidator.MatchString(s.Name) {
			errs = append(errs, fmt.Errorf("service name %s invalid, %s", s.Name, ValidNameDescription))
		}

		for _, r := range s.ResourceMap() {
			if _, err := m.Resource(r.Name); err != nil {
				if strings.HasPrefix(err.Error(), "no such resource") {
					errs = append(errs, fmt.Errorf("service %s references a resource that does not exist: %s", s.Name, r.Name))
				}
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
	}

	return errs
}
