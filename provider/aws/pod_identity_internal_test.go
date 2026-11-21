package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type fakePodIdentityEKS struct {
	eksiface.EKSAPI
	associations map[string]*eks.PodIdentityAssociation
	createErr    error
	describeErr  error
	lists        []string
	creates      []string
	describes    []string
}

func (f *fakePodIdentityEKS) ListPodIdentityAssociations(in *eks.ListPodIdentityAssociationsInput) (*eks.ListPodIdentityAssociationsOutput, error) {
	key := aws.StringValue(in.Namespace) + "/" + aws.StringValue(in.ServiceAccount)
	f.lists = append(f.lists, key)

	out := &eks.ListPodIdentityAssociationsOutput{}
	if a, ok := f.associations[key]; ok {
		out.Associations = []*eks.PodIdentityAssociationSummary{{AssociationId: a.AssociationId, ClusterName: in.ClusterName}}
	}

	return out, nil
}

func (f *fakePodIdentityEKS) CreatePodIdentityAssociation(in *eks.CreatePodIdentityAssociationInput) (*eks.CreatePodIdentityAssociationOutput, error) {
	key := aws.StringValue(in.Namespace) + "/" + aws.StringValue(in.ServiceAccount)
	f.creates = append(f.creates, key)

	if f.createErr != nil {
		return nil, f.createErr
	}

	a := &eks.PodIdentityAssociation{AssociationId: aws.String("a-" + key), CreatedAt: aws.Time(time.Now())}
	if f.associations == nil {
		f.associations = map[string]*eks.PodIdentityAssociation{}
	}
	f.associations[key] = a

	return &eks.CreatePodIdentityAssociationOutput{Association: a}, nil
}

func (f *fakePodIdentityEKS) DescribePodIdentityAssociation(in *eks.DescribePodIdentityAssociationInput) (*eks.DescribePodIdentityAssociationOutput, error) {
	f.describes = append(f.describes, aws.StringValue(in.AssociationId))

	if f.describeErr != nil {
		return nil, f.describeErr
	}

	for _, a := range f.associations {
		if aws.StringValue(a.AssociationId) == aws.StringValue(in.AssociationId) {
			return &eks.DescribePodIdentityAssociationOutput{Association: a}, nil
		}
	}

	return nil, awserr.New(eks.ErrCodeResourceNotFoundException, "association not found", nil)
}

type fakePodIdentityIAM struct {
	iamiface.IAMAPI
	attached map[string][]string
}

func (f *fakePodIdentityIAM) GetRole(in *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
	if _, ok := f.attached[aws.StringValue(in.RoleName)]; !ok {
		return nil, awserr.New(iam.ErrCodeNoSuchEntityException, "role not found", nil)
	}

	return &iam.GetRoleOutput{Role: &iam.Role{Arn: aws.String("arn:aws:iam::123456789012:role/convox/" + aws.StringValue(in.RoleName))}}, nil
}

func (f *fakePodIdentityIAM) CreateRole(in *iam.CreateRoleInput) (*iam.CreateRoleOutput, error) {
	if f.attached == nil {
		f.attached = map[string][]string{}
	}
	f.attached[aws.StringValue(in.RoleName)] = nil

	return &iam.CreateRoleOutput{Role: &iam.Role{Arn: aws.String("arn:aws:iam::123456789012:role/convox/" + aws.StringValue(in.RoleName))}}, nil
}

func (f *fakePodIdentityIAM) ListAttachedRolePolicies(in *iam.ListAttachedRolePoliciesInput) (*iam.ListAttachedRolePoliciesOutput, error) {
	out := &iam.ListAttachedRolePoliciesOutput{}
	for _, arn := range f.attached[aws.StringValue(in.RoleName)] {
		out.AttachedPolicies = append(out.AttachedPolicies, &iam.AttachedPolicy{PolicyArn: aws.String(arn)})
	}

	return out, nil
}

func (f *fakePodIdentityIAM) AttachRolePolicy(in *iam.AttachRolePolicyInput) (*iam.AttachRolePolicyOutput, error) {
	role := aws.StringValue(in.RoleName)
	f.attached[role] = append(f.attached[role], aws.StringValue(in.PolicyArn))

	return &iam.AttachRolePolicyOutput{}, nil
}

const podIdentityManifest = `services:
  web:
    image: docker.io/library/nginx
    accessControl:
      awsPodIdentity:
        policyArns:
          - arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess
  worker:
    image: docker.io/library/nginx
timers:
  cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: web
  report:
    schedule: "0 4 * * *"
    command: bin/report
    service: worker
`

func podIdentityProvider(eksFake *fakePodIdentityEKS) *Provider {
	return &Provider{Provider: &k8s.Provider{Name: "rack"}, EKS: eksFake, IAM: &fakePodIdentityIAM{}}
}

func podIdentityTestManifest(t *testing.T) *manifest.Manifest {
	t.Helper()

	m, err := manifest.Load([]byte(podIdentityManifest), map[string]string{})
	require.NoError(t, err)

	return m
}

func oldPodIdentityAssociation(id string, created *time.Time) *eks.PodIdentityAssociation {
	return &eks.PodIdentityAssociation{AssociationId: aws.String(id), CreatedAt: created}
}

func TestSyncPodIdentityNewAssociations(t *testing.T) {
	eksFake := &fakePodIdentityEKS{}
	p := podIdentityProvider(eksFake)

	start := time.Now()
	created, err := p.SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.NoError(t, err)

	require.Less(t, time.Since(start), 5*time.Second)
	require.WithinDuration(t, time.Now(), created, 5*time.Second)
	require.Equal(t, []string{"rack-app1/web", "rack-app1/timer-cleanup"}, eksFake.lists)
	require.Equal(t, []string{"rack-app1/web", "rack-app1/timer-cleanup"}, eksFake.creates)
	require.Empty(t, eksFake.describes)

	iamFake, ok := p.IAM.(*fakePodIdentityIAM)
	require.True(t, ok)
	require.Equal(t, []string{"arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"}, iamFake.attached[p.getRoleName("app1", "web")])
	require.Equal(t, []string{"arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"}, iamFake.attached[p.getRoleName("app1", "timer-cleanup")])
}

func TestSyncPodIdentityExistingAssociations(t *testing.T) {
	older := time.Now().Add(-time.Hour)
	newer := time.Now().Add(-30 * time.Second)

	eksFake := &fakePodIdentityEKS{associations: map[string]*eks.PodIdentityAssociation{
		"rack-app1/web":           oldPodIdentityAssociation("a-web", &older),
		"rack-app1/timer-cleanup": oldPodIdentityAssociation("a-timer", &newer),
	}}

	created, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.NoError(t, err)

	require.Equal(t, newer, created)
	require.Empty(t, eksFake.creates)
	require.Equal(t, []string{"a-web", "a-timer"}, eksFake.describes)
}

func TestSyncPodIdentityMixedAssociations(t *testing.T) {
	older := time.Now().Add(-time.Hour)

	eksFake := &fakePodIdentityEKS{associations: map[string]*eks.PodIdentityAssociation{
		"rack-app1/timer-cleanup": oldPodIdentityAssociation("a-timer", &older),
	}}

	created, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.NoError(t, err)

	require.WithinDuration(t, time.Now(), created, 5*time.Second)
	require.Equal(t, []string{"rack-app1/web"}, eksFake.creates)
	require.Equal(t, []string{"a-timer"}, eksFake.describes)
}

func TestSyncPodIdentityMissingCreatedAt(t *testing.T) {
	eksFake := &fakePodIdentityEKS{associations: map[string]*eks.PodIdentityAssociation{
		"rack-app1/web":           oldPodIdentityAssociation("a-web", nil),
		"rack-app1/timer-cleanup": oldPodIdentityAssociation("a-timer", nil),
	}}

	created, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), created, 5*time.Second)
}

func TestSyncPodIdentityNone(t *testing.T) {
	eksFake := &fakePodIdentityEKS{}

	m, err := manifest.Load([]byte("services:\n  web:\n    image: docker.io/library/nginx\n"), map[string]string{})
	require.NoError(t, err)

	created, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", m)
	require.NoError(t, err)

	require.True(t, created.IsZero())
	require.Empty(t, eksFake.lists)
}

func TestSyncPodIdentityCreateError(t *testing.T) {
	eksFake := &fakePodIdentityEKS{createErr: errors.New("denied")}

	created, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.EqualError(t, err, "failed to create pod identity association: denied")
	require.True(t, created.IsZero())
}

func TestSyncPodIdentityPolicyArn(t *testing.T) {
	tests := []struct {
		Arn   string
		Valid bool
	}{
		{Arn: "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess", Valid: true},
		{Arn: "arn:aws-us-gov:iam::123456789012:policy/team/reports", Valid: true},
		{Arn: "arn:aws:iam::123456789012:role/reports"},
		{Arn: "arn:aws:s3:::reports"},
		{Arn: "AmazonS3ReadOnlyAccess"},
		{Arn: ""},
	}

	for _, test := range tests {
		t.Run(test.Arn, func(t *testing.T) {
			eksFake := &fakePodIdentityEKS{}
			p := podIdentityProvider(eksFake)

			m := podIdentityTestManifest(t)
			require.Equal(t, "worker", m.Services[1].Name)
			m.Services[1].AccessControl.AWSPodIdentity = &manifest.AWSPodIdentityOptions{PolicyArns: []string{"arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess", test.Arn}}

			_, err := p.SyncPodIdentity("app1", m)
			if test.Valid {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, fmt.Sprintf("service worker: accessControl.awsPodIdentity.policyArns: %q is not an IAM policy ARN", test.Arn))
			require.Empty(t, eksFake.lists)

			iamFake, ok := p.IAM.(*fakePodIdentityIAM)
			require.True(t, ok)
			require.Empty(t, iamFake.attached)
		})
	}
}

func TestSyncPodIdentityDescribeError(t *testing.T) {
	older := time.Now().Add(-time.Hour)

	eksFake := &fakePodIdentityEKS{
		associations: map[string]*eks.PodIdentityAssociation{"rack-app1/web": oldPodIdentityAssociation("a-web", &older)},
		describeErr:  errors.New("throttled"),
	}

	_, err := podIdentityProvider(eksFake).SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.EqualError(t, err, "failed to describe pod identity association: throttled")
}

func TestSyncPodIdentityTenantNamespace(t *testing.T) {
	eksFake := &fakePodIdentityEKS{}

	sp := podIdentityProvider(eksFake).WithContext(context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, "t1"))
	p, ok := sp.(*Provider)
	require.True(t, ok)

	engine, ok := p.Engine.(k8s.PodIdentityEngine)
	require.True(t, ok)

	_, err := engine.SyncPodIdentity("app1", podIdentityTestManifest(t))
	require.NoError(t, err)
	require.Equal(t, []string{"rack-t1-app1/web", "rack-t1-app1/timer-cleanup"}, eksFake.creates)
}

func TestReleasePromotePodIdentityDependency(t *testing.T) {
	app := appNamespace("app1", "", "")
	app.Annotations["convox.com/registry"] = "registry.example/app1"

	objs := []runtime.Object{app, &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack-system"}}}
	cobjs := []runtime.Object{appRelease("rack-app1", "release1", podIdentityManifest)}

	p := retentionProvider(t, &mocks.CloudWatchLogsAPI{}, 0, objs, cobjs)

	eksFake := &fakePodIdentityEKS{}
	p.EKS = eksFake
	p.IAM = &fakePodIdentityIAM{}

	a, ok := p.Atom.(*atom.MockInterface)
	require.True(t, ok)

	var deps []string
	a.On("Apply", "rack-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
		cfg, ok := args.Get(2).(*atom.ApplyConfig)
		require.True(t, ok)
		deps = cfg.Dependencies
	})

	require.NoError(t, p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{}))

	require.Len(t, deps, 1)
	require.Contains(t, deps[0], "|app:app1|type:podidentity-wait|")
	require.Equal(t, []string{"rack-app1/web", "rack-app1/timer-cleanup"}, eksFake.lists)
	a.AssertExpectations(t)
}
