package rack

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const nlbSecurityGroupInertWarning = "WARNING: nlb_security_group is set but is not being applied to this rack's primary router load balancer. That load balancer is not managed by the AWS Load Balancer Controller, so the security group has no effect on it. Attaching one requires replacing the load balancer, so contact Convox support to plan that change.\n"

func (t Terraform) inertNLBSecurityGroupWarning() string {
	if t.provider != "aws" {
		return ""
	}

	vars, err := t.vars()
	if err != nil {
		return ""
	}

	if strings.TrimSpace(vars["nlb_security_group"]) == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := strings.ToLower(t.name)

	client, err := eksClient(ctx, cluster, vars)
	if err != nil {
		return ""
	}

	return routerSecurityGroupWarning(ctx, client, cluster, vars["private_eks_host"] != "")
}

// An empty loadBalancerClass predates the controller, which never adopts it.
// A missing Service is not evidence either way.
func routerSecurityGroupWarning(ctx context.Context, client kubernetes.Interface, rack string, privateEKS bool) string {
	svc, err := client.CoreV1().Services(rack+"-system").Get(ctx, "router", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ""
		}

		// a transport error carries the request URL, and the private host is masked everywhere else
		if privateEKS {
			return "NOTICE: could not check whether nlb_security_group applies to this rack\n"
		}

		return fmt.Sprintf("NOTICE: could not check whether nlb_security_group applies to this rack: %v\n", err)
	}

	if class := svc.Spec.LoadBalancerClass; class == nil || *class == "" {
		return nlbSecurityGroupInertWarning
	}

	return ""
}
