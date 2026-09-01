package k8s

import (
	"strings"
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
	require.NotContains(t, out, "enable-tcp-udp-listener")
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

func balancerRenderedPorts(t *testing.T, out string) string {
	t.Helper()

	i := strings.Index(out, "\n  ports:\n")
	j := strings.Index(out, "\n  selector:\n")
	require.NotEqual(t, -1, i)
	require.NotEqual(t, -1, j)

	return out[i+1 : j+1]
}

func TestRenderBalancerControllerPortsGolden(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "alpha",
		Service:                   "web",
		AwsLoadBalancerController: true,
		Ports: manifest.BalancerPorts{
			{Source: 5000, Protocol: "TCP", Target: 5001},
			{Source: 7000, Target: 7001},
			{Source: 6000, Protocol: "UDP", Target: 6001},
		},
	})

	require.Equal(t, `  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
    targetPort: 5001
  - name: "7000"
    port: 7000
    protocol: null
    targetPort: 7001
  - name: "6000"
    port: 6000
    protocol: UDP
    targetPort: 6001
`, balancerRenderedPorts(t, out))
}

func TestRenderBalancerControllerTcpUdpGolden(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "dns",
		Service:                   "resolver",
		AwsLoadBalancerController: true,
		Ports: manifest.BalancerPorts{
			{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
			{Source: 8080, Protocol: "TCP", Target: 8080},
		},
	})

	require.Equal(t, `  ports:
  - name: 5353-tcp
    port: 5353
    protocol: TCP
    targetPort: 5300
  - name: 5353-udp
    port: 5353
    protocol: UDP
    targetPort: 5300
  - name: "8080"
    port: 8080
    protocol: TCP
    targetPort: 8080
`, balancerRenderedPorts(t, out))
	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-enable-tcp-udp-listener: \"true\"\n")
	require.NotContains(t, out, "aws-load-balancer-healthcheck")
}

func TestRenderBalancerControllerTcpUdpKeepsGeneratedListenerAnnotation(t *testing.T) {
	out := renderBalancer(t, "aws", &manifest.Balancer{
		Name:                      "dns",
		Service:                   "resolver",
		AwsLoadBalancerController: true,
		Annotations:               manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-enable-tcp-udp-listener=false"},
		Ports:                     manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300}},
	})

	require.Contains(t, out, "service.beta.kubernetes.io/aws-load-balancer-enable-tcp-udp-listener: \"true\"\n")
	require.NotContains(t, out, "enable-tcp-udp-listener: \"false\"")
}

func TestRenderBalancerTcpUdpRequiresController(t *testing.T) {
	_, err := balancerRenderProvider("aws").releaseTemplateBalancer(&structs.App{Name: "app1"}, &structs.Release{Id: "release1"}, manifest.Balancer{
		Name:    "dns",
		Service: "resolver",
		Ports:   manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300}},
	}, nil)

	require.EqualError(t, err, "balancer dns: protocol TCP_UDP requires awsLoadBalancerController: true, update convox.yml and build again")
}

func TestBalancerRenderPorts(t *testing.T) {
	ports, tcpUdp := balancerRenderPorts(manifest.BalancerPorts{
		{Source: 5000, Protocol: "TCP", Target: 5001},
		{Source: 7000, Target: 7001},
		{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
	})

	require.True(t, tcpUdp)
	require.Equal(t, []balancerRenderPort{
		{Name: "5000", Source: 5000, Protocol: "TCP", Target: 5001},
		{Name: "7000", Source: 7000, Protocol: "", Target: 7001},
		{Name: "5353-tcp", Source: 5353, Protocol: "TCP", Target: 5300},
		{Name: "5353-udp", Source: 5353, Protocol: "UDP", Target: 5300},
	}, ports)

	ports, tcpUdp = balancerRenderPorts(manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}})

	require.False(t, tcpUdp)
	require.Equal(t, []balancerRenderPort{{Name: "5000", Source: 5000, Protocol: "TCP", Target: 5001}}, ports)
}

func TestBalancerHealthCheckPortTcpUdp(t *testing.T) {
	for _, c := range []struct {
		name  string
		ports manifest.BalancerPorts
		want  int
	}{
		{
			name:  "a pair alone needs no annotation",
			ports: manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300}},
		},
		{
			name: "a pair beside a tcp port needs no annotation",
			ports: manifest.BalancerPorts{
				{Source: 8080, Protocol: "TCP", Target: 8080},
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
			},
		},
		{
			name: "a pair beside a udp port probes the pair",
			ports: manifest.BalancerPorts{
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
				{Source: 514, Protocol: "UDP", Target: 5140},
			},
			want: 5300,
		},
		{
			name: "a udp port before a pair still probes the pair",
			ports: manifest.BalancerPorts{
				{Source: 514, Protocol: "UDP", Target: 5140},
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
			},
			want: 5300,
		},
		{
			name:  "udp only is unchanged",
			ports: manifest.BalancerPorts{{Source: 514, Protocol: "UDP", Target: 5140}},
		},
		{
			name:  "tcp only is unchanged",
			ports: manifest.BalancerPorts{{Source: 5000, Protocol: "TCP", Target: 5001}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, balancerHealthCheckPort(c.ports))
		})
	}
}
