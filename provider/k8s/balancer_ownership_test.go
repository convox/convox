package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

const balancerFlipError = "balancer custom cannot be switched to the AWS Load Balancer Controller in place, because the existing load balancer would be left running in your AWS account. Add a new balancer with awsLoadBalancerController: true, move traffic to it, then remove this one"

const balancerControllerManifest = `balancers:
  custom:
    service: web
    awsLoadBalancerController: true
    ports:
      5000: 5001
      6000:
        protocol: UDP
        port: 6001
resources:
  cache:
    type: elasticache-redis
services:
  web:
    build: .
    port: 5000
`

func createBalancerService(t *testing.T, kk *fake.Clientset, ns, name, lbType string, ingress bool) {
	t.Helper()

	s := &ac.Service{
		ObjectMeta: am.ObjectMeta{
			Name:        "balancer-" + name,
			Namespace:   ns,
			Annotations: map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": lbType},
		},
	}

	if ingress {
		s.Status.LoadBalancer.Ingress = []ac.LoadBalancerIngress{{Hostname: "lb.example.com"}}
	}

	_, err := kk.CoreV1().Services(ns).Create(context.TODO(), s, am.CreateOptions{})
	require.NoError(t, err)
}

func TestValidateBalancerOwnership(t *testing.T) {
	controller := manifest.Balancer{Name: "custom", Service: "web", AwsLoadBalancerController: true}
	legacy := manifest.Balancer{Name: "custom", Service: "web"}
	annotated := manifest.Balancer{
		Name:        "custom",
		Service:     "web",
		Annotations: manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-type=external"},
	}
	annotatedNlbIp := manifest.Balancer{
		Name:        "custom",
		Service:     "web",
		Annotations: manifest.BalancerAnnotations{"service.beta.kubernetes.io/aws-load-balancer-type=nlb-ip"},
	}

	for _, c := range []struct {
		name      string
		provider  string
		balancer  manifest.Balancer
		liveType  string
		ingress   bool
		noService bool
		err       string
	}{
		{name: "no existing service", provider: "aws", balancer: controller, noService: true},
		{name: "existing service never got an address", provider: "aws", balancer: controller, liveType: "nlb"},
		{name: "existing load balancer would be orphaned", provider: "aws", balancer: controller, liveType: "nlb", ingress: true, err: balancerFlipError},
		{name: "already on the controller", provider: "aws", balancer: controller, liveType: "external", ingress: true},
		{name: "unflagged balancer keeps its load balancer", provider: "aws", balancer: legacy, liveType: "nlb", ingress: true},
		{name: "annotation only balancer already on the controller", provider: "aws", balancer: annotated, liveType: "external", ingress: true},
		{name: "annotation only flip is refused", provider: "aws", balancer: annotated, liveType: "nlb", ingress: true, err: balancerFlipError},
		{name: "annotation only flip to the older spelling is refused", provider: "aws", balancer: annotatedNlbIp, liveType: "nlb", ingress: true, err: balancerFlipError},
		{name: "already on the controller under the older spelling", provider: "aws", balancer: controller, liveType: "nlb-ip", ingress: true},
		{name: "non aws rack", provider: "test", balancer: controller, liveType: "nlb", ingress: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, kk, _ := minimalProvider(t)
			p.Provider = c.provider

			if !c.noService {
				createBalancerService(t, kk, p.AppNamespace("app1"), c.balancer.Name, c.liveType, c.ingress)
			}

			err := p.validateBalancerOwnership("app1", manifest.Balancers{c.balancer})

			if c.err == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, c.err)
		})
	}
}

func TestValidateBalancerOwnershipApiError(t *testing.T) {
	p, kk, _ := minimalProvider(t)
	p.Provider = "aws"

	kk.PrependReactor("get", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, kerr.NewInternalError(context.DeadlineExceeded)
	})

	err := p.validateBalancerOwnership("app1", manifest.Balancers{{Name: "custom", Service: "web", AwsLoadBalancerController: true}})
	require.Error(t, err)
}

func TestReleasePromoteRejectsBalancerControllerFlip(t *testing.T) {
	t.Setenv("TEST", "true")

	p, kk, kc := minimalProvider(t)
	p.Provider = "aws"
	p.FeatureGates = map[string]bool{options.FeatureGateElasticacheDisable: true}

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: p.Namespace}}, am.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, p.Initialize(structs.ProviderOptions{}))

	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", balancerControllerManifest)
	createBalancerService(t, kk, p.AppNamespace("app1"), "custom", "nlb", true)

	require.EqualError(t, p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{}), balancerFlipError)
}
