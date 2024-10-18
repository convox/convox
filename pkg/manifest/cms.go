package manifest

type CMSInstance struct {
	Name string `yaml:"-"`

	Type          string        `yaml:"type,omitempty"`
	Image         string        `yaml:"image,omitempty"`
	DrupalOptions DrupalOptions `yaml:"drupalOptions,omitempty"`
	Resources     []string      `yaml:"resources,omitempty"`
	Scale         CmsScaleCount  `yaml:"scale,omitempty"`
}

type CMSInstances []CMSInstance

type DrupalOptions struct {
	Domain        string   `yaml:"domain,omitempty"`
	Database      Database `yaml:"database,omitempty"`
	SiteName      string   `yaml:"siteName,omitempty"`
	Email         string   `yaml:"email,omitempty"`
	AdminUsername string   `yaml:"adminUsername,omitempty"`
	AdminPassword string   `yaml:"adminPassword,omitempty"`
	Country       string   `yaml:"country,omitempty"`
	Modules       []string `yaml:"modules,omitempty"`
	Themes        []string `yaml:"themes,omitempty"`
}

type Database struct {
	Type        string `yaml:"type,omitempty"`
	TablePrefix string `yaml:"tablePrefix,omitempty"`
	Url         string `yaml:"url,omitempty"`
	Host        string `yaml:"host,omitempty"`
	Port        string `yaml:"port,omitempty"`
	Username    string `yaml:"username,omitempty"`
	Password    string `yaml:"password,omitempty"`
	DbName      string `yaml:"dbName,omitempty"`
}

type CmsScaleCount struct {
	Min int `yaml:"min,omitempty"`
	Max int `yaml:"max,omitempty"`
}
