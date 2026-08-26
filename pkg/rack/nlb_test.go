package rack

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	ac "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func routerService(namespace string, class *string) *ac.Service {
	return &ac.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "router", Namespace: namespace},
		Spec:       ac.ServiceSpec{Type: ac.ServiceTypeLoadBalancer, LoadBalancerClass: class},
	}
}

func TestRouterSecurityGroupWarning(t *testing.T) {
	empty := ""
	managed := "service.k8s.aws/nlb"
	other := "example.com/custom"

	for _, tc := range []struct {
		name   string
		client kubernetes.Interface
		expect string
	}{
		{
			name:   "class unset warns",
			client: fake.NewSimpleClientset(routerService("myrack-system", nil)),
			expect: nlbSecurityGroupInertWarning,
		},
		{
			name:   "class empty warns",
			client: fake.NewSimpleClientset(routerService("myrack-system", &empty)),
			expect: nlbSecurityGroupInertWarning,
		},
		{
			name:   "controller managed stays quiet",
			client: fake.NewSimpleClientset(routerService("myrack-system", &managed)),
			expect: "",
		},
		{
			name:   "foreign class stays quiet",
			client: fake.NewSimpleClientset(routerService("myrack-system", &other)),
			expect: "",
		},
		{
			name:   "missing service stays quiet",
			client: fake.NewSimpleClientset(),
			expect: "",
		},
		{
			name:   "service in another namespace stays quiet",
			client: fake.NewSimpleClientset(routerService("otherrack-system", nil)),
			expect: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, routerSecurityGroupWarning(context.Background(), tc.client, "myrack"))
		})
	}
}

func TestRouterSecurityGroupWarningUnreadable(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "router", fmt.Errorf("no access"))
	})

	got := routerSecurityGroupWarning(context.Background(), client, "myrack")

	assert.Contains(t, got, "could not check whether nlb_security_group applies")
	assert.NotEqual(t, nlbSecurityGroupInertWarning, got)
}

func TestInertNLBSecurityGroupWarningSkipsNonAWS(t *testing.T) {
	assert.Empty(t, Terraform{provider: "gcp", name: "myrack"}.inertNLBSecurityGroupWarning())
}

func TestInertNLBSecurityGroupWarningParameterGate(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
	}{
		{name: "parameter absent", vars: map[string]string{"region": "us-east-1"}},
		{name: "parameter cleared", vars: map[string]string{"region": "us-east-1", "nlb_security_group": ""}},
		{name: "parameter whitespace", vars: map[string]string{"region": "us-east-1", "nlb_security_group": "   "}},
		{name: "private endpoint skipped", vars: map[string]string{"region": "us-east-1", "nlb_security_group": "sg-abc123", "private_eks_host": "https://example.com/eks/"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			f := &reconcileFixture{rackName: "myrack", vars: tc.vars}
			f.setup(t, root)

			var got string
			withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
				got = tf.inertNLBSecurityGroupWarning()
			})

			assert.Empty(t, got)
		})
	}
}
