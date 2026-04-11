package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// terminateKarpenterEC2 terminates EC2 instances and waits for them to reach
// the terminated state (up to 120s). Used during Karpenter disable to kill
// kubelet before deleting K8s node objects.
func (p *Provider) terminateKarpenterEC2(instanceIDs []string) error {
	ids := make([]*string, len(instanceIDs))
	for i, id := range instanceIDs {
		ids[i] = aws.String(id)
	}

	fmt.Printf("KarpenterCleanup: terminating %d EC2 instances: %v\n", len(instanceIDs), instanceIDs)
	if _, err := p.Ec2.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: ids,
	}); err != nil {
		return fmt.Errorf("terminate instances: %w", err)
	}

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		out, err := p.Ec2.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: ids,
		})
		if err != nil {
			fmt.Printf("KarpenterCleanup WARNING: DescribeInstances: %v\n", err)
			time.Sleep(10 * time.Second)
			continue
		}

		allTerminated := true
		for _, res := range out.Reservations {
			for _, inst := range res.Instances {
				if aws.StringValue(inst.State.Name) != "terminated" {
					allTerminated = false
				}
			}
		}

		if allTerminated {
			fmt.Println("KarpenterCleanup: all EC2 instances terminated")
			return nil
		}
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("EC2 instances did not reach terminated state within 120s")
}
