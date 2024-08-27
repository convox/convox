package manifest

import (
	"fmt"
	"regexp"
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

	RdsNameValidationRegex = regexp.MustCompile("^[a-z]([a-z0-9-]*[a-z0-9])?$")
)

type Resource struct {
	Name    string            `yaml:"-"`
	Type    string            `yaml:"type"`
	Image   string            `yaml:"image"`
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

func (r Resource) IsCustomManagedResource() bool {
	return r.IsRds() || r.IsElastiCache()
}

func (r Resource) IsRds() bool {
	return strings.HasPrefix(r.Type, "rds-")
}

func (r Resource) RdsNameValidate() error {
	if !RdsNameValidationRegex.MatchString(r.Name) {
		return fmt.Errorf("invalid rds resource name: only alphanumeric letter and hypen allowed")
	}
	if len(r.Name) > 20 {
		return fmt.Errorf("rds resource name must not excced 20 char limit")
	}
	return nil
}

func (r Resource) IsElastiCache() bool {
	return strings.HasPrefix(r.Type, "elasticache-")
}

func (r Resource) ElastiCacheNameValidate() error {
	if !RdsNameValidationRegex.MatchString(r.Name) {
		return fmt.Errorf("invalid elasticache resource name: only alphanumeric letter and hypen allowed")
	}
	if len(r.Name) > 20 {
		return fmt.Errorf("elasticache resource name must not excced 40 char limit")
	}
	return nil
}
