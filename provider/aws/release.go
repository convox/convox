package aws

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

func (p *Provider) ReleasePromote(app, id string, opts structs.ReleasePromoteOptions) error {
	m, _, err := common.ReleaseManifest(p, app, id)
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range m.Services {
		err = p.processAccessControl(app, m.Services[i].Name, m.Services[i].AccessControl)
		if err != nil {
			return err
		}
	}

	return p.Provider.ReleasePromote(app, id, opts)
}

func (p *Provider) processAccessControl(app, service string, opts manifest.AccessControlOptions) error {
	return p.processAwsPodIdentityAccess(app, service, opts.AWSPodIdentity)
}

func (p *Provider) processAwsPodIdentityAccess(app, service string, opts *manifest.AWSPodIdentityOptions) error {
	if opts == nil || len(opts.PolicyArns) == 0 {
		return nil
	}

	roleArn, err := p.createRoleIfNotExist(app, service)
	if err != nil {
		return fmt.Errorf("failed to create role: %s", err)
	}

	if err := p.syncRolePolices(app, service, opts); err != nil {
		return fmt.Errorf("failed to sync role policies: %s", err)
	}

	return p.associatePodIdentityIfNotExist(app, service, roleArn)
}

func (p *Provider) createRoleIfNotExist(app, service string) (string, error) {
	role, err := p.IAM.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(p.getRoleName(app, service)),
	})
	if err != nil {
		trustPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"pods.eks.amazonaws.com"},"Action":["sts:AssumeRole","sts:TagSession"]}]}`
		nrole, err := p.IAM.CreateRole(&iam.CreateRoleInput{
			RoleName:                 aws.String(p.getRoleName(app, service)),
			AssumeRolePolicyDocument: &trustPolicy,
			Description:              aws.String(fmt.Sprintf("iam role for %s service %s app", service, app)),
			Path:                     aws.String("/convox/"),
			Tags: []*iam.Tag{
				{
					Key:   aws.String("Rack"),
					Value: aws.String(p.Name),
				},
				{
					Key:   aws.String("System"),
					Value: aws.String("convox"),
				},
				{
					Key:   aws.String("App"),
					Value: aws.String(app),
				},
				{
					Key:   aws.String("Service"),
					Value: aws.String(service),
				},
			},
		})
		if err != nil {
			return "", err
		}
		return *nrole.Role.Arn, nil
	}
	return *role.Role.Arn, nil
}

func (p *Provider) syncRolePolices(app, service string, opts *manifest.AWSPodIdentityOptions) error {
	if opts == nil || len(opts.PolicyArns) == 0 {
		return nil
	}

	resp, err := p.IAM.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
		MaxItems: aws.Int64(1000),
		RoleName: aws.String(p.getRoleName(app, service)),
	})
	if err != nil {
		return fmt.Errorf("failed to list role policies: %s", err)
	}

	pMap := map[string]int{}
	for i := range opts.PolicyArns {
		pMap[opts.PolicyArns[i]] = 1 // need to attach
	}

	for i := range resp.AttachedPolicies {
		if _, has := pMap[*resp.AttachedPolicies[i].PolicyArn]; !has {
			pMap[*resp.AttachedPolicies[i].PolicyArn] = -1 // need to detach
		} else {
			pMap[*resp.AttachedPolicies[i].PolicyArn] = 0 // no need to attach
		}
	}

	for k, v := range pMap {
		if v == 1 {
			_, err = p.IAM.AttachRolePolicy(&iam.AttachRolePolicyInput{
				RoleName:  aws.String(p.getRoleName(app, service)),
				PolicyArn: aws.String(k),
			})
			if err != nil {
				return fmt.Errorf("failed to attach the policy: %s", err)
			}
		} else if v == -1 {
			_, err = p.IAM.DetachRolePolicy(&iam.DetachRolePolicyInput{
				RoleName:  aws.String(p.getRoleName(app, service)),
				PolicyArn: aws.String(k),
			})
			if err != nil {
				return fmt.Errorf("failed to dettach the policy: %s", err)
			}
		}
	}

	return nil
}

func (p *Provider) associatePodIdentityIfNotExist(app, service, roleArn string) error {
	resp, err := p.EKS.ListPodIdentityAssociations(&eks.ListPodIdentityAssociationsInput{
		ClusterName:    aws.String(p.Name),
		Namespace:      aws.String(p.Provider.AppNamespace(app)),
		ServiceAccount: aws.String(service),
	})
	if err != nil {
		return fmt.Errorf("failed to list associated pod identitiy: %s", err)
	}

	if len(resp.Associations) == 0 {
		_, err = p.EKS.CreatePodIdentityAssociation(&eks.CreatePodIdentityAssociationInput{
			ClusterName:    aws.String(p.Name),
			Namespace:      aws.String(p.Provider.AppNamespace(app)),
			ServiceAccount: aws.String(service),
			RoleArn:        aws.String(roleArn),
		})
		if err != nil {
			return fmt.Errorf("failed to list associated pod identitiy: %s", err)
		}
		// wait 90s to sync pod identity association
		time.Sleep(90 * time.Second)
	}

	return nil
}

func (p *Provider) getRoleName(app, service string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%s", p.Name, app, service)))
	name := fmt.Sprintf("%s-%s-%s-%x", p.Name, app, service, hash[0:8])
	if len(name) > 63 {
		name = name[0:62]
	}
	return name

}
