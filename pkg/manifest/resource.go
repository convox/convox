package manifest

import (
	"fmt"
	"strings"
)

type Resource struct {
	Name    string            `yaml:"-"`
	Type    string            `yaml:"type"`
	Options map[string]string `yaml:"options"`
}

type Resources []Resource

func (r Resource) DefaultEnv() string {
	return fmt.Sprintf("%s_URL", strings.Replace(strings.ToUpper(r.Name), "-", "_", -1))
}

func (r Resource) GetName() string {
	return r.Name
}
