package do

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/elastic/go-elasticsearch/v6"
)

type Provider struct {
	*k8s.Provider

	Bucket         string
	Region         string
	Registry       string
	Secret         string
	SpacesAccess   string
	SpacesEndpoint string
	SpacesSecret   string

	Elastic *elasticsearch.Client
	S3      s3iface.S3API
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider:       k,
		Bucket:         os.Getenv("BUCKET"),
		Region:         os.Getenv("REGION"),
		Registry:       os.Getenv("REGISTRY"),
		Secret:         os.Getenv("SECRET"),
		SpacesAccess:   os.Getenv("SPACES_ACCESS"),
		SpacesEndpoint: os.Getenv("SPACES_ENDPOINT"),
		SpacesSecret:   os.Getenv("SPACES_SECRET"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeDOServices(); err != nil {
		return err
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}

func (p *Provider) initializeDOServices() error {
	es, err := elasticsearch.NewDefaultClient()
	if err != nil {
		return err
	}

	p.Elastic = es

	s, err := session.NewSession(&aws.Config{
		Region:      aws.String(p.Region),
		Credentials: credentials.NewStaticCredentials(p.SpacesAccess, p.SpacesSecret, ""),
	})
	if err != nil {
		return err
	}

	p.S3 = s3.New(s, &aws.Config{
		Endpoint: aws.String(p.SpacesEndpoint),
	})

	return nil
}
