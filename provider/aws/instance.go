package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
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
		return nil, fmt.Errorf("instance type not found")
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
