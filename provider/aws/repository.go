package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	ecrTypes "github.com/aws/aws-sdk-go/service/ecr"
)

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	host, _, err := p.RepositoryHost(app)
	if err != nil {
		return "", "", err
	}

	return p.ecrAuth(host, "", "")
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	registry, err := p.appRegistry(app)
	if err != nil {
		return "", false, err
	}

	return registry, true, nil
}

func (p *Provider) RepositoryPrefix() string {
	if p.ContextTID() != "" {
		return fmt.Sprintf("%s-%s/", p.Name, p.ContextTID())
	}
	return fmt.Sprintf("%s/", p.Name)
}

func (p *Provider) RepositoryImagesBatchDelete(app string, tags []string) error {
	imageIds := []*ecrTypes.ImageIdentifier{}

	for _, tag := range tags {
		imageIds = append(imageIds, &ecrTypes.ImageIdentifier{ImageTag: aws.String(tag)})
	}

	repo := aws.String(fmt.Sprintf("%s%s", p.RepositoryPrefix(), app))

	for len(imageIds) > 0 {
		n := min(len(imageIds), 100)

		if _, err := p.ECR.BatchDeleteImage(&ecrTypes.BatchDeleteImageInput{
			RepositoryName: repo,
			ImageIds:       imageIds[:n],
		}); err != nil {
			return err
		}

		imageIds = imageIds[n:]
	}

	return nil
}
