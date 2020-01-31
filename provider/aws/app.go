package aws

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"k8s.io/apimachinery/pkg/types"
)

func (p *Provider) AppCreate(name string, opts structs.AppCreateOptions) (*structs.App, error) {
	a, err := p.Provider.AppCreate(name, opts)
	if err != nil {
		return nil, err
	}

	res, err := p.ECR.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(fmt.Sprintf("%s/%s", p.Name, name)),
	})
	if err != nil {
		return nil, err
	}

	patches := []k8s.Patch{
		{Op: "add", Path: "/metadata/annotations/convox.com~1registry", Value: *res.Repository.RepositoryUri},
	}

	patch, err := json.Marshal(patches)
	if err != nil {
		return nil, err
	}

	if _, err := p.Cluster.CoreV1().Namespaces().Patch(p.AppNamespace(name), types.JSONPatchType, patch); err != nil {
		return nil, err
	}

	return a, nil
}

func (p *Provider) AppDelete(name string) error {
	_, err := p.ECR.DeleteRepository(&ecr.DeleteRepositoryInput{
		Force:          aws.Bool(true),
		RepositoryName: aws.String(fmt.Sprintf("%s/%s", p.Name, name)),
	})
	if err != nil {
		switch awsErrorCode(err) {
		case "RepositoryNotFoundException":
		default:
			return err
		}
	}

	return p.Provider.AppDelete(name)
}
