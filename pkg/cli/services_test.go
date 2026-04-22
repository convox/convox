package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestServices(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// i.On("ClientType").Return("standard")
		i.On("ServiceList", "app1").Return(structs.Services{*fxService(), *fxService()}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS",
			"service1  domain  1:2 1:2",
			"service1  domain  1:2 1:2",
		})
	})
}

func TestServicesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// i.On("ClientType").Return("standard")
		i.On("ServiceList", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestServicesRestart(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceRestart", "app1", "service1").Return(nil)

		res, err := testExecute(e, "services restart service1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Restarting service1... OK"})
	})
}

func TestServicesRestartError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceRestart", "app1", "service1").Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "services restart service1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Restarting service1... "})
	})
}

func TestServicesWithNLB(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{*fxServiceNLB(), *fxService()}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443 9443:8080(internal)",
			"service1  domain  1:2 1:2  ",
		})
	})
}

func TestServicesAllWithNLB(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{*fxServiceNLB(), *fxServiceNLB()}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443 9443:8080(internal)",
			"service1  domain  1:2 1:2  8443:8443 9443:8080(internal)",
		})
	})
}

func TestServicesWithNLBTLS(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{*fxServiceNLBTLS(), *fxService()}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8080/tls 9443:8080(internal)",
			"service1  domain  1:2 1:2  ",
		})
	})
}

func TestServicesMixedSchemesSingleService(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public"},
			{Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "internal"},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443 9443:8080(internal)",
		})
	})
}

func TestServicesMixedSchemesOrderPreserved(t *testing.T) {
	t.Run("public first preserves order", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public"},
				{Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "internal"},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  8443:8443 9443:8080(internal)",
			})
		})
	})

	t.Run("internal first preserves order", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "internal"},
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public"},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  9443:8080(internal) 8443:8443",
			})
		})
	})
}

func TestServicesEmptyNlbSlice(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS",
			"service1  domain  1:2 1:2",
		})
	})
}

func TestServicesWorkerOnlyNLB(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		worker := &structs.Service{
			Name: "worker",
			Nlb:  []structs.ServiceNlbPort{{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public"}},
		}
		web := &structs.Service{
			Name:   "web",
			Domain: "domain",
			Ports:  []structs.ServicePort{{Balancer: 1, Container: 2}},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*worker, *web}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE  DOMAIN  PORTS  NLB PORTS",
			"worker                  8443:8443",
			"web      domain  1:2    ",
		})
	})
}

// TestServicesNLBAllDefaults — NLB port with no hardening fields renders plain.
func TestServicesNLBAllDefaults(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public"},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443",
		})
	})
}

func TestServicesNLBCrossZone(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", CrossZone: options.Bool(true)},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  8443:8443[cz=true]",
			})
		})
	})

	t.Run("false", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", CrossZone: options.Bool(false)},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  8443:8443[cz=false]",
			})
		})
	})
}

func TestServicesNLBAllowCIDR(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{
				Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public",
				AllowCIDR: []string{"10.0.0.0/24", "10.1.0.0/24"},
			},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443[allow=2]",
		})
	})
}

func TestServicesNLBPreserveClientIP(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", PreserveClientIP: options.Bool(true)},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  8443:8443[pcip=true]",
			})
		})
	})

	t.Run("false", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			s := fxService()
			s.Nlb = []structs.ServiceNlbPort{
				{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", PreserveClientIP: options.Bool(false)},
			}
			i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE   DOMAIN  PORTS    NLB PORTS",
				"service1  domain  1:2 1:2  8443:8443[pcip=false]",
			})
		})
	})
}

func TestServicesNLBAllThree(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{*fxServiceNLBHardening()}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443[cz=true allow=2 pcip=false]",
		})
	})
}

// TestServicesNLBTLSWithHardening — combine /tls marker + brackets on same cell.
func TestServicesNLBTLSWithHardening(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{
				Port: 8443, Protocol: "tls", ContainerPort: 8080, Scheme: "public",
				Certificate: "arn:aws:acm:us-east-1:123456789012:certificate/abc",
				CrossZone:   options.Bool(true),
				AllowCIDR:   []string{"10.0.0.0/24", "10.1.0.0/24"},
			},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8080/tls[cz=true allow=2]",
		})
	})
}

// TestServicesNLBInternalWithHardening — combine (internal) + brackets.
func TestServicesNLBInternalWithHardening(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{
				Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "internal",
				CrossZone: options.Bool(true),
			},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  9443:8080(internal)[cz=true]",
		})
	})
}

// TestServicesNLBTLSInternalAllThree — annotation order /tls → (internal) → brackets.
func TestServicesNLBTLSInternalAllThree(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{
				Port: 9443, Protocol: "tls", ContainerPort: 8080, Scheme: "internal",
				Certificate:      "arn:aws:acm:us-east-1:123456789012:certificate/abc",
				CrossZone:        options.Bool(true),
				AllowCIDR:        []string{"10.0.0.0/24", "10.1.0.0/24"},
				PreserveClientIP: options.Bool(false),
			},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  9443:8080/tls(internal)[cz=true allow=2 pcip=false]",
		})
	})
}

// TestServicesNLBPerPortIndependentBrackets — proves per-cell brackets are independent.
func TestServicesNLBPerPortIndependentBrackets(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", CrossZone: options.Bool(true)},
			{Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "public"},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443[cz=true] 9443:8080",
		})
	})
}

// TestServicesNLBDifferentHardeningPerPort — different hardening per port renders independently.
func TestServicesNLBDifferentHardeningPerPort(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", CrossZone: options.Bool(true)},
			{Port: 9443, Protocol: "tcp", ContainerPort: 8080, Scheme: "public", AllowCIDR: []string{"10.0.0.0/24", "10.1.0.0/24"}},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443[cz=true] 9443:8080[allow=2]",
		})
	})
}

// TestServicesNLBEmptyAllowCIDRNoBracket — explicit empty slice must not render `allow=0`.
func TestServicesNLBEmptyAllowCIDRNoBracket(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		s := fxService()
		s.Nlb = []structs.ServiceNlbPort{
			{Port: 8443, Protocol: "tcp", ContainerPort: 8443, Scheme: "public", AllowCIDR: []string{}},
		}
		i.On("ServiceList", "app1").Return(structs.Services{*s}, nil)

		res, err := testExecute(e, "services -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DOMAIN  PORTS    NLB PORTS",
			"service1  domain  1:2 1:2  8443:8443",
		})
	})
}
