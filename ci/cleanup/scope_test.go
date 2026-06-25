package cleanup

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

func tag(k, v string) ec2types.Tag {
	return ec2types.Tag{Key: aws.String(k), Value: aws.String(v)}
}

func TestIsCITag(t *testing.T) {
	cases := []struct {
		name string
		tags []ec2types.Tag
		want bool
	}{
		{"rack ci", []ec2types.Tag{tag("Rack", "ci-20260618")}, true},
		{"k8s cluster ci", []ec2types.Tag{tag("kubernetes.io/cluster/ci-20260618", "shared")}, true},
		{"cni cluster ci", []ec2types.Tag{tag("cluster.k8s.amazonaws.com/name", "ci-20260618")}, true},
		{"rack non-ci", []ec2types.Tag{tag("Rack", "prod-rack")}, false},
		{"name only ci is not a signal", []ec2types.Tag{tag("Name", "ci-20260618")}, false},
		{"k8s cluster non-ci", []ec2types.Tag{tag("kubernetes.io/cluster/prod", "owned")}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isCITag(c.tags); got != c.want {
			t.Errorf("%s: isCITag=%v want %v", c.name, got, c.want)
		}
	}
}

func TestReapInVPC(t *testing.T) {
	ci := map[string]struct{}{"vpc-ci123": {}}
	allow := "vpc-0f18b6d1265717215" // allowlisted
	rackTag := []ec2types.Tag{tag("Rack", "ci-20260618")}

	cases := []struct {
		name string
		vpc  string
		tags []ec2types.Tag
		want bool
	}{
		{"ci vpc, any tags", "vpc-ci123", nil, true},
		{"ci vpc, ci tags", "vpc-ci123", rackTag, true},
		{"allowlisted vpc, ci-tagged orphan", allow, rackTag, true},
		{"allowlisted vpc, non-ci resource", allow, []ec2types.Tag{tag("Name", "byo")}, false},
		{"allowlisted vpc, no tags", allow, nil, false},
		{"default/other vpc, ci-tagged", "vpc-other", rackTag, false},
		{"default/other vpc, no tags", "vpc-other", nil, false},
	}
	for _, c := range cases {
		if got := reapInVPC(c.vpc, ci, c.tags); got != c.want {
			t.Errorf("%s: reapInVPC=%v want %v", c.name, got, c.want)
		}
	}
}

func TestIsCITagELBv2(t *testing.T) {
	tg := func(k, v string) elbv2types.Tag {
		return elbv2types.Tag{Key: aws.String(k), Value: aws.String(v)}
	}
	cases := []struct {
		name string
		tags []elbv2types.Tag
		want bool
	}{
		{"rack ci", []elbv2types.Tag{tg("Rack", "ci-20260618")}, true},
		{"lbc cluster ci", []elbv2types.Tag{tg("elbv2.k8s.aws/cluster", "ci-20260618")}, true},
		{"k8s cluster ci", []elbv2types.Tag{tg("kubernetes.io/cluster/ci-20260618", "owned")}, true},
		{"rack non-ci", []elbv2types.Tag{tg("Rack", "prod-rack")}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isCITagELBv2(c.tags); got != c.want {
			t.Errorf("%s: isCITagELBv2=%v want %v", c.name, got, c.want)
		}
	}
}
