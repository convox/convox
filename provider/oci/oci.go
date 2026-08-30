package oci

import (
	"context"
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

	Bucket           string
	Region           string
	Registry         string
	RegistryPassword string
	RegistryUsername string
	S3Access         string
	S3Endpoint       string
	S3Secret         string

	elastic *elastic.Client
	s3      s3iface.S3API
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider:         k,
		Bucket:           os.Getenv("BUCKET"),
		Region:           os.Getenv("REGION"),
		Registry:         os.Getenv("REGISTRY"),
		RegistryPassword: os.Getenv("REGISTRY_PASSWORD"),
		RegistryUsername: os.Getenv("REGISTRY_USERNAME"),
		S3Access:         os.Getenv("S3_ACCESS"),
		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		S3Secret:         os.Getenv("S3_SECRET"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeOciServices(); err != nil {
		return err
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	if kp, ok := pp.Provider.WithContext(ctx).(*k8s.Provider); ok {
		pp.Provider = kp
	}
	return &pp
}

func (p *Provider) initializeOciServices() error {
	ec, err := elastic.New(os.Getenv("ELASTIC_URL"))
	if err != nil {
		return err
	}

	p.elastic = ec

	s, err := session.NewSession(&aws.Config{
		Region:      aws.String(p.Region),
		Credentials: credentials.NewStaticCredentials(p.S3Access, p.S3Secret, ""),
	})
	if err != nil {
		return err
	}

	// OCI Object Storage speaks the S3 API only at a per-namespace endpoint and
	// only with path-style addressing.
	p.s3 = s3.New(s, &aws.Config{
		Endpoint:         aws.String(p.S3Endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})

	return nil
}
