package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s"
	"github.com/gobuffalo/packr"
)

type Provider struct {
	*k8s.Provider

	Bucket string
	Region string

	CloudFormation cloudformationiface.CloudFormationAPI
	CloudWatchLogs cloudwatchlogsiface.CloudWatchLogsAPI
	ECR            ecriface.ECRAPI
	S3             s3iface.S3API
	SQS            sqsiface.SQSAPI

	templater *templater.Templater
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider: k,
		Bucket:   os.Getenv("BUCKET"),
		Region:   os.Getenv("AWS_REGION"),
	}

	p.templater = templater.New(packr.NewBox("../aws/template"), p.templateHelpers())

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
	s, err := session.NewSession()
	if err != nil {
		return err
	}

	p.CloudFormation = cloudformation.New(s)
	p.CloudWatchLogs = cloudwatchlogs.New(s)
	p.ECR = ecr.New(s)
	p.S3 = s3.New(s)
	p.SQS = sqs.New(s)

	return nil
}
