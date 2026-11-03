package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/smithy-go"
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
		{"rack ci", []ec2types.Tag{tag("Rack", "ci-20260618000000")}, true},
		{"k8s cluster ci", []ec2types.Tag{tag("kubernetes.io/cluster/ci-20260618000000", "shared")}, true},
		{"cni cluster ci", []ec2types.Tag{tag("cluster.k8s.amazonaws.com/name", "ci-20260618000000")}, true},
		{"rack non-ci", []ec2types.Tag{tag("Rack", "prod-rack")}, false},
		{"name only ci is not a signal", []ec2types.Tag{tag("Name", "ci-20260618000000")}, false},
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
	rackTag := []ec2types.Tag{tag("Rack", "ci-20260618000000")}

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
		{"rack ci", []elbv2types.Tag{tg("Rack", "ci-20260618000000")}, true},
		{"lbc cluster ci", []elbv2types.Tag{tg("elbv2.k8s.aws/cluster", "ci-20260618000000")}, true},
		{"k8s cluster ci", []elbv2types.Tag{tg("kubernetes.io/cluster/ci-20260618000000", "owned")}, true},
		{"rack non-ci", []elbv2types.Tag{tg("Rack", "prod-rack")}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isCITagELBv2(c.tags); got != c.want {
			t.Errorf("%s: isCITagELBv2=%v want %v", c.name, got, c.want)
		}
	}
}

func TestIsCIName(t *testing.T) {
	defer func(n func() time.Time) { now = n }(now)
	now = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }

	cases := []struct {
		name string
		want bool
	}{
		{"ci-20261103010000", true},
		{"ci-20261103040000downgrade", true},
		{"ci-20261103040001ssh", false},
		{"ci-20261103115959", false},
		{"ci-20261103010000/httpd", true},
		{"ci-20260618", false},
		{"ci-legacy-rack", false},
		{"ci-20034-0223", false},
		{"ci-20034-0223-registry-abc", false},
		{"ci-20261399999999", false},
		{"prod-ci-20261103010000", false},
		{"convox-ci/registry-test", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isCIName(c.name); got != c.want {
			t.Errorf("isCIName(%q)=%v want %v", c.name, got, c.want)
		}
	}
}

func TestIsCITagSparesYoungRack(t *testing.T) {
	defer func(n func() time.Time) { now = n }(now)
	now = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }

	young := []ec2types.Tag{tag("Rack", "ci-20261103110000")}
	if isCITag(young) {
		t.Error("rack tagged an hour ago must not be reaped")
	}
	if isCITag([]ec2types.Tag{tag("kubernetes.io/cluster/ci-20261103110000", "owned")}) {
		t.Error("cluster tag of a young rack must not be reaped")
	}
	if !isCITag([]ec2types.Tag{tag("kubernetes.io/cluster/ci-20261102010000", "owned")}) {
		t.Error("cluster tag of an old rack must be reaped")
	}
}

func TestWithRetry(t *testing.T) {
	defer func(d time.Duration) { dependencyDelay = d }(dependencyDelay)
	dependencyDelay = time.Millisecond
	ctx := context.Background()

	calls := 0
	err := withRetry(ctx, 3, time.Millisecond, func() error {
		calls++
		return &smithy.GenericAPIError{Code: "InvalidVpcID.NotFound"}
	})
	if err != nil || calls != 1 {
		t.Errorf("not found: err=%v calls=%d, want nil after one call", err, calls)
	}

	calls = 0
	err = withRetry(ctx, 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return &smithy.GenericAPIError{Code: "DependencyViolation"}
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Errorf("dependency violation: err=%v calls=%d, want nil after three calls", err, calls)
	}

	calls = 0
	err = withRetry(ctx, 3, time.Millisecond, func() error {
		calls++
		return &smithy.GenericAPIError{Code: "UnauthorizedOperation"}
	})
	if err == nil || calls != 1 {
		t.Errorf("other 4xx: err=%v calls=%d, want an error after one call", err, calls)
	}
}
