package manifest

type AppSettings struct {
	AwsLogs *ServiceLogsRetention `yaml:"aws-logs,omitempty"`
}

type ServiceLogsRetention struct {
	CwRetention      int  `yaml:"cw-retention,omitempty"`
	RetentionDisable bool `yaml:"disableRetention,omitempty"`
}
