package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) InstanceKeyroll() (*structs.KeyPair, error) {
	key := fmt.Sprintf("%s-keypair-%d", p.Name, time.Now().Unix())

	res, err := p.Ec2.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: &key,
	})
	if err != nil {
		return nil, err
	}

	return &structs.KeyPair{
		Name:       &key,
		PrivateKey: res.KeyMaterial,
	}, nil
}

func (p *Provider) InstanceTerminate(id string) error {
	ctx := p.Provider.Context()

	node, err := p.Cluster.CoreV1().Nodes().Get(ctx, id, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return errors.WithStack(structs.ErrNotFound("instance not found: %s", id))
		}
		return errors.WithStack(err)
	}

	instanceID := parseInstanceID(node.Spec.ProviderID)

	if err := p.Provider.InstanceTerminate(id); err != nil {
		return err
	}

	if instanceID != "" {
		_, err := p.Ec2.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{aws.String(instanceID)},
		})
		if err != nil {
			return errors.WithStack(fmt.Errorf("failed to terminate EC2 instance %s: %s", instanceID, err))
		}
	}

	return nil
}

func parseInstanceID(providerID string) string {
	if !strings.HasPrefix(providerID, "aws://") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(providerID, "aws://"), "/")
	instanceID := parts[len(parts)-1]
	if !strings.HasPrefix(instanceID, "i-") {
		return ""
	}
	return instanceID
}

func (p *Provider) GPUIntanceList(instanceTypes []string) ([]string, error) {
	return isNvididGpuInstanceType(p.Ec2, instanceTypes)
}

// Check if instance type has NVIDIA GPU (based on hardware accelerator info)
func isNvididGpuInstanceType(client *ec2.EC2, instanceTypes []string) ([]string, error) {
	input := &ec2.DescribeInstanceTypesInput{
		InstanceTypes: options.StringPtrArray(instanceTypes),
	}

	resp, err := client.DescribeInstanceTypes(input)
	if err != nil {
		return nil, err
	}

	if len(resp.InstanceTypes) == 0 {
		return nil, structs.ErrNotFound("instance type not found")
	}

	results := []string{}
	for i := range resp.InstanceTypes {
		isNvida := false
		if resp.InstanceTypes[i].GpuInfo != nil {
			for _, accel := range resp.InstanceTypes[i].GpuInfo.Gpus {
				if accel.Manufacturer != nil && *accel.Manufacturer == "NVIDIA" {
					isNvida = true
					break
				}
			}
		}

		if isNvida && resp.InstanceTypes[i].InstanceType != nil {
			results = append(results, *resp.InstanceTypes[i].InstanceType)
		}
	}

	return results, nil
}
