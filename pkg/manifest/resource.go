package manifest

import (
	"fmt"
	"strings"
)

const (
	DEFAULT_RESOURCE_ENV_NAME = "URL"
)

var (
	AdditionalEnvNames = []string{
		DEFAULT_RESOURCE_ENV_NAME,
		"USER",
		"PASS",
		"HOST",
		"PORT",
		"NAME",
	}
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
	envs := []string{}
	for _, e := range AdditionalEnvNames {
		envs = append(envs, r.mountEnv(e))
	}
	return envs
}

func (r Resource) GetName() string {
	return r.Name
}

func (r Resource) mountEnv(envVar string) string {
	return fmt.Sprintf("%s_%s", strings.ReplaceAll(strings.ToUpper(r.Name), "-", "_"), envVar)
}

func (r Resource) IsRds() bool {
	return strings.HasPrefix(r.Type, "rds-")
}
