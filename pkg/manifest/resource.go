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

func (r Resource) LoadEnv() []string {
	return []string{
		r.mountEnv("URL"),
		r.mountEnv("USER"),
		r.mountEnv("PASS"),
		r.mountEnv("HOST"),
		r.mountEnv("PORT"),
		r.mountEnv("NAME"),
	}
}

func (r Resource) GetName() string {
	return r.Name
}

func (r Resource) mountEnv(envVar string) string {
	return fmt.Sprintf("%s_%s", strings.ReplaceAll(strings.ToUpper(r.Name), "-", "_"), envVar)
}
