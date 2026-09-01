package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func balancerManifest(b *Balancer) *Manifest {
	return &Manifest{
		Balancers: Balancers{*b},
		Services:  Services{{Name: "web"}},
	}
}

func balancerErrors(t *testing.T, b *Balancer) []string {
	t.Helper()

	var ss []string

	for _, err := range balancerManifest(b).validateBalancers() {
		ss = append(ss, err.Error())
	}

	return ss
}

func TestValidateBalancersTcpUdp(t *testing.T) {
	for _, c := range []struct {
		name     string
		balancer Balancer
		errs     []string
	}{
		{
			name: "accepted with the controller flag",
			balancer: Balancer{
				Name:                      "dns",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports:                     BalancerPorts{{Source: 53, Protocol: BalancerProtocolTcpUdp, Target: 5300}},
			},
		},
		{
			name: "rejected without the controller flag",
			balancer: Balancer{
				Name:    "dns",
				Service: "web",
				Ports:   BalancerPorts{{Source: 53, Protocol: BalancerProtocolTcpUdp, Target: 5300}},
			},
			errs: []string{"balancer dns port 53 uses protocol TCP_UDP, which requires awsLoadBalancerController: true"},
		},
		{
			name: "rejected with no target port",
			balancer: Balancer{
				Name:                      "dns",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports:                     BalancerPorts{{Source: 53, Protocol: BalancerProtocolTcpUdp}},
			},
			errs: []string{"balancer dns port 53 uses protocol TCP_UDP and must set a target port"},
		},
		{
			name: "both rules fire together",
			balancer: Balancer{
				Name:    "dns",
				Service: "web",
				Ports:   BalancerPorts{{Source: 53, Protocol: BalancerProtocolTcpUdp}},
			},
			errs: []string{
				"balancer dns port 53 uses protocol TCP_UDP, which requires awsLoadBalancerController: true",
				"balancer dns port 53 uses protocol TCP_UDP and must set a target port",
			},
		},
		{
			name: "the udp only health check rule stays quiet",
			balancer: Balancer{
				Name:                      "dns",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports: BalancerPorts{
					{Source: 53, Protocol: BalancerProtocolTcpUdp, Target: 5300},
					{Source: 514, Protocol: BalancerProtocolUdp, Target: 5140},
				},
			},
		},
		{
			name: "the udp only health check rule still fires",
			balancer: Balancer{
				Name:                      "syslog",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports:                     BalancerPorts{{Source: 514, Protocol: BalancerProtocolUdp, Target: 5140}},
			},
			errs: []string{"balancer syslog has UDP ports and no TCP port, set service.beta.kubernetes.io/aws-load-balancer-healthcheck-port in annotations"},
		},
		{
			name: "an unrelated protocol is still rejected",
			balancer: Balancer{
				Name:                      "sctp",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports:                     BalancerPorts{{Source: 3000, Protocol: "SCTP", Target: 3001}},
			},
			errs: []string{"balancer sctp port 3000 has unsupported protocol SCTP"},
		},
		{
			name: "a repeated source port points at the new value",
			balancer: Balancer{
				Name:                      "dns",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports: BalancerPorts{
					{Source: 53, Protocol: BalancerProtocolTcp, Target: 5300},
					{Source: 53, Protocol: BalancerProtocolUdp, Target: 5300},
				},
			},
			errs: []string{"balancer dns declares port 53 more than once, use protocol: TCP_UDP on a single entry to serve both protocols on one port number"},
		},
		{
			name: "a repeated source port on a legacy balancer is still allowed",
			balancer: Balancer{
				Name:    "legacy",
				Service: "web",
				Ports: BalancerPorts{
					{Source: 8000, Protocol: BalancerProtocolTcp, Target: 8001},
					{Source: 8000, Protocol: BalancerProtocolTcp, Target: 8002},
				},
			},
		},
		{
			name: "an empty protocol is still a tcp port",
			balancer: Balancer{
				Name:                      "mixed",
				Service:                   "web",
				AwsLoadBalancerController: true,
				Ports: BalancerPorts{
					{Source: 7000, Target: 7001},
					{Source: 514, Protocol: BalancerProtocolUdp, Target: 5140},
				},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.errs, balancerErrors(t, &c.balancer))
		})
	}
}

func TestValidateBalancersTcpUdpLowercase(t *testing.T) {
	m, err := Load([]byte(`balancers:
  dns:
    awsLoadBalancerController: true
    ports:
      53:
        protocol: tcp_udp
        port: 5300
    service: web
services:
  web:
    build: .
`), map[string]string{})
	require.NoError(t, err)
	require.Equal(t, BalancerProtocolTcpUdp, m.Balancers[0].Ports[0].Protocol)
	require.Empty(t, m.validateBalancers())
}
