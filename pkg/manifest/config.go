package manifest

import "fmt"

type AppConfigs []AppConfig

type AppConfig struct {
	Id    string  `yaml:"id"`
	Name  string  `yaml:"name,omitempty"` // the actual name to create secret config
	Value *string `yaml:"value"`
}

func (a *AppConfig) Validate() error {
	if a.Id == "" {
		return fmt.Errorf("configs[*].id is required")
	}
	return nil
}

type ConfigMounts []ConfigMount

type ConfigMount struct {
	Id       string `yaml:"id"`
	Dir      string `yaml:"dir"`
	Filename string `json:"filename"`
}

func (c *ConfigMount) Validate() error {
	if c.Id == "" {
		return fmt.Errorf("configMounts[*].id is required")
	}
	if c.Dir == "" {
		return fmt.Errorf("configMounts[*].dir is required")
	}
	if c.Filename == "" {
		return fmt.Errorf("configMounts[*].filename is required")
	}
	return nil
}
