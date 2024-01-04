package exoscale

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/convox/convox/pkg/elastic"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
)

type Provider struct {
	*k8s.Provider

	Bucket string
	Zone   string

	Registry       string
	RegistrySecret string

	accessKey string
	secretKey string

	elastic *elastic.Client
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
		Zone:           os.Getenv("EXOSCALE_ZONE"),
		accessKey:      os.Getenv("EXOSCALE_ACCESS_KEY"),
		secretKey:      os.Getenv("EXOSCALE_SECRET_KEY"),
		Registry:       os.Getenv("REGISTRY"),
		RegistrySecret: os.Getenv("REGISTRY_SECRET"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeAwsServices(); err != nil {
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

func (p *Provider) initializeAwsServices() error {
	ec, err := elastic.New(os.Getenv("ELASTIC_URL"))
	if err != nil {
		return err
	}

	p.elastic = ec

	s, err := session.NewSession(&aws.Config{
		Region:                        aws.String(p.Zone),
		Endpoint:                      aws.String(fmt.Sprintf("https://sos-%s.exo.io", p.Zone)),
		S3DisableContentMD5Validation: aws.Bool(true),
		Credentials:                   credentials.NewStaticCredentials(p.accessKey, p.secretKey, ""),
	})
	if err != nil {
		return err
	}
	p.S3 = s3.New(s)

	return nil
}

func (p *Provider) awsConfig() *aws.Config {
	config := &aws.Config{}

	if os.Getenv("DEBUG") == "true" {
		config.WithLogLevel(aws.LogDebugWithHTTPBody)
	}

	config.MaxRetries = aws.Int(7)

	return config
}
