package manifest

type AppSettings struct {
	AwsLogs *ServiceLogsRetention `yaml:"awsLogs,omitempty"`
}

type ServiceLogsRetention struct {
	CwRetention      int  `yaml:"cwRetention,omitempty"`
	RetentionDisable bool `yaml:"disableRetention,omitempty"`
}
