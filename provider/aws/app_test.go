package aws_test

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/convox/convox/pkg/atom"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
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
				// AppCreate now invokes p.Atom.Status three times: (1) inside
				// ReleasePromote → AppGet → appFromNamespace, (2) the new
				// release-watcher supersession-detection capture in
				// ReleasePromote, (3) the final AppGet → appFromNamespace at
				// the end of AppCreate.
				aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Times(3)
				aa.On("Apply", test.Namespace, "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
					cfg := args.Get(2).(*atom.ApplyConfig)
					requireYamlFixture(t, cfg.Template, "app.yml")
				})

				ecrapi := p.ECR.(*mocks.ECRAPI)
				ecrapi.On("CreateRepository", &ecr.CreateRepositoryInput{
					RepositoryName: awssdk.String(test.BucketName),
					ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
						ScanOnPush: awssdk.Bool(false),
					},
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

func TestAppCreateImmutableTags(t *testing.T) {
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
		p.EcrImmutableTagsEnabled = true

		for _, test := range tests {
			fn := func(t *testing.T) {
				aa, ok := p.Atom.(*atom.MockInterface)
				require.True(t, ok)
				aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Times(3)
				aa.On("Apply", test.Namespace, "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
					cfg, ok := args.Get(2).(*atom.ApplyConfig)
					require.True(t, ok)
					requireYamlFixture(t, cfg.Template, "app.yml")
				})

				ecrapi, ok := p.ECR.(*mocks.ECRAPI)
				require.True(t, ok)
				ecrapi.On("CreateRepository", &ecr.CreateRepositoryInput{
					RepositoryName: awssdk.String(test.BucketName),
					ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
						ScanOnPush: awssdk.Bool(false),
					},
					ImageTagMutability: awssdk.String(ecr.ImageTagMutabilityImmutable),
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
		AppName    string
		Create     bool
		Locked     bool
		RepoErr    error
		DeleteRepo bool
		Statuses   int
		Lists      int
		Err        string
		Kept       bool
	}{
		{Name: "unlocked", AppName: "app1", Create: true, DeleteRepo: true, Statuses: 2, Lists: 1},
		{Name: "ecr-error", AppName: "app2", Create: true, RepoErr: errors.New("error on delete app repository"), DeleteRepo: true, Statuses: 1, Lists: 1, Err: "error on delete app repository", Kept: true},
		{Name: "locked", AppName: "app3", Create: true, Locked: true, Statuses: 1, Err: "app is locked: app3", Kept: true},
		{Name: "missing", AppName: "app4", Err: "app not found: app4"},
	}

	testProvider(t, func(p *aws.Provider) {
		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)

		for _, test := range tests {
			t.Run(test.Name, func(t *testing.T) {
				t.Setenv("TEST", "")

				ns := "rack1-" + test.AppName
				eksapi := &appDeleteEKS{}
				p.EKS = eksapi
				ecrapi := &mocks.ECRAPI{}
				p.ECR = ecrapi

				if test.Create {
					require.NoError(t, appCreate(kk, "rack1", test.AppName))
				}

				if test.Locked {
					obj, err := kk.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
					require.NoError(t, err)
					obj.Annotations["convox.com/lock"] = "true"
					_, err = kk.CoreV1().Namespaces().Update(context.TODO(), obj, am.UpdateOptions{})
					require.NoError(t, err)
				}

				if test.Statuses > 0 {
					aa.On("Status", ns, "app").Return("Running", "R1234567", nil).Times(test.Statuses)
				}

				if test.DeleteRepo {
					ecrapi.On("DeleteRepository", &ecr.DeleteRepositoryInput{
						Force:          awssdk.Bool(true),
						RepositoryName: awssdk.String("rack1/" + test.AppName),
					}).Return(nil, test.RepoErr).Once()
				}

				err := p.AppDelete(test.AppName)
				if test.Err == "" {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, test.Err)
				}

				require.Equal(t, test.Lists, eksapi.lists)
				ecrapi.AssertExpectations(t)

				_, err = kk.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
				if test.Kept {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, fmt.Sprintf("namespaces %q not found", ns))
				}
			})
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

type appDeleteEKS struct {
	eksiface.EKSAPI
	lists int
}

func (f *appDeleteEKS) ListPodIdentityAssociations(*eks.ListPodIdentityAssociationsInput) (*eks.ListPodIdentityAssociationsOutput, error) {
	f.lists++
	return &eks.ListPodIdentityAssociationsOutput{}, nil
}
