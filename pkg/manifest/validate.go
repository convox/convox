package manifest

import (
	"fmt"
	"strings"
)

func (m *Manifest) Validate() error {
	if err := m.validateEnv(); err != nil {
		return err
	}

	if err := m.validateResources(); err != nil {
		return err
	}

	return nil
}

func (m *Manifest) validateEnv() error {
	for _, s := range m.Services {
		if _, err := m.ServiceEnvironment(s.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manifest) validateResources() error {
	for _, r := range m.Resources {
		if strings.TrimSpace(r.Type) == "" {
			return fmt.Errorf("resource %q has blank type", r.Name)
		}
	}

	return nil
}
