package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
)

type Provider struct {
	*k8s.Provider

	Bucket        string
	EncryptionKey string
	Region        string

	Ec2 *ec2.EC2

	CloudFormation cloudformationiface.CloudFormationAPI
	CloudWatchLogs cloudwatchlogsiface.CloudWatchLogsAPI
	ECR            ecriface.ECRAPI
	EKS            eksiface.EKSAPI
	IAM            iamiface.IAMAPI
	S3             s3iface.S3API
	SQS            sqsiface.SQSAPI
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider:      k,
		Bucket:        os.Getenv("BUCKET"),
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
		Region:        os.Getenv("AWS_REGION"),
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
	s, err := session.NewSession()
	if err != nil {
		return err
	}

	p.Ec2 = ec2.New(s, p.config())

	p.CloudFormation = cloudformation.New(s)
	p.CloudWatchLogs = cloudwatchlogs.New(s)
	p.ECR = ecr.New(s)
	p.IAM = iam.New(s)
	p.EKS = eks.New(s)
	p.S3 = s3.New(s)
	p.SQS = sqs.New(s)

	return nil
}

func (p *Provider) config() *aws.Config {
	config := &aws.Config{
		Region: aws.String(p.Region),
	}

	if os.Getenv("DEBUG") == "true" {
		config.WithLogLevel(aws.LogDebugWithHTTPBody)
	}

	config.MaxRetries = aws.Int(7)

	return config
}
