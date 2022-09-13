package aws_test

import (
	"testing"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/convox/convox/provider/aws/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
