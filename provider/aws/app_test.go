package aws_test

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/convox/convox/mock/aws"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAppCreate(t *testing.T) {
	tests := []struct {
		Name       string
		BucketName string
		AppName    string
		Namespace  string
		Output     *ecr.CreateRepositoryOutput
		Err        error
	}{
		{
			Name:       "Success",
			BucketName: "rack1/app1",
			AppName:    "app1",
			Namespace:  "rack1-app1",
			Output: &ecr.CreateRepositoryOutput{
				Repository: &ecr.Repository{
					RepositoryUri: awssdk.String("uribucket"),
				},
			},
			Err: nil,
		},
	}

	testProvider(t, func(p *aws.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				aa := p.Atom.(*atom.MockInterface)
				aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Twice()
				aa.On("Apply", test.Namespace, "app", "", mock.Anything, int32(30)).Return(nil).Once().Run(func(args mock.Arguments) {
					requireYamlFixture(t, args.Get(3).([]byte), "app.yml")
				})

				ecrapi := p.ECR.(*mocks.ECRAPI)
				ecrapi.On("CreateRepository", &ecr.CreateRepositoryInput{
					RepositoryName: awssdk.String(test.BucketName),
				}).Return(test.Output, test.Err)

				a, err := p.AppCreate(test.AppName, structs.AppCreateOptions{})
				if err == nil {
					require.NoError(t, err)
					require.NotNil(t, a)

					assert.Equal(t, "3", a.Generation)
					assert.Equal(t, test.AppName, a.Name)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestAppDelete(t *testing.T) {
	tests := []struct {
		Name       string
		BucketName string
		AppName    string
		Namespace  string
		Err        error
	}{
		{
			Name:       "Success",
			BucketName: "rack1/app1",
			AppName:    "app1",
			Namespace:  "rack1-app1",
			Err:        nil,
		},
		{
			Name:       "Success",
			BucketName: "rack1/app2",
			AppName:    "app2",
			Namespace:  "rack1-app2",
			Err:        errors.New("error on delete app repository"),
		},
	}

	testProvider(t, func(p *aws.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Provider.Cluster.(*fake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Once()
				}

				ecrapi := p.ECR.(*mocks.ECRAPI)
				ecrapi.On("DeleteRepository", &ecr.DeleteRepositoryInput{
					Force:          awssdk.Bool(true),
					RepositoryName: awssdk.String(test.BucketName),
				}).Return(nil, test.Err)

				err := p.AppDelete(test.AppName)
				if err == nil {
					require.NoError(t, err)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func appCreate(c kubernetes.Interface, rack, name string) error {
	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(),
		&ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", rack, name),
				Annotations: map[string]string{
					"convox.com/registry": "134537970938.dkr.ecr.us-east-1.amazonaws.com/dev-remote/httpd",
					"convox.com/lock":     "false",
				},
				Labels: map[string]string{
					"app":    name,
					"name":   name,
					"rack":   rack,
					"system": "convox",
					"type":   "app",
				},
			},
		},
		am.CreateOptions{},
	)

	return errors.WithStack(err)
}
