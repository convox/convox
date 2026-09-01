package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ktesting "k8s.io/client-go/testing"
)

const balancerFreezeError = "balancer dns: TCP and UDP on one port number can only be set up on a new balancer, and its ports cannot be changed afterwards. It is currently serving 5353/TCP->5300, 5353/UDP->5300. Restore this balancer's previous ports in convox.yml to deploy it again, or replace it: add a second balancer with the ports you want and move traffic to it, or remove this one, deploy, then add it back. Replacing it gives a new address"

func livePort(name string, port, target int32, protocol ac.Protocol) ac.ServicePort {
	return ac.ServicePort{Name: name, Port: port, Protocol: protocol, TargetPort: intstr.FromInt32(target)}
}

func livePair() []ac.ServicePort {
	return []ac.ServicePort{
		livePort("5353-tcp", 5353, 5300, ac.ProtocolTCP),
		livePort("5353-udp", 5353, 5300, ac.ProtocolUDP),
	}
}

func tcpUdpBalancer(ports manifest.BalancerPorts) manifest.Balancer {
	return manifest.Balancer{Name: "dns", Service: "resolver", AwsLoadBalancerController: true, Ports: ports}
}

func TestValidateBalancerPortStability(t *testing.T) {
	pair := manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300}}

	for _, c := range []struct {
		name      string
		provider  string
		balancers manifest.Balancers
		live      []ac.ServicePort
		noService bool
		noRead    bool
		err       string
	}{
		{
			name:      "a non aws rack never reads",
			noRead:    true,
			provider:  "test",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5400}})},
			live:      livePair(),
		},
		{
			name:      "dropping the controller flag off a live pair is refused",
			provider:  "aws",
			balancers: manifest.Balancers{{Name: "dns", Service: "resolver", Ports: manifest.BalancerPorts{{Source: 5353, Protocol: "TCP", Target: 5300}}}},
			live:      livePair(),
			err:       balancerFreezeError,
		},
		{
			name:      "a balancer without the controller flag and no live pair is left alone",
			provider:  "aws",
			balancers: manifest.Balancers{{Name: "dns", Service: "resolver", Ports: manifest.BalancerPorts{{Source: 8000, Protocol: "TCP", Target: 9001}}}},
			live:      []ac.ServicePort{livePort("8000", 8000, 8001, ac.ProtocolTCP)},
		},
		{
			name:      "removing the balancer from the manifest never reads",
			noRead:    true,
			provider:  "aws",
			balancers: manifest.Balancers{},
			live:      livePair(),
		},
		{
			name:      "a manifest naming only another balancer never reads this one",
			noRead:    true,
			provider:  "aws",
			balancers: manifest.Balancers{{Name: "other", Service: "resolver", AwsLoadBalancerController: true, Ports: manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5400}}}},
			live:      livePair(),
		},
		{
			name:      "a balancer with no live service is the create path",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(pair)},
			noService: true,
		},
		{
			name:      "an unchanged pair is the steady state",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(pair)},
			live:      livePair(),
		},
		{
			name:     "an unchanged pair beside a long form port with no protocol or target",
			provider: "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
				{Source: 8080},
			})},
			live: append(livePair(), livePort("8080", 8080, 8080, ac.ProtocolTCP)),
		},
		{
			name:      "no mixed pair on either side is today's behavior",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{{Source: 8000, Protocol: "TCP", Target: 9000}})},
			live:      []ac.ServicePort{livePort("8000", 8000, 8001, ac.ProtocolTCP)},
		},
		{
			name:     "a same protocol repeat is not a pair",
			provider: "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{
				{Source: 8000, Protocol: "TCP", Target: 8001},
				{Source: 8000, Protocol: "TCP", Target: 8002},
			})},
			live: []ac.ServicePort{livePort("8000", 8000, 8001, ac.ProtocolTCP)},
		},
		{
			name:     "a skipped balancer does not stop the check",
			provider: "aws",
			balancers: manifest.Balancers{
				{Name: "unflagged", Service: "resolver", Ports: manifest.BalancerPorts{{Source: 8000, Protocol: "TCP", Target: 8001}}},
				{Name: "new", Service: "resolver", AwsLoadBalancerController: true, Ports: pair},
				tcpUdpBalancer(manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5400}}),
			},
			live: livePair(),
			err:  balancerFreezeError,
		},
		{
			name:      "a target port edit is refused",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5400}})},
			live:      livePair(),
			err:       balancerFreezeError,
		},
		{
			name:      "dropping to a single protocol is refused",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{{Source: 5353, Protocol: "UDP", Target: 5300}})},
			live:      livePair(),
			err:       balancerFreezeError,
		},
		{
			name:     "adding a port is refused",
			provider: "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
				{Source: 9090, Protocol: "TCP", Target: 9090},
			})},
			live: livePair(),
			err:  balancerFreezeError,
		},
		{
			name:      "removing a port is refused",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(pair)},
			live:      append(livePair(), livePort("9090", 9090, 9090, ac.ProtocolTCP)),
			err:       "balancer dns: TCP and UDP on one port number can only be set up on a new balancer, and its ports cannot be changed afterwards. It is currently serving 5353/TCP->5300, 5353/UDP->5300, 9090/TCP->9090. Restore this balancer's previous ports in convox.yml to deploy it again, or replace it: add a second balancer with the ports you want and move traffic to it, or remove this one, deploy, then add it back. Replacing it gives a new address",
		},
		{
			name:     "a reorder is refused while a pair is present",
			provider: "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{
				{Source: 8080, Protocol: "TCP", Target: 8080},
				{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300},
			})},
			live: append(livePair(), livePort("8080", 8080, 8080, ac.ProtocolTCP)),
			err:  "balancer dns: TCP and UDP on one port number can only be set up on a new balancer, and its ports cannot be changed afterwards. It is currently serving 5353/TCP->5300, 5353/UDP->5300, 8080/TCP->8080. Restore this balancer's previous ports in convox.yml to deploy it again, or replace it: add a second balancer with the ports you want and move traffic to it, or remove this one, deploy, then add it back. Replacing it gives a new address",
		},
		{
			name:     "a reorder is allowed with no pair present",
			provider: "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{
				{Source: 8080, Protocol: "TCP", Target: 8080},
				{Source: 9090, Protocol: "TCP", Target: 9090},
			})},
			live: []ac.ServicePort{
				livePort("9090", 9090, 9090, ac.ProtocolTCP),
				livePort("8080", 8080, 8080, ac.ProtocolTCP),
			},
		},
		{
			name:      "adding a pair to a live single port balancer is refused",
			provider:  "aws",
			balancers: manifest.Balancers{tcpUdpBalancer(pair)},
			live:      []ac.ServicePort{livePort("5353", 5353, 5300, ac.ProtocolTCP)},
			err:       "balancer dns: TCP and UDP on one port number can only be set up on a new balancer, and its ports cannot be changed afterwards. It is currently serving 5353/TCP->5300. Restore this balancer's previous ports in convox.yml to deploy it again, or replace it: add a second balancer with the ports you want and move traffic to it, or remove this one, deploy, then add it back. Replacing it gives a new address",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, kk, _ := minimalProvider(t)
			p.Provider = c.provider

			if !c.noService {
				createBalancerService(t, kk, p.AppNamespace("app1"), "dns", "external", true, c.live...)
			}

			kk.ClearActions()

			err := p.validateBalancerPortStability("app1", c.balancers)

			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, c.err)
			}

			if c.noRead {
				for _, a := range kk.Actions() {
					g, ok := a.(ktesting.GetAction)
					require.True(t, ok)
					require.NotEqual(t, "balancer-dns", g.GetName())
				}
			}
		})
	}
}

func TestValidateBalancerPortStabilityApiError(t *testing.T) {
	p, kk, _ := minimalProvider(t)
	p.Provider = "aws"

	kk.PrependReactor("get", "services", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, kerr.NewInternalError(context.DeadlineExceeded)
	})

	err := p.validateBalancerPortStability("app1", manifest.Balancers{tcpUdpBalancer(manifest.BalancerPorts{{Source: 5353, Protocol: manifest.BalancerProtocolTcpUdp, Target: 5300}})})
	require.ErrorContains(t, err, "Internal error occurred")
}

const balancerFreezeManifest = `balancers:
  dns:
    service: web
    awsLoadBalancerController: true
    ports:
      5353:
        protocol: TCP_UDP
        port: 5400
services:
  web:
    build: .
`

func TestReleasePromoteRejectsBalancerPortChange(t *testing.T) {
	t.Setenv("TEST", "true")

	p, kk, kc := minimalProvider(t)
	p.Provider = "aws"

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: p.Namespace}}, am.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, p.Initialize(structs.ProviderOptions{}))

	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", balancerFreezeManifest)
	createBalancerService(t, kk, p.AppNamespace("app1"), "dns", "external", true, livePair()...)

	require.EqualError(t, p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{}), balancerFreezeError)
}
