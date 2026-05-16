package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	k8s "github.com/convox/convox/provider/k8s"
	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) KarpenterCleanup() error {
	ctx := p.Context()

	nodes, err := p.Cluster.CoreV1().Nodes().List(ctx, am.ListOptions{
		LabelSelector: k8s.KarpenterNodePoolLabel,
	})
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to list karpenter nodes: %s", err))
	}

	if len(nodes.Items) == 0 {
		return nil
	}

	var instanceIDs []*string
	for _, node := range nodes.Items {
		if id := parseInstanceID(node.Spec.ProviderID); id != "" {
			instanceIDs = append(instanceIDs, aws.String(id))
		}
	}

	// Terminate EC2 instances individually — batch TerminateInstances fails
	// atomically if any instance is already gone (InvalidInstanceID.NotFound).
	// Per-instance calls let us skip terminated instances and still clean up the rest.
	for _, id := range instanceIDs {
		_, err := p.Ec2.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{id},
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && strings.Contains(aerr.Code(), "InvalidInstanceID") {
				continue
			}
			return errors.WithStack(fmt.Errorf("failed to terminate EC2 instance %s: %s", *id, err))
		}
	}

	// Now cordon, drain, and delete k8s node objects.
	// EC2 instances are already terminating, so pods will be evicted naturally,
	// but we drain explicitly to respect PDBs and ensure graceful shutdown.
	if err := p.Provider.KarpenterCleanup(); err != nil {
		return err
	}

	return nil
}

