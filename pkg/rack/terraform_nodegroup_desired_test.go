package rack

import (
	"encoding/base64"
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
		name              string
		nodegroups        []string
		scaling           map[string]*eks.NodegroupScalingConfig
		groups            []nodeGroupDesired
		listErr           error
		wantNames         []string
		wantDesired       int64
		wantMaxSet        bool
		wantMax           int64
		buildMin          int64
		onDemandMin       int64
		wantDesiredByName map[string]int64
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
		{
			name:        "build node group below minimum is raised",
			nodegroups:  []string{"rack-build-us-east-1a-0aaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-build-us-east-1a-0aaaa": scaling(0, 0, 100)},
			buildMin:    1,
			wantNames:   []string{"rack-build-us-east-1a-0aaaa"},
			wantDesired: 1,
		},
		{
			name:       "build node group at minimum is left alone",
			nodegroups: []string{"rack-build-us-east-1a-0aaaa"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-build-us-east-1a-0aaaa": scaling(1, 0, 100)},
			buildMin:   1,
		},
		{
			name:        "build node group under a karpenter max of one is raised without widening",
			nodegroups:  []string{"rack-build-us-east-1a-0aaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-build-us-east-1a-0aaaa": scaling(0, 0, 1)},
			buildMin:    1,
			wantNames:   []string{"rack-build-us-east-1a-0aaaa"},
			wantDesired: 1,
		},
		{
			name:        "build node group under a karpenter max of one widens max when the minimum exceeds it",
			nodegroups:  []string{"rack-build-us-east-1a-0aaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-build-us-east-1a-0aaaa": scaling(0, 0, 1)},
			buildMin:    2,
			wantNames:   []string{"rack-build-us-east-1a-0aaaa"},
			wantDesired: 2,
			wantMaxSet:  true,
			wantMax:     2,
		},
		{
			name:       "additional build group below the build minimum is not raised",
			nodegroups: []string{"rack-build-additional-0-bbbb", "rack-build-us-east-1a-0aaaa"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-build-additional-0-bbbb": scaling(0, 0, 5),
				"rack-build-us-east-1a-0aaaa":  scaling(0, 0, 100),
			},
			buildMin:    1,
			wantNames:   []string{"rack-build-us-east-1a-0aaaa"},
			wantDesired: 1,
		},
		{
			name:       "two build node groups sharing the prefix are both raised",
			nodegroups: []string{"rack-build-us-east-1a-0aaaa", "rack-build-us-east-1a-0cccc"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-build-us-east-1a-0aaaa": scaling(0, 0, 100),
				"rack-build-us-east-1a-0cccc": scaling(0, 0, 100),
			},
			buildMin:    1,
			wantNames:   []string{"rack-build-us-east-1a-0aaaa", "rack-build-us-east-1a-0cccc"},
			wantDesired: 1,
		},
		{
			name:       "build and additional node groups are raised in one pass",
			nodegroups: []string{"rack-build-us-east-1a-0aaaa", "rack-additional-0-dddd"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-build-us-east-1a-0aaaa": scaling(0, 0, 100),
				"rack-additional-0-dddd":      scaling(1, 1, 5),
			},
			groups:            []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}},
			buildMin:          1,
			wantNames:         []string{"rack-additional-0-dddd", "rack-build-us-east-1a-0aaaa"},
			wantDesiredByName: map[string]int64{"rack-additional-0-dddd": 3, "rack-build-us-east-1a-0aaaa": 1},
		},
		{
			name:       "no build node group is a no-op",
			nodegroups: []string{"rack-us-east-1a-0eeee"},
			scaling:    map[string]*eks.NodegroupScalingConfig{"rack-us-east-1a-0eeee": scaling(1, 1, 5)},
			buildMin:   2,
		},
		{
			name:        "on-demand node group below minimum is raised",
			nodegroups:  []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-us-east-1a-0aaaaaaaaaaaaaaaa": scaling(1, 1, 100)},
			onDemandMin: 2,
			wantNames:   []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			wantDesired: 2,
		},
		{
			name: "only the first system node group is the on-demand one",
			nodegroups: []string{
				"rack-us-east-1a-0aaaaaaaaaaaaaaaa",
				"rack-us-east-1b-1aaaaaaaaaaaaaaaa",
				"rack-us-east-1c-2aaaaaaaaaaaaaaaa",
				"rack-build-us-east-1a-0cccccccccccccccc",
				"rack-additional-0-dddddddddddddddd",
			},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-us-east-1a-0aaaaaaaaaaaaaaaa":       scaling(1, 1, 100),
				"rack-us-east-1b-1aaaaaaaaaaaaaaaa":       scaling(1, 1, 100),
				"rack-us-east-1c-2aaaaaaaaaaaaaaaa":       scaling(1, 1, 100),
				"rack-build-us-east-1a-0cccccccccccccccc": scaling(0, 0, 100),
				"rack-additional-0-dddddddddddddddd":      scaling(1, 1, 5),
			},
			onDemandMin: 2,
			wantNames:   []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			wantDesired: 2,
		},
		{
			name:        "on-demand node group at minimum is left alone",
			nodegroups:  []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-us-east-1a-0aaaaaaaaaaaaaaaa": scaling(3, 1, 100)},
			onDemandMin: 2,
		},
		{
			name:        "on-demand node group widens max when the minimum exceeds it",
			nodegroups:  []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			scaling:     map[string]*eks.NodegroupScalingConfig{"rack-us-east-1a-0aaaaaaaaaaaaaaaa": scaling(1, 1, 2)},
			onDemandMin: 5,
			wantNames:   []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			wantDesired: 5,
			wantMaxSet:  true,
			wantMax:     5,
		},
		{
			name:       "two on-demand node groups sharing the index are both raised",
			nodegroups: []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa", "rack-us-east-1a-0bbbbbbbbbbbbbbbb"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-us-east-1a-0aaaaaaaaaaaaaaaa": scaling(1, 1, 100),
				"rack-us-east-1a-0bbbbbbbbbbbbbbbb": scaling(1, 1, 100),
			},
			onDemandMin: 2,
			wantNames:   []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa", "rack-us-east-1a-0bbbbbbbbbbbbbbbb"},
			wantDesired: 2,
		},
		{
			name:       "build and on-demand node groups are raised in one pass",
			nodegroups: []string{"rack-us-east-1a-0aaaaaaaaaaaaaaaa", "rack-build-us-east-1a-0cccccccccccccccc"},
			scaling: map[string]*eks.NodegroupScalingConfig{
				"rack-us-east-1a-0aaaaaaaaaaaaaaaa":       scaling(1, 1, 100),
				"rack-build-us-east-1a-0cccccccccccccccc": scaling(0, 0, 100),
			},
			buildMin:          1,
			onDemandMin:       2,
			wantNames:         []string{"rack-build-us-east-1a-0cccccccccccccccc", "rack-us-east-1a-0aaaaaaaaaaaaaaaa"},
			wantDesiredByName: map[string]int64{"rack-build-us-east-1a-0cccccccccccccccc": 1, "rack-us-east-1a-0aaaaaaaaaaaaaaaa": 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeEKS{nodegroups: tc.nodegroups, scaling: tc.scaling, listErr: tc.listErr}

			err := reconcileNodeGroupDesired(f, nodeGroupTargets{cluster: "rack", groups: tc.groups, buildMin: tc.buildMin, onDemandMin: tc.onDemandMin})
			assert.NoError(t, err)
			assert.Len(t, f.updates, len(tc.wantNames))

			var gotNames []string
			for _, u := range f.updates {
				gotNames = append(gotNames, aws.StringValue(u.NodegroupName))
				assert.Nil(t, u.ScalingConfig.MinSize)
				if want, ok := tc.wantDesiredByName[aws.StringValue(u.NodegroupName)]; ok {
					assert.Equal(t, want, aws.Int64Value(u.ScalingConfig.DesiredSize), aws.StringValue(u.NodegroupName))
				} else if tc.wantDesired > 0 {
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

func TestTargetsFromVars(t *testing.T) {
	twoGroupsB64 := base64.StdEncoding.EncodeToString([]byte(`[{"id":0,"min_size":1},{"id":1,"min_size":2}]`))
	twoGroups := []nodeGroupDesired{{Id: intp(0), MinSize: intp(1)}, {Id: intp(1), MinSize: intp(2)}}

	cases := []struct {
		name         string
		vars         map[string]string
		wantCluster  string
		wantGroups   []nodeGroupDesired
		wantBuildMin int64
		wantOnDemand int64
	}{
		{name: "name is lowercased", vars: map[string]string{"name": "Rack"}, wantCluster: "rack"},
		{name: "name falls back to the rack name", vars: map[string]string{}, wantCluster: "fallback"},
		{name: "build enabled with minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "1"}, wantCluster: "rack", wantBuildMin: 1},
		{name: "karpenter true gates", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "2", "karpenter_enabled": "true"}, wantCluster: "rack"},
		{name: "karpenter True does not gate", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "2", "karpenter_enabled": "True"}, wantCluster: "rack", wantBuildMin: 2},
		{name: "karpenter false does not gate", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "2", "karpenter_enabled": "false"}, wantCluster: "rack", wantBuildMin: 2},
		{name: "no build_node_enabled", vars: map[string]string{"name": "rack", "build_node_min_count": "1"}, wantCluster: "rack"},
		{name: "build_node_enabled false", vars: map[string]string{"name": "rack", "build_node_enabled": "false", "build_node_min_count": "1"}, wantCluster: "rack"},
		{name: "build_node_enabled 1", vars: map[string]string{"name": "rack", "build_node_enabled": "1", "build_node_min_count": "1"}, wantCluster: "rack", wantBuildMin: 1},
		{name: "blank minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": ""}, wantCluster: "rack"},
		{name: "abc minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "abc"}, wantCluster: "rack"},
		{name: "zero minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "0"}, wantCluster: "rack"},
		{name: "negative minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "-1"}, wantCluster: "rack"},
		{name: "101 minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "101"}, wantCluster: "rack"},
		{name: "100 minimum", vars: map[string]string{"name": "rack", "build_node_enabled": "true", "build_node_min_count": "100"}, wantCluster: "rack", wantBuildMin: 100},
		{name: "type error in additional config yields no groups", vars: map[string]string{"name": "rack", "additional_node_groups_config": `[{"id":"x","min_size":3}]`, "build_node_enabled": "true", "build_node_min_count": "1"}, wantCluster: "rack", wantBuildMin: 1},
		{name: "malformed additional config does not suppress build", vars: map[string]string{"name": "rack", "additional_node_groups_config": "not json", "build_node_enabled": "true", "build_node_min_count": "1"}, wantCluster: "rack", wantBuildMin: 1},
		{name: "raw json additional config", vars: map[string]string{"name": "rack", "additional_node_groups_config": `[{"id":0,"min_size":3}]`}, wantCluster: "rack", wantGroups: []nodeGroupDesired{{Id: intp(0), MinSize: intp(3)}}},
		{name: "empty list raw", vars: map[string]string{"name": "rack", "additional_node_groups_config": "[]"}, wantCluster: "rack"},
		{name: "empty list base64", vars: map[string]string{"name": "rack", "additional_node_groups_config": "W10="}, wantCluster: "rack"},
		{name: "base64 two groups", vars: map[string]string{"name": "rack", "additional_node_groups_config": twoGroupsB64}, wantCluster: "rack", wantGroups: twoGroups},
		{name: "base64 two groups with karpenter gating build", vars: map[string]string{"name": "rack", "additional_node_groups_config": twoGroupsB64, "build_node_enabled": "true", "build_node_min_count": "2", "karpenter_enabled": "true"}, wantCluster: "rack", wantGroups: twoGroups},
		{name: "mixed capacity with on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "2"}, wantCluster: "rack", wantOnDemand: 2},
		{name: "mixed capacity is matched regardless of case", vars: map[string]string{"name": "rack", "node_capacity_type": "MIXED", "min_on_demand_count": "2"}, wantCluster: "rack", wantOnDemand: 2},
		{name: "on_demand capacity ignores the on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "on_demand", "min_on_demand_count": "2"}, wantCluster: "rack"},
		{name: "no node_capacity_type ignores the on-demand minimum", vars: map[string]string{"name": "rack", "min_on_demand_count": "2"}, wantCluster: "rack"},
		{name: "karpenter gates the on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "2", "karpenter_enabled": "true"}, wantCluster: "rack"},
		{name: "on-demand minimum above the default maximum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "150"}, wantCluster: "rack"},
		{name: "on-demand minimum within a raised maximum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "150", "max_on_demand_count": "200"}, wantCluster: "rack", wantOnDemand: 150},
		{name: "on-demand minimum at the default maximum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "100"}, wantCluster: "rack", wantOnDemand: 100},
		{name: "on-demand minimum above a lowered maximum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "5", "max_on_demand_count": "3"}, wantCluster: "rack"},
		{name: "on-demand minimum at a lowered maximum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "3", "max_on_demand_count": "3"}, wantCluster: "rack", wantOnDemand: 3},
		{name: "unparsable on-demand maximum falls back to the default", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "2", "max_on_demand_count": "abc"}, wantCluster: "rack", wantOnDemand: 2},
		{name: "zero on-demand maximum gates the minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "2", "max_on_demand_count": "0"}, wantCluster: "rack"},
		{name: "blank on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": ""}, wantCluster: "rack"},
		{name: "abc on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "abc"}, wantCluster: "rack"},
		{name: "zero on-demand minimum", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "0"}, wantCluster: "rack"},
		{name: "build and on-demand minimums together", vars: map[string]string{"name": "rack", "node_capacity_type": "mixed", "min_on_demand_count": "2", "build_node_enabled": "true", "build_node_min_count": "1"}, wantCluster: "rack", wantBuildMin: 1, wantOnDemand: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := targetsFromVars(tc.vars, "fallback")
			assert.Equal(t, tc.wantCluster, got.cluster)
			if tc.wantGroups == nil {
				assert.Empty(t, got.groups)
			} else {
				assert.Equal(t, tc.wantGroups, got.groups)
			}
			assert.Equal(t, tc.wantBuildMin, got.buildMin)
			assert.Equal(t, tc.wantOnDemand, got.onDemandMin)
		})
	}
}
