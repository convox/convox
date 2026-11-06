package aws_test

import (
	"errors"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/convox/convox/pkg/atom"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRepositoryAuth(t *testing.T) {
	tests := []struct {
		Name      string
		Appname   string
		Namespace string
		Err       error
	}{
		{
			Name:      "Error Auth registry",
			Appname:   "app1",
			Namespace: "rack1-app1",
			Err:       errors.New("unable to authenticate with ecr registry: 134537970938.dkr.ecr.us-east-1.amazonaws.com/dev-remote/httpd"),
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

				host, _, err := p.RepositoryAuth(test.Appname)
				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, host, "AWS")
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestRepositoryImagesBatchDelete(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		ecrapi, ok := p.ECR.(*mocks.ECRAPI)
		require.True(t, ok)

		tags := make([]string, 150)
		for i := range tags {
			tags[i] = fmt.Sprintf("web.B%d", i)
		}

		firsts := []string{}
		sizes := []int{}
		ecrapi.On("BatchDeleteImage", mock.Anything).Return(&ecr.BatchDeleteImageOutput{
			Failures: []*ecr.ImageFailure{{FailureCode: awssdk.String(ecr.ImageFailureCodeImageNotFound)}},
		}, nil).Run(func(args mock.Arguments) {
			in, ok := args.Get(0).(*ecr.BatchDeleteImageInput)
			require.True(t, ok)
			assert.Equal(t, "rack1/app1", *in.RepositoryName)
			firsts = append(firsts, *in.ImageIds[0].ImageTag)
			sizes = append(sizes, len(in.ImageIds))
		})

		require.NoError(t, p.RepositoryImagesBatchDelete("app1", tags))
		assert.Equal(t, []int{100, 50}, sizes)
		assert.Equal(t, []string{"web.B0", "web.B100"}, firsts)
	})
}

func TestRepositoryImagesBatchDeleteError(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		ecrapi, ok := p.ECR.(*mocks.ECRAPI)
		require.True(t, ok)

		tags := make([]string, 150)
		for i := range tags {
			tags[i] = fmt.Sprintf("web.B%d", i)
		}

		ecrapi.On("BatchDeleteImage", mock.Anything).Return(nil, errors.New("denied")).Once()

		require.EqualError(t, p.RepositoryImagesBatchDelete("app1", tags), "denied")
		ecrapi.AssertNumberOfCalls(t, "BatchDeleteImage", 1)
	})
}
