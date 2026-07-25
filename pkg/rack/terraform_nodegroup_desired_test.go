package rack

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/stretchr/testify/assert"
)

type fakeEKS struct {
	eksiface.EKSAPI
	nodegroups []string
	scaling    map[string]*eks.NodegroupScalingConfig
	updates    []*eks.UpdateNodegroupConfigInput
	listErr    error
}

func (f *fakeEKS) ListNodegroups(*eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := &eks.ListNodegroupsOutput{}
	for _, n := range f.nodegroups {
		out.Nodegroups = append(out.Nodegroups, aws.String(n))
	}
	return out, nil
}

func (f *fakeEKS) DescribeNodegroup(in *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	sc, ok := f.scaling[aws.StringValue(in.NodegroupName)]
	if !ok {
		return nil, fmt.Errorf("no such node group")
	}
	return &eks.DescribeNodegroupOutput{Nodegroup: &eks.Nodegroup{ScalingConfig: sc}}, nil
}

func (f *fakeEKS) UpdateNodegroupConfig(in *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	f.updates = append(f.updates, in)
	return &eks.UpdateNodegroupConfigOutput{Update: &eks.Update{Id: aws.String("u-1"), Status: aws.String(eks.UpdateStatusSuccessful)}}, nil
}

func (f *fakeEKS) DescribeUpdate(in *eks.DescribeUpdateInput) (*eks.DescribeUpdateOutput, error) {
	return &eks.DescribeUpdateOutput{Update: &eks.Update{Id: in.UpdateId, Status: aws.String(eks.UpdateStatusSuccessful)}}, nil
}

func intp(i int) *int { return &i }

func scaling(desired, min, max int64) *eks.NodegroupScalingConfig {
	return &eks.NodegroupScalingConfig{
		DesiredSize: aws.Int64(desired),
		MinSize:     aws.Int64(min),
		MaxSize:     aws.Int64(max),
	}
}

func TestReconcileNodeGroupDesired(t *testing.T) {
	cases := []struct {
		name        string
		nodegroups  []string
		scaling     map[string]*eks.NodegroupScalingConfig
		groups      []nodeGroupDesired
		listErr     error
		wantNames   []string
		wantDesired int64
		wantMaxSet  bool
		wantMax     int64
	}{
		{
			name:       "no min set is a no-op",
			nodegroups: []string{"rack-additional-0-abcd"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-additional-0-abcd": scaling(1, 1, 5)},
			groups:     []nodeGroupDesired{{Id: intp(0), MaxSize: intp(9)}},
		},
		{
			name:       "group absent is a no-op",
			nodegroups: []string{"rack-additional-7-abcd"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-additional-7-abcd": scaling(1, 1, 5)},
			groups:     []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}},
		},
		{
			name:       "new min at or below desired is a no-op",
			nodegroups: []string{"rack-additional-0-abcd"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-additional-0-abcd": scaling(3, 1, 5)},
			groups:     []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}},
		},
		{
			name:        "raise desired within existing max",
			nodegroups:  []string{"rack-additional-101-abcd"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-101-abcd": scaling(1, 1, 5)},
			groups:      []nodeGroupDesired{{Id: intp(101), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-101-abcd"},
			wantDesired: 3,
		},
		{
			name:        "raise desired above existing max also widens max",
			nodegroups:  []string{"rack-additional-0-abcd"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-0-abcd": scaling(1, 1, 2)},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(8), MaxSize: intp(20)}},
			wantNames:   []string{"rack-additional-0-abcd"},
			wantDesired: 8,
			wantMaxSet:  true,
			wantMax:     8,
		},
		{
			name:        "nil id falls back to slice index",
			nodegroups:  []string{"rack-additional-0-abcd"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-0-abcd": scaling(1, 1, 5)},
			groups:      []nodeGroupDesired{{MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-abcd"},
			wantDesired: 3,
		},
		{
			name:        "id prefix does not collide with a longer id listed first",
			nodegroups:  []string{"rack-additional-11-bbbb", "rack-additional-1-aaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-11-bbbb": scaling(1, 1, 5), "rack-additional-1-aaaa": scaling(1, 1, 5)},
			groups:      []nodeGroupDesired{{Id: intp(1), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-1-aaaa"},
			wantDesired: 3,
		},
		{
			name:       "id does not match a longer id when its own group is absent",
			nodegroups: []string{"rack-additional-11-bbbb"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-additional-11-bbbb": scaling(1, 1, 5)},
			groups:     []nodeGroupDesired{{Id: intp(1), MinSize: intp(3)}},
		},
		{
			name:        "describe error on one group still reconciles the others",
			nodegroups:  []string{"rack-additional-0-xxxx", "rack-additional-1-yyyy"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-1-yyyy": scaling(1, 1, 5)},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}, {Id: intp(1), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-1-yyyy"},
			wantDesired: 3,
		},
		{
			name:        "multiple groups each get raised",
			nodegroups:  []string{"rack-additional-0-xxxx", "rack-additional-1-yyyy"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-additional-0-xxxx": scaling(1, 1, 5), "rack-additional-1-yyyy": scaling(1, 1, 5)},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)}, {Id: intp(1), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-xxxx", "rack-additional-1-yyyy"},
			wantDesired: 3,
		},
		{
			name:       "create before destroy leaves two groups sharing a prefix, both get raised",
			nodegroups: []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-additional-0-aaaa": scaling(1, 1, 5),
				"rack-additional-0-bbbb": scaling(1, 1, 5),
			},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			wantDesired: 3,
		},
		{
			name:       "a shared prefix on one id does not disturb another id",
			nodegroups: []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb", "rack-additional-1-cccc"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-additional-0-aaaa": scaling(1, 1, 5),
				"rack-additional-0-bbbb": scaling(1, 1, 5),
				"rack-additional-1-cccc": scaling(1, 1, 5),
			},
			groups: []nodeGroupDesired{
				{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)},
				{Id: intp(1), MinSize: intp(3), MaxSize: intp(9)},
			},
			wantNames:   []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb", "rack-additional-1-cccc"},
			wantDesired: 3,
		},
		{
			name:       "a build node group is not treated as a shared prefix match",
			nodegroups: []string{"rack-build-additional-0-aaaa", "rack-additional-0-bbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-build-additional-0-aaaa": scaling(1, 1, 5),
				"rack-additional-0-bbbb":       scaling(1, 1, 5),
			},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-bbbb"},
			wantDesired: 3,
		},
		{
			name:       "only the under provisioned side of a shared prefix is raised",
			nodegroups: []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-additional-0-aaaa": scaling(3, 1, 5),
				"rack-additional-0-bbbb": scaling(1, 1, 5),
			},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-bbbb"},
			wantDesired: 3,
		},
		{
			name:       "describe failure on one side of a shared prefix still raises the other",
			nodegroups: []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-additional-0-bbbb": scaling(1, 1, 5),
			},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(3), MaxSize: intp(9)}},
			wantNames:   []string{"rack-additional-0-bbbb"},
			wantDesired: 3,
		},
		{
			name:       "both sides of a shared prefix widen max independently",
			nodegroups: []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-additional-0-aaaa": scaling(1, 1, 2),
				"rack-additional-0-bbbb": scaling(1, 1, 2),
			},
			groups:      []nodeGroupDesired{{Id: intp(0), MinSize: intp(8), MaxSize: intp(20)}},
			wantNames:   []string{"rack-additional-0-aaaa", "rack-additional-0-bbbb"},
			wantDesired: 8,
			wantMaxSet:  true,
			wantMax:     8,
		},
		{
			name:    "list error is best-effort no-op",
			listErr: fmt.Errorf("throttled"),
			groups:  []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeEKS{nodegroups: tc.nodegroups, scaling: tc.scaling, listErr: tc.listErr}

			err := reconcileNodeGroupDesired(f, "rack", tc.groups)
			assert.NoError(t, err)
			assert.Len(t, f.updates, len(tc.wantNames))

			var gotNames []string
			for _, u := range f.updates {
				gotNames = append(gotNames, aws.StringValue(u.NodegroupName))
				assert.Nil(t, u.ScalingConfig.MinSize)
				if tc.wantDesired > 0 {
					assert.Equal(t, tc.wantDesired, aws.Int64Value(u.ScalingConfig.DesiredSize))
				}
			}
			assert.Equal(t, tc.wantNames, gotNames)

			for _, u := range f.updates {
				if tc.wantMaxSet {
					assert.Equal(t, tc.wantMax, aws.Int64Value(u.ScalingConfig.MaxSize), aws.StringValue(u.NodegroupName))
				} else {
					assert.Nil(t, u.ScalingConfig.MaxSize, aws.StringValue(u.NodegroupName))
				}
			}
		})
	}
}
