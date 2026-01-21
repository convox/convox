package manifest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
)

// Helper functions for pointer values in tests
func ptrBool(b bool) *bool {
	return &b
}

func ptrInt64(i int64) *int64 {
	return &i
}

func TestManifestLoad(t *testing.T) {
	n := &manifest.Manifest{
		Balancers: manifest.Balancers{
			manifest.Balancer{
				Name: "main",
				Ports: manifest.BalancerPorts{
					manifest.BalancerPort{
						Protocol: "TCP",
						Source:   3000,
						Target:   1000,
					},
					manifest.BalancerPort{
						Protocol: "TCP",
						Source:   3001,
						Target:   5000,
					},
				},
				Service: "api",
			},
			manifest.Balancer{
				Name: "alternate",
				Ports: manifest.BalancerPorts{
					manifest.BalancerPort{
						Protocol: "TCP",
						Source:   4000,
						Target:   4001,
					},
				},
				Service:   "foo",
				Whitelist: []string{"127.0.0.0/24"},
			},
		},
		Environment: manifest.Environment{
			"DEVELOPMENT=true",
			"GLOBAL=true",
			"OTHERGLOBAL",
		},
		Params: manifest.Params{
			"Foo": "bar",
		},
		Resources: manifest.Resources{
			manifest.Resource{
				Name: "database",
				Type: "postgres",
				Options: map[string]string{
					"size": "db.t2.large",
				},
			},
		},
		Services: manifest.Services{
			manifest.Service{
				Name: "api",
				Annotations: manifest.Annotations{
					manifest.Annotation{
						Key:   "eks.amazonaws.com/role-arn",
						Value: "arn:aws:iam::123456789012:role/eksctl-irptest-addon-iamsa-default-my-serviceaccount-Role1-UCGG6NDYZ3UE",
					},
					manifest.Annotation{
						Key:   "test.other.com/annotation",
						Value: "myothervalue",
					},
					manifest.Annotation{
						Key:   "string.test.com/annotation",
						Value: "\"thishasquotes\"",
					},
				},
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile2",
					Path:     "api",
				},
				Command: "",
				Deployment: manifest.ServiceDeployment{
					Minimum: 25,
					Maximum: 110,
				},
				Domains: []string{"foo.example.org"},
				Drain:   30,
				Environment: []string{
					"DEFAULT=test",
					"DEVELOPMENT=false",
					"SECRET",
				},
				Health: manifest.ServiceHealth{
					Grace:    10,
					Path:     "/",
					Interval: 10,
					Timeout:  9,
				},
				Init: false,
				Port: manifest.ServicePortScheme{Port: 1000, Scheme: "http"},
				Ports: []manifest.ServicePortProtocol{
					manifest.ServicePortProtocol{Port: 2000, Protocol: "tcp"},
					manifest.ServicePortProtocol{Port: 3000, Protocol: "udp"},
				},
				Resources: []string{"database"},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 3, Max: 10},
					Cpu:    250,
					Memory: 512,
				},
				Sticky: false,
				Test:   "make  test",
				Termination: manifest.ServiceTermination{
					Grace: 45,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: false,
				},
				Whitelist: "127.0.0.0/24",
			},
			manifest.Service{
				Name:    "proxy",
				Command: "bash",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Domains: []string{"bar.example.org", "*.example.org"},
				Drain:   30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Path:     "/auth",
					Interval: 5,
					Timeout:  4,
				},
				Image: "ubuntu:16.04",
				Init:  true,
				Environment: []string{
					"SECRET",
				},
				Port: manifest.ServicePortScheme{Port: 2000, Scheme: "https"},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    512,
					Memory: 1024,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "foo",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Command: "foo",
				Deployment: manifest.ServiceDeployment{
					Minimum: 0,
					Maximum: 100,
				},
				Domains: []string{"baz.example.org", "qux.example.org"},
				Drain:   60,
				Health: manifest.ServiceHealth{
					Grace:    2,
					Interval: 5,
					Path:     "/",
					Timeout:  3,
				},
				Init: true,
				Port: manifest.ServicePortScheme{Port: 3000, Scheme: "https"},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 0, Max: 0},
					Cpu:    250,
					Memory: 512,
				},
				Singleton: true,
				Sticky:    true,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 3600,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "bar",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Command: "",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Interval: 5,
					Path:     "/",
					Timeout:  4,
				},
				Init: true,
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    250,
					Memory: 512,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "gpuscaler",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Command: "",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Interval: 5,
					Path:     "/",
					Timeout:  4,
				},
				Init: true,
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    768,
					Gpu:    manifest.ServiceScaleGpu{Count: 1, Vendor: "amd"},
					Memory: 2048,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "defaultgpuscaler",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Command: "",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Interval: 5,
					Path:     "/",
					Timeout:  4,
				},
				Init: true,
				Scale: manifest.ServiceScale{
					Count: manifest.ServiceScaleCount{Min: 1, Max: 1},
					Gpu:   manifest.ServiceScaleGpu{Count: 2, Vendor: "nvidia"},
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "scaler",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Command: "",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Interval: 5,
					Path:     "/",
					Timeout:  4,
				},
				Init: true,
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 5},
					Cpu:    250,
					Memory: 512,
					Targets: manifest.ServiceScaleTargets{
						Cpu:      50,
						Memory:   75,
						Requests: 200,
						Custom: manifest.ServiceScaleMetrics{
							{
								Aggregate:  "max",
								Dimensions: map[string]string{"QueueName": "testqueue"},
								Namespace:  "AWS/SQS",
								Name:       "ApproximateNumberOfMessagesVisible",
								Value:      float64(200),
							},
						},
					},
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name:    "inherit",
				Command: "inherit",
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Domains: []string{"bar.example.org", "*.example.org"},
				Drain:   30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Path:     "/auth",
					Interval: 5,
					Timeout:  4,
				},
				Image: "ubuntu:16.04",
				Init:  true,
				Environment: []string{
					"SECRET",
				},
				Port: manifest.ServicePortScheme{Port: 2000, Scheme: "https"},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    512,
					Memory: 1024,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "agent",
				Agent: manifest.ServiceAgent{
					Enabled: true,
				},
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Deployment: manifest.ServiceDeployment{
					Minimum: 0,
					Maximum: 100,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Path:     "/",
					Interval: 5,
					Timeout:  4,
				},
				Init: true,
				Ports: []manifest.ServicePortProtocol{
					{Port: 5000, Protocol: "udp"},
					{Port: 5001, Protocol: "tcp"},
					{Port: 5002, Protocol: "tcp"},
				},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    250,
					Memory: 512,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
			manifest.Service{
				Name: "secured",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Health: manifest.ServiceHealth{
					Grace:    5,
					Path:     "/",
					Interval: 5,
					Timeout:  4,
				},
				Init: true,
				Port: manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    250,
					Memory: 512,
				},
				SecurityContext: manifest.ServiceSecurityContext{
					RunAsNonRoot:             ptrBool(true),
					RunAsUser:                ptrInt64(1000),
					RunAsGroup:               ptrInt64(1000),
					ReadOnlyRootFilesystem:   ptrBool(true),
					AllowPrivilegeEscalation: ptrBool(false),
					Capabilities: &manifest.ServiceSecurityContextCapabilities{
						Drop: []string{"ALL"},
						Add:  []string{"NET_BIND_SERVICE"},
					},
					SeccompProfile: "RuntimeDefault",
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
		},
		Timers: manifest.Timers{
			manifest.Timer{
				Command:  "bin/alpha",
				Name:     "alpha",
				Schedule: "*/1 * * * *",
				Service:  "api",
			},
			manifest.Timer{
				Command:  "bin/bravo",
				Name:     "bravo",
				Schedule: "*/1 * * * *",
				Service:  "api",
			},
			manifest.Timer{
				Command:  "bin/charlie",
				Name:     "charlie",
				Schedule: "*/1 * * * *",
				Service:  "api",
			},
		},
	}

	attrs := []string{
		"balancers",
		"balancers.alternate",
		"balancers.alternate.ports",
		"balancers.alternate.ports.4000",
		"balancers.alternate.service",
		"balancers.alternate.whitelist",
		"balancers.main",
		"balancers.main.ports",
		"balancers.main.ports.3000",
		"balancers.main.ports.3000.port",
		"balancers.main.ports.3000.protocol",
		"balancers.main.ports.3001",
		"balancers.main.service",
		"environment",
		"params",
		"params.Foo",
		"resources",
		"resources.database",
		"resources.database.options",
		"resources.database.options.size",
		"resources.database.type",
		"services",
		"services.agent",
		"services.agent.agent",
		"services.agent.ports",
		"services.api",
		"services.api.annotations",
		"services.api.build",
		"services.api.build.manifest",
		"services.api.build.path",
		"services.api.deployment",
		"services.api.deployment.maximum",
		"services.api.deployment.minimum",
		"services.api.domain",
		"services.api.environment",
		"services.api.health",
		"services.api.health.interval",
		"services.api.init",
		"services.api.port",
		"services.api.ports",
		"services.api.resources",
		"services.api.scale",
		"services.api.test",
		"services.api.termination",
		"services.api.termination.grace",
		"services.api.tls",
		"services.api.tls.redirect",
		"services.api.whitelist",
		"services.bar",
		"services.defaultgpuscaler",
		"services.defaultgpuscaler.scale",
		"services.defaultgpuscaler.scale.gpu",
		"services.foo",
		"services.foo.command",
		"services.foo.domain",
		"services.foo.drain",
		"services.foo.health",
		"services.foo.health.grace",
		"services.foo.health.timeout",
		"services.foo.port",
		"services.foo.port.port",
		"services.foo.port.scheme",
		"services.foo.scale",
		"services.foo.singleton",
		"services.foo.sticky",
		"services.foo.timeout",
		"services.gpuscaler",
		"services.gpuscaler.scale",
		"services.gpuscaler.scale.cpu",
		"services.gpuscaler.scale.gpu",
		"services.gpuscaler.scale.gpu.count",
		"services.gpuscaler.scale.gpu.vendor",
		"services.gpuscaler.scale.memory",
		"services.inherit",
		"services.inherit.command",
		"services.inherit.domain",
		"services.inherit.environment",
		"services.inherit.health",
		"services.inherit.image",
		"services.inherit.port",
		"services.inherit.scale",
		"services.inherit.scale.cpu",
		"services.inherit.scale.memory",
		"services.proxy",
		"services.proxy.command",
		"services.proxy.domain",
		"services.proxy.environment",
		"services.proxy.health",
		"services.proxy.image",
		"services.proxy.port",
		"services.proxy.scale",
		"services.proxy.scale.cpu",
		"services.proxy.scale.memory",
		"services.scaler",
		"services.scaler.scale",
		"services.scaler.scale.count",
		"services.scaler.scale.targets",
		"services.scaler.scale.targets.cpu",
		"services.scaler.scale.targets.custom",
		"services.scaler.scale.targets.custom.AWS/SQS/ApproximateNumberOfMessagesVisible",
		"services.scaler.scale.targets.custom.AWS/SQS/ApproximateNumberOfMessagesVisible.aggregate",
		"services.scaler.scale.targets.custom.AWS/SQS/ApproximateNumberOfMessagesVisible.dimensions",
		"services.scaler.scale.targets.custom.AWS/SQS/ApproximateNumberOfMessagesVisible.dimensions.QueueName",
		"services.scaler.scale.targets.custom.AWS/SQS/ApproximateNumberOfMessagesVisible.value",
		"services.scaler.scale.targets.memory",
		"services.scaler.scale.targets.requests",
		"services.secured",
		"services.secured.build",
		"services.secured.port",
		"services.secured.securityContext",
		"services.secured.securityContext.allowPrivilegeEscalation",
		"services.secured.securityContext.capabilities",
		"services.secured.securityContext.capabilities.add",
		"services.secured.securityContext.capabilities.drop",
		"services.secured.securityContext.readOnlyRootFilesystem",
		"services.secured.securityContext.runAsGroup",
		"services.secured.securityContext.runAsNonRoot",
		"services.secured.securityContext.runAsUser",
		"services.secured.securityContext.seccompProfile",
		"timers",
		"timers.alpha",
		"timers.alpha.command",
		"timers.alpha.schedule",
		"timers.alpha.service",
		"timers.bravo",
		"timers.bravo.command",
		"timers.bravo.schedule",
		"timers.bravo.service",
		"timers.charlie",
		"timers.charlie.command",
		"timers.charlie.schedule",
		"timers.charlie.service",
	}

	env := map[string]string{"FOO": "bar", "SECRET": "shh", "OTHERGLOBAL": "test"}

	n.SetAttributes(attrs)
	n.SetEnv(env)

	// env processing that normally happens as part of load
	require.NoError(t, n.CombineEnv())

	m, err := testdataManifest("full", env)
	require.NoError(t, err)
	require.Equal(t, n, m)

	senv, err := m.ServiceEnvironment("api")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"DEFAULT": "test", "DEVELOPMENT": "false", "GLOBAL": "true", "OTHERGLOBAL": "test", "SECRET": "shh"}, senv)

	s1, err := m.Service("api")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"DEFAULT": "test", "DEVELOPMENT": "false", "GLOBAL": "true"}, s1.EnvironmentDefaults())
	require.Equal(t, "DEFAULT,DEVELOPMENT,GLOBAL,OTHERGLOBAL,SECRET", s1.EnvironmentKeys())

	s2, err := m.Service("proxy")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"DEVELOPMENT": "true", "GLOBAL": "true"}, s2.EnvironmentDefaults())
	require.Equal(t, "DEVELOPMENT,GLOBAL,OTHERGLOBAL,SECRET", s2.EnvironmentKeys())
}

func TestManifestLoadSimple(t *testing.T) {
	_, err := testdataManifest("simple", map[string]string{})
	require.EqualError(t, err, "required env: REQUIRED")

	n := &manifest.Manifest{
		Services: manifest.Services{
			manifest.Service{
				Name: "web",
				Build: manifest.ServiceBuild{
					Manifest: "Dockerfile",
					Path:     ".",
				},
				Deployment: manifest.ServiceDeployment{
					Minimum: 50,
					Maximum: 200,
				},
				Drain: 30,
				Environment: manifest.Environment{
					"REQUIRED",
					"DEFAULT=true",
				},
				Health: manifest.ServiceHealth{
					Grace:    5,
					Interval: 5,
					Path:     "/",
					Timeout:  4,
				},
				Init: true,
				Scale: manifest.ServiceScale{
					Count:  manifest.ServiceScaleCount{Min: 1, Max: 1},
					Cpu:    250,
					Memory: 512,
				},
				Sticky: false,
				Termination: manifest.ServiceTermination{
					Grace: 30,
				},
				Timeout: 60,
				Tls: manifest.ServiceTls{
					Redirect: true,
				},
			},
		},
	}

	n.SetAttributes([]string{"services", "services.web", "services.web.build", "services.web.environment"})
	n.SetEnv(map[string]string{"REQUIRED": "test"})

	// env processing that normally happens as part of load
	require.NoError(t, n.CombineEnv())

	m, err := testdataManifest("simple", map[string]string{"REQUIRED": "test"})
	require.NoError(t, err)
	require.Equal(t, n, m)
}

func TestManifestLoadClobberEnv(t *testing.T) {
	env := map[string]string{"FOO": "bar", "REQUIRED": "false"}

	_, err := testdataManifest("simple", env)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"FOO": "bar", "REQUIRED": "false"}, env)
}

func TestManifestLoadInvalid(t *testing.T) {
	m, err := testdataManifest("full", map[string]string{})
	require.Nil(t, m)
	require.Error(t, err, "required env: OTHERGLOBAL, SECRET")

	m, err = testdataManifest("invalid.1", map[string]string{})
	require.Nil(t, m)
	require.Error(t, err, "yaml: line 2: did not find expected comment or line break")

	m, err = testdataManifest("invalid.2", map[string]string{})
	require.NotNil(t, m)
	require.NoError(t, err)
	require.Len(t, m.Services, 0)
}

func TestManifestEnvManipulation(t *testing.T) {
	m, err := testdataManifest("env", map[string]string{})
	require.NotNil(t, m)
	require.NoError(t, err)

	require.Equal(t, "train-intent", m.Services[0].EnvironmentDefaults()["QUEUE_NAME"])
	require.Equal(t, "delete-intent", m.Services[1].EnvironmentDefaults()["QUEUE_NAME"])
}

func testdataManifest(name string, env map[string]string) (*manifest.Manifest, error) {
	data, err := common.Testdata(name)
	if err != nil {
		return nil, err
	}

	m, err := manifest.Load(data, env)
	if err != nil {
		return nil, err
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return m, nil
}

func TestManifestValidate(t *testing.T) {
	m, err := testdataManifest("validate", map[string]string{})
	require.Nil(t, m)

	errors := []string{
		"balancer alpha has no ports",
		"balancer alpha has blank service",
		"balancer alpha whitelist 1.1.1.1 is not a valid cidr range",
		"balancer bravo refers to unknown service nosuch",
		"resource name 1resource invalid, must contain only lowercase alphanumeric and dashes",
		"service deployment-invalid-low deployment minimum can not be less than 0",
		"service deployment-invalid-low deployment maximum can not be less than 100",
		"service deployment-invalid-high deployment minimum can not be greater than 100",
		"service deployment-invalid-high deployment maximum can not be greater than 200",
		"service internal-router-invalid can not have both internal and internalRouter set as true",
		"service name serviceF invalid, must contain only lowercase alphanumeric and dashes",
		"service serviceF references a resource that does not exist: foo",
		"timer name timer_1 invalid, must contain only lowercase alphanumeric and dashes",
		"timer timer_1 references a service that does not exist: someservice",
	}

	require.EqualError(t, err, fmt.Sprintf("validation errors:\n%s", strings.Join(errors, "\n")))
}
