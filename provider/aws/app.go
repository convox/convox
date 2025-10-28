package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (p *Provider) AppCreate(name string, opts structs.AppCreateOptions) (*structs.App, error) {
	a, err := p.Provider.AppCreate(name, opts)
	if err != nil {
		return nil, err
	}

	res, err := p.ECR.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(fmt.Sprintf("%s%s", p.RepositoryPrefix(), name)),
		ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
			ScanOnPush: aws.Bool(p.EcrScanOnPushEnable),
		},
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

	if _, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(), p.AppNamespace(name), types.JSONPatchType, patch, v1.PatchOptions{}); err != nil {
		return nil, err
	}

	return a, nil
}

func (p *Provider) AppDelete(name string) error {
	err := p.cleanUpPodIdentity(name)
	if err != nil {
		return fmt.Errorf("cleanup associated pod identitiy: %s", err)
	}

	_, err = p.ECR.DeleteRepository(&ecr.DeleteRepositoryInput{
		Force:          aws.Bool(true),
		RepositoryName: aws.String(fmt.Sprintf("%s%s", p.RepositoryPrefix(), name)),
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

func (p *Provider) cleanUpPodIdentity(app string) error {
	if os.Getenv("TEST") == "true" {
		return nil
	}

	podIdentityResp, err := p.EKS.ListPodIdentityAssociations(&eks.ListPodIdentityAssociationsInput{
		ClusterName: aws.String(p.Name),
		Namespace:   aws.String(p.Provider.AppNamespace(app)),
	})
	if err != nil {
		return fmt.Errorf("failed to list associated pod identitiy: %s", err)
	}

	for i := range podIdentityResp.Associations {
		role := p.getRoleName(app, *podIdentityResp.Associations[i].ServiceAccount)

		rolePolicesResp, err := p.IAM.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(role),
		})
		if err != nil {
			return err
		}

		for i := range rolePolicesResp.AttachedPolicies {
			_, err = p.IAM.DetachRolePolicy(&iam.DetachRolePolicyInput{
				PolicyArn: rolePolicesResp.AttachedPolicies[i].PolicyArn,
				RoleName:  aws.String(role),
			})
			if err != nil {
				return err
			}
		}

		_, err = p.IAM.DeleteRole(&iam.DeleteRoleInput{
			RoleName: aws.String(role), // service name is same as sa name
		})
		p.IAM.DetachRolePolicy(&iam.DetachRolePolicyInput{})
		if err != nil {
			if awsErrorCode(err) != iam.ErrCodeNoSuchEntityException {
				return err
			}
		}

		_, err = p.EKS.DeletePodIdentityAssociation(&eks.DeletePodIdentityAssociationInput{
			AssociationId: podIdentityResp.Associations[i].AssociationId,
			ClusterName:   podIdentityResp.Associations[i].ClusterName,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
