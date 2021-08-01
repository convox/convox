package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/convox/convox/pkg/common"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ecrHostMatcher = regexp.MustCompile(`(\d+)\.dkr\.ecr\.([^.]+)\.amazonaws\.com`)
)

func (p *Provider) appRegistry(app string) (string, error) {
	ns, err := p.Provider.Cluster.CoreV1().Namespaces().Get(context.Background(), p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		return "", err
	}

	as := ns.ObjectMeta.Annotations

	registry := common.CoalesceString(as["convox.com/registry"], as["convox.registry"])

	if registry == "" {
		return "", fmt.Errorf("no registry for app: %s", app)
	}

	return registry, nil
}

func (p *Provider) ecrAuth(host, access, secret string) (string, string, error) {
	if !ecrHostMatcher.MatchString(host) {
		return "", "", fmt.Errorf("invalid ecr registry: %s", host)
	}

	m := ecrHostMatcher.FindStringSubmatch(host)

	c := &aws.Config{
		Region: aws.String(m[2]),
	}

	if access != "" {
		c.Credentials = credentials.NewStaticCredentials(access, secret, "")
	}

	s, err := session.NewSession(c)
	if err != nil {
		return "", "", err
	}

	e := ecr.New(s)

	res, err := e.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{
		RegistryIds: []*string{aws.String(m[1])},
	})
	if err != nil {
		return "", "", fmt.Errorf("unable to authenticate with ecr registry: %s", host)
	}
	if len(res.AuthorizationData) != 1 {
		return "", "", fmt.Errorf("invalid authorization data from ecr registry: %s", host)
	}

	token, err := base64.StdEncoding.DecodeString(*res.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(string(token), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid authorization token from ecr registry: %s", host)
	}

	return parts[0], parts[1], nil
}

func awsErrorCode(err error) string {
	if ae, ok := err.(awserr.Error); ok {
		return ae.Code()
	}

	return ""
}
