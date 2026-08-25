package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
)

func balancerRenderProvider(provider string) *Provider {
	p := &Provider{Engine: &mock.TestEngine{}, Name: "rack1", Provider: provider}
	p.templater = templater.New(template.TemplatesFS)
	return p
}

func renderBalancer(t *testing.T, provider string, b *manifest.Balancer) string {
	t.Helper()

	data, err := balancerRenderProvider(provider).releaseTemplateBalancer(&structs.App{Name: "app1"}, &structs.Release{Id: "release1"}, *b, nil)
	require.NoError(t, err)

	return string(data)
}

func TestRenderBalancerLegacy(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:    "alpha",
		Service: "web",
		Ports:   manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout: \"3600\"\n")
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-type: nlb\n")
	require.NotContains(t, out, "aws-load-balancer-nlb-target-type")
	require.NotContains(t, out, "aws-load-balancer-scheme")
	require.NotContains(t, out, "aws-load-balancer-healthcheck")
}

func TestRenderBalancerController(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Ports: manifest.BalancerPorts{
			{Source: 6000, Protocol: "UDP", Target: 6001},
			{Source: 5000, Protocol: "TCP", Target: 5001},
			{Source: 7000, Protocol: "TCP", Target: 7001},
		},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-type: external\n")
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: ip\n")
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing\n")
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-healthcheck-protocol: tcp\n")
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-healthcheck-port: \"5001\"\n")
	require.NotContains(t, out, "aws-load-balancer-connection-idle-timeout")
	require.Contains(t, out, "protocol: TCP\n")
	require.Contains(t, out, "protocol: UDP\n")
}

func TestRenderBalancerControllerUnsetProtocolIsTcp(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Ports: manifest.BalancerPorts{
			{Source: 5000, Target: 5001},
			{Source: 6000, Protocol: "UDP", Target: 6001},
		},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-healthcheck-port: \"5001\"\n")
}

func TestRenderBalancerControllerTcpOnly(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Ports:                     manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
	})

	require.NotContains(t, out, "aws-load-balancer-healthcheck")
}

func TestRenderBalancerControllerSchemeOverride(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Annotations:               manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-scheme=internal"},
		Ports:                     manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-scheme: internal\n")
	require.NotContains(t, out, "internet-facing")
}

func TestRenderBalancerControllerInternalAnnotation(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Annotations:               manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-internal=true"},
		Ports:                     manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-internal: \"true\"\n")
	require.NotContains(t, out, "aws-load-balancer-scheme")
}

func TestRenderBalancerControllerRequiresAws(t *testing.T) {
	_, err := balancerRenderProvider("gcp").releaseTemplateBalancer(&structs.App{Name: "app1"}, &structs.Release{Id: "release1"}, manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Ports:                     manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
	}, nil)

	require.EqualError(t, err, "balancer alpha: awsLoadBalancerController is only supported on AWS racks")
}
