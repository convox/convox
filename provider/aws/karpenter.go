package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
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

	if err := p.Provider.KarpenterCleanup(); err != nil {
		return err
	}

	if len(instanceIDs) > 0 {
		_, err := p.Ec2.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			return errors.WithStack(fmt.Errorf("failed to terminate EC2 instances: %s", err))
		}
	}

	return nil
}

func parseInstanceID(providerID string) string {
	if !strings.HasPrefix(providerID, "aws://") {
		return ""
	}
	id := strings.TrimPrefix(providerID, "aws://")
	parts := strings.Split(id, "/")
	instanceID := parts[len(parts)-1]
	if !strings.HasPrefix(instanceID, "i-") {
		return ""
	}
	return instanceID
}
