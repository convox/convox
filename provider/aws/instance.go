package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
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
