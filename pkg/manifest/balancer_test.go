package manifest_test

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
)

func TestBalancerAwsLoadBalancerController(t *testing.T) {
	m, err := testdataManifest("balancers", map[string]string{})
	require.NoError(t, err)

	require.Equal(t, manifest.Balancers{
		manifest.Balancer{
			Name:                      "controller",
			AwsLoadBalancerController: true,
			Ports: manifest.BalancerPorts{
				{Source: 5000, Protocol: "TCP", Target: 5001},
				{Source: 6000, Protocol: "UDP", Target: 6001},
				{Source: 7000, Target: 7001},
			},
			Service: "web",
		},
		manifest.Balancer{
			Name: "legacy",
			Ports: manifest.BalancerPorts{
				{Source: 8000, Protocol: "TCP", Target: 8001},
				{Source: 8000, Protocol: "TCP", Target: 8002},
			},
			Service: "web",
		},
		manifest.Balancer{
			Name:                      "udponly",
			Annotations:               manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-healthcheck-port=9001"},
			AwsLoadBalancerController: true,
			Ports:                     manifest.BalancerPorts{{Source: 9000, Protocol: "UDP", Target: 9001}},
			Service:                   "web",
		},
	}, m.Balancers)
}
