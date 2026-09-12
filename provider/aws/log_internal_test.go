package aws

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func rnfError() error {
	return awserr.New("ResourceNotFoundException", "The specified log group does not exist.", nil)
}

func throttleError() error {
	return awserr.New("ThrottlingException", "Rate exceeded", nil)
}

func fastLogRetries(t *testing.T, max int) {
	prevSleep, prevMax := logRetrySleep, resourceNotFoundMaxRetries
	t.Cleanup(func() {
		logRetrySleep = prevSleep
		resourceNotFoundMaxRetries = prevMax
	})
	logRetrySleep = time.Millisecond
	resourceNotFoundMaxRetries = max
}

// streamLogs must STOP retrying a log group that does not exist, instead of
// looping FilterLogEvents forever (the missing-group storm on fluentd-disabled
// racks). Against the pre-fix code this hangs and the test times out.
func TestStreamLogsStopsOnMissingLogGroup(t *testing.T) {
	fastLogRetries(t, 3)

	m := &mocks.CloudWatchLogsAPI{}
	m.On("FilterLogEvents", mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), rnfError())
	p := &Provider{CloudWatchLogs: m}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, r) }()

	done := make(chan error, 1)
	go func() {
		done <- p.streamLogs(ctx, w, "/convox/rack/missing", "", structs.LogsOptions{})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on missing group, got %v", err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("streamLogs did not stop on a persistently-missing log group (infinite ResourceNotFound retry)")
	}
	m.AssertNumberOfCalls(t, "FilterLogEvents", 3)
}

// With cloudwatch_disable=true, Log short-circuits before any CloudWatch call,
// so no log group is created or written. A mock with no expectations panics if
// the guard is bypassed.
func TestLogCloudwatchDisableShortCircuits(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := &Provider{CloudWatchLogs: m, CloudwatchDisable: true}

	if err := p.Log("app", "stream", time.Now(), "msg"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	m.AssertNumberOfCalls(t, "PutLogEvents", 0)
	m.AssertNumberOfCalls(t, "CreateLogGroup", 0)
}

func TestAppAndSystemLogsCloudwatchDisableReturnEmpty(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := &Provider{CloudWatchLogs: m, CloudwatchDisable: true}

	for _, tc := range []struct {
		name string
		open func() (io.ReadCloser, error)
	}{
		{"AppLogs", func() (io.ReadCloser, error) { return p.AppLogs("app", structs.LogsOptions{}) }},
		{"SystemLogs", func() (io.ReadCloser, error) { return p.SystemLogs(structs.LogsOptions{}) }},
	} {
		r, err := tc.open()
		if err != nil {
			t.Fatalf("%s: expected nil error, got %v", tc.name, err)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("%s: read: %v", tc.name, err)
		}
		if len(b) != 0 {
			t.Fatalf("%s: expected empty reader, got %q", tc.name, b)
		}
	}
	m.AssertNumberOfCalls(t, "FilterLogEvents", 0)
}

func TestUpdateOrDisableLogGroupRetentionCloudwatchDisable(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := &Provider{CloudWatchLogs: m, CloudwatchDisable: true}

	if err := p.UpdateOrDisableLogGroupRetention("app", 30, false); err != nil {
		t.Fatalf("set retention: expected nil, got %v", err)
	}
	if err := p.UpdateOrDisableLogGroupRetention("app", 0, true); err != nil {
		t.Fatalf("disable retention: expected nil, got %v", err)
	}
	m.AssertNumberOfCalls(t, "PutRetentionPolicy", 0)
	m.AssertNumberOfCalls(t, "DeleteRetentionPolicy", 0)
}

// A successful FilterLogEvents must reset the consecutive-miss counter, so an
// eventual-consistency blip while a group is being created never kills a tail.
func TestStreamLogsResourceNotFoundResetsOnSuccess(t *testing.T) {
	fastLogRetries(t, 3)

	follow := true
	withEvent := &cloudwatchlogs.FilterLogEventsOutput{
		Events: []*cloudwatchlogs.FilteredLogEvent{
			{EventId: aws.String("e1"), Timestamp: aws.Int64(1), Message: aws.String("hi")},
		},
	}

	m := &mocks.CloudWatchLogsAPI{}
	m.On("FilterLogEvents", mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), rnfError()).Once()
	m.On("FilterLogEvents", mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), rnfError()).Once()
	m.On("FilterLogEvents", mock.Anything).Return(withEvent, nil).Once() // success: resets the counter
	m.On("FilterLogEvents", mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), rnfError())
	p := &Provider{CloudWatchLogs: m}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, r) }()

	done := make(chan error, 1)
	go func() {
		done <- p.streamLogs(ctx, w, "/convox/rack/eventual", "", structs.LogsOptions{Follow: &follow})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(6 * time.Second):
		cancel()
		<-done
		t.Fatal("streamLogs did not return")
	}
	// 2 RNF, 1 success (reset), then 3 RNF to hit the cap = 6 calls.
	// Without the reset it would stop at 4 calls.
	m.AssertNumberOfCalls(t, "FilterLogEvents", 6)
}

// ThrottlingException is transient and must NOT be bounded by the
// ResourceNotFound cap.
func TestStreamLogsThrottlingNotBounded(t *testing.T) {
	fastLogRetries(t, 3)

	withEvent := &cloudwatchlogs.FilterLogEventsOutput{
		Events: []*cloudwatchlogs.FilteredLogEvent{
			{EventId: aws.String("e1"), Timestamp: aws.Int64(1), Message: aws.String("hi")},
		},
	}

	m := &mocks.CloudWatchLogsAPI{}
	for i := 0; i < 5; i++ { // more throttles than the RNF cap
		m.On("FilterLogEvents", mock.Anything).Return((*cloudwatchlogs.FilterLogEventsOutput)(nil), throttleError()).Once()
	}
	m.On("FilterLogEvents", mock.Anything).Return(withEvent, nil) // success (no NextToken), follow=false -> return
	p := &Provider{CloudWatchLogs: m}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, r) }()

	noFollow := false
	done := make(chan error, 1)
	go func() {
		done <- p.streamLogs(ctx, w, "/convox/rack/app", "", structs.LogsOptions{Follow: &noFollow})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(6 * time.Second):
		cancel()
		<-done
		t.Fatal("streamLogs did not return after throttles cleared")
	}
	// 5 throttles (not bounded by the cap of 3) + 1 success = 6 calls.
	m.AssertNumberOfCalls(t, "FilterLogEvents", 6)
}

// app_cloudwatch_disable must gate app traffic and leave the rack system group
// reachable. "system" is the discriminator every write funnels through, so the
// two flags are pinned in both directions on that name.
func TestLogAppCloudwatchDisable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		appDisable bool
		app        string
		wantGroup  string
	}{
		{name: "off, app writes", app: "myapp", wantGroup: "/convox/rack/myapp"},
		{name: "on, app gated", appDisable: true, app: "myapp"},
		{name: "on, system reachable", appDisable: true, app: "system", wantGroup: "/convox/rack/system"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mocks.CloudWatchLogsAPI{}
			m.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{
				NextSequenceToken: aws.String("token"),
			}, nil)
			p := &Provider{Provider: &k8s.Provider{Name: "rack"}, CloudWatchLogs: m, AppCloudwatchDisable: tc.appDisable}

			stream := fmt.Sprintf("stream/%s", tc.name)
			t.Cleanup(func() { sequenceTokens.Delete(fmt.Sprintf("%s/%s", tc.wantGroup, stream)) })

			if err := p.Log(tc.app, stream, time.Now(), "msg"); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			m.AssertNumberOfCalls(t, "CreateLogGroup", 0)

			if tc.wantGroup == "" {
				m.AssertNumberOfCalls(t, "PutLogEvents", 0)
				return
			}
			m.AssertNumberOfCalls(t, "PutLogEvents", 1)
			in, ok := m.Calls[0].Arguments.Get(0).(*cloudwatchlogs.PutLogEventsInput)
			if !ok {
				t.Fatalf("unexpected PutLogEvents input type %T", m.Calls[0].Arguments.Get(0))
			}
			if got := aws.StringValue(in.LogGroupName); got != tc.wantGroup {
				t.Errorf("log group: got %q, want %q", got, tc.wantGroup)
			}
		})
	}
}

// cloudwatch_disable keeps covering the system group. Collapsing the two flags
// into one derived boolean passes every other case here while silently
// re-enabling rack system writes on racks that already opted out.
func TestLogCloudwatchDisableStillGatesSystem(t *testing.T) {
	for _, appDisable := range []bool{false, true} {
		m := &mocks.CloudWatchLogsAPI{}
		p := &Provider{CloudWatchLogs: m, CloudwatchDisable: true, AppCloudwatchDisable: appDisable}

		if err := p.Log("system", "stream", time.Now(), "msg"); err != nil {
			t.Fatalf("app_cloudwatch_disable=%v: expected nil, got %v", appDisable, err)
		}
		m.AssertNumberOfCalls(t, "PutLogEvents", 0)
		m.AssertNumberOfCalls(t, "CreateLogGroup", 0)
	}
}

// The bare struct literal is deliberate: the embedded provider is nil, so this
// only passes while the guard sits above the ContextTID block.
func TestAppLogsAppCloudwatchDisableReturnsEmpty(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := &Provider{CloudWatchLogs: m, AppCloudwatchDisable: true}

	r, err := p.AppLogs("app", structs.LogsOptions{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(b) != 0 {
		t.Fatalf("expected empty reader, got %q", b)
	}
	m.AssertNumberOfCalls(t, "FilterLogEvents", 0)
}

func TestSystemLogsAppCloudwatchDisableStillReads(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("FilterLogEvents", mock.Anything).Return(&cloudwatchlogs.FilterLogEventsOutput{
		Events: []*cloudwatchlogs.FilteredLogEvent{
			{EventId: aws.String("e1"), Timestamp: aws.Int64(1), Message: aws.String("hi")},
		},
	}, nil)
	p, ok := (&Provider{Provider: &k8s.Provider{Name: "rack"}, CloudWatchLogs: m, AppCloudwatchDisable: true}).
		WithContext(context.Background()).(*Provider)
	if !ok {
		t.Fatal("WithContext did not return an aws provider")
	}

	follow := false
	r, err := p.SystemLogs(structs.LogsOptions{Follow: &follow})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(b), "hi") {
		t.Fatalf("expected system logs to stream, got %q", b)
	}

	in, ok := m.Calls[0].Arguments.Get(0).(*cloudwatchlogs.FilterLogEventsInput)
	if !ok {
		t.Fatalf("unexpected FilterLogEvents input type %T", m.Calls[0].Arguments.Get(0))
	}
	if got := aws.StringValue(in.LogGroupName); got != "/convox/rack/system" {
		t.Errorf("log group: got %q, want %q", got, "/convox/rack/system")
	}
}

func TestUpdateOrDisableLogGroupRetentionAppCloudwatchDisable(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := &Provider{CloudWatchLogs: m, AppCloudwatchDisable: true}

	if err := p.UpdateOrDisableLogGroupRetention("app", 30, false); err != nil {
		t.Fatalf("set retention: expected nil, got %v", err)
	}
	if err := p.UpdateOrDisableLogGroupRetention("app", 0, true); err != nil {
		t.Fatalf("disable retention: expected nil, got %v", err)
	}
	m.AssertNumberOfCalls(t, "PutRetentionPolicy", 0)
	m.AssertNumberOfCalls(t, "DeleteRetentionPolicy", 0)
}

func fastRetentionPut(t *testing.T) {
	prev := logRetentionPutInterval
	t.Cleanup(func() { logRetentionPutInterval = prev })
	logRetentionPutInterval = time.Millisecond
}

func retentionManifest(t *testing.T, body string) *manifest.Manifest {
	t.Helper()

	m, err := manifest.Load([]byte(body), map[string]string{})
	require.NoError(t, err)

	return m
}

func appNamespace(name, release, tid string) *ac.Namespace {
	ns := &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: fmt.Sprintf("rack-%s", name),
			Labels: map[string]string{
				"app":    name,
				"rack":   "rack",
				"system": "convox",
				"type":   "app",
			},
			Annotations: map[string]string{"convox.com/app-release": release},
		},
	}

	if tid != "" {
		ns.Name = fmt.Sprintf("rack-%s-%s", tid, name)
		ns.Labels["tid"] = tid
	}

	return ns
}

func appRelease(namespace, id, body string) *ca.Release {
	return &ca.Release{
		ObjectMeta: am.ObjectMeta{Name: id, Namespace: namespace},
		Spec: ca.ReleaseSpec{
			Created:  time.Now().UTC().Format(common.SortableTime),
			Manifest: body,
		},
	}
}

func retentionProvider(t *testing.T, m *mocks.CloudWatchLogsAPI, days int, objs []runtime.Object, cobjs []runtime.Object) *Provider {
	t.Helper()
	t.Setenv("TEST", "true")
	fastRetentionPut(t)

	a := &atom.MockInterface{}
	a.On("Status", mock.Anything, "app").Return("", "", nil)

	k := &k8s.Provider{
		Atom:      a,
		Cluster:   fake.NewSimpleClientset(objs...),
		Convox:    cvfake.NewSimpleClientset(cobjs...),
		Name:      "rack",
		Namespace: "rack-system",
		Provider:  "test",
	}
	require.NoError(t, k.Initialize(structs.ProviderOptions{}))

	p := &Provider{Provider: k, CloudWatchLogs: m, CloudwatchRetentionInDays: days}
	k.Engine = p

	return p
}

func describePrefixes(m *mocks.CloudWatchLogsAPI) []string {
	var prefixes []string

	for i := range m.Calls {
		c := &m.Calls[i]

		var in *cloudwatchlogs.DescribeLogGroupsInput

		switch c.Method {
		case "DescribeLogGroups":
			in, _ = c.Arguments.Get(0).(*cloudwatchlogs.DescribeLogGroupsInput)
		case "DescribeLogGroupsPages":
			in, _ = c.Arguments.Get(0).(*cloudwatchlogs.DescribeLogGroupsInput)
		default:
			continue
		}

		if in != nil {
			prefixes = append(prefixes, aws.StringValue(in.LogGroupNamePrefix))
		}
	}

	return prefixes
}

func logGroup(name string, retention int64) *cloudwatchlogs.LogGroup {
	g := &cloudwatchlogs.LogGroup{LogGroupName: aws.String(name)}
	if retention > 0 {
		g.RetentionInDays = aws.Int64(retention)
	}
	return g
}

// pageLogGroups drives the generated DescribeLogGroupsPages mock, which hands
// the page function to Called and never invokes it.
func pageLogGroups(m *mocks.CloudWatchLogsAPI, pages ...[]*cloudwatchlogs.LogGroup) {
	m.On("DescribeLogGroupsPages", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn, ok := args.Get(1).(func(*cloudwatchlogs.DescribeLogGroupsOutput, bool) bool)
		if !ok {
			return
		}
		for i, page := range pages {
			if !fn(&cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: page}, i == len(pages)-1) {
				return
			}
		}
	})
}

func retentionPuts(m *mocks.CloudWatchLogsAPI) map[string]int64 {
	puts := map[string]int64{}

	for i := range m.Calls {
		c := &m.Calls[i]
		if c.Method != "PutRetentionPolicy" {
			continue
		}
		in, ok := c.Arguments.Get(0).(*cloudwatchlogs.PutRetentionPolicyInput)
		if !ok {
			continue
		}
		puts[aws.StringValue(in.LogGroupName)] = aws.Int64Value(in.RetentionInDays)
	}

	return puts
}

func TestAppLogRetention(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantDays    int
		wantDisable bool
		wantOk      bool
	}{
		{"no appSettings", "services:\n  web:\n", 0, false, false},
		{"empty awsLogs", "appSettings:\n  awsLogs:\n", 0, false, false},
		{"disableRetention false alone", "appSettings:\n  awsLogs:\n    disableRetention: false\n", 0, false, false},
		{"misspelled key", "appSettings:\n  awsLogs:\n    cwretention: 31\n", 0, false, false},
		{"zero cwRetention", "appSettings:\n  awsLogs:\n    cwRetention: 0\n", 0, false, false},
		{"negative cwRetention", "appSettings:\n  awsLogs:\n    cwRetention: -5\n", 0, false, false},
		{"cwRetention", "appSettings:\n  awsLogs:\n    cwRetention: 31\n", 31, false, true},
		{"disableRetention", "appSettings:\n  awsLogs:\n    disableRetention: true\n", 0, true, true},
		{"both", "appSettings:\n  awsLogs:\n    cwRetention: 31\n    disableRetention: true\n", 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			days, disable, ok := appLogRetention(retentionManifest(t, tc.body))

			if days != tc.wantDays || disable != tc.wantDisable || ok != tc.wantOk {
				t.Fatalf("got (%d, %v, %v), want (%d, %v, %v)", days, disable, ok, tc.wantDays, tc.wantDisable, tc.wantOk)
			}
		})
	}
}

func TestCreateLogGroupRetention(t *testing.T) {
	for _, tc := range []struct {
		name  string
		param int
		want  int64
	}{
		{"unset", 0, 7},
		{"set", 30, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mocks.CloudWatchLogsAPI{}
			m.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil)
			m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)

			p := &Provider{Provider: &k8s.Provider{Name: "rack"}, CloudWatchLogs: m, CloudwatchRetentionInDays: tc.param}

			if err := p.createLogGroup("app1"); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			if got := retentionPuts(m)["/convox/rack/app1"]; got != tc.want {
				t.Errorf("retention: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReconcileLogRetentionOff(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	pageLogGroups(m, []*cloudwatchlogs.LogGroup{logGroup("/convox/rack/app1", 0)})

	p := retentionProvider(t, m, 0, nil, nil)

	// A cancelled context so a regression that drops the guard returns here
	// instead of blocking on the ticker.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p.runLogRetentionReconciler(ctx)

	m.AssertNumberOfCalls(t, "DescribeLogGroupsPages", 0)
	m.AssertNumberOfCalls(t, "DescribeLogGroups", 0)
	m.AssertNumberOfCalls(t, "PutRetentionPolicy", 0)
}

// The per-app skip is the one part of this change that can destroy user data:
// an app declaring its own retention must survive a rack-wide value that is
// shorter.
func TestReconcileLogRetentionSkipsAppOverrides(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	pageLogGroups(m,
		[]*cloudwatchlogs.LogGroup{
			logGroup("/convox/rack/system", 0),
			logGroup("/convox/rack/keep", 0),
			logGroup("/convox/rack/never", 0),
		},
		[]*cloudwatchlogs.LogGroup{
			logGroup("/convox/rack/settled", 30),
			logGroup("/convox/rack/plain", 1),
			logGroup("/convox/rack/gone", 0),
		},
	)

	objs := []runtime.Object{
		appNamespace("keep", "RABCDEFGHIJ", ""),
		appNamespace("never", "RDBCDEFGHIJ", ""),
		appNamespace("settled", "RBBCDEFGHIJ", ""),
		appNamespace("plain", "RCBCDEFGHIJ", ""),
	}
	cobjs := []runtime.Object{
		appRelease("rack-keep", "rabcdefghij", "appSettings:\n  awsLogs:\n    cwRetention: 365\n"),
		appRelease("rack-never", "rdbcdefghij", "appSettings:\n  awsLogs:\n    disableRetention: true\n"),
		appRelease("rack-settled", "rbbcdefghij", "services:\n  web:\n"),
		appRelease("rack-plain", "rcbcdefghij", "appSettings:\n  awsLogs:\n    disableRetention: false\n"),
	}

	p := retentionProvider(t, m, 30, objs, cobjs)

	// cloudwatch_disable and app_cloudwatch_disable stop Convox writing and
	// reading CloudWatch; the groups still exist and still fill.
	p.CloudwatchDisable = true
	p.AppCloudwatchDisable = true

	if err := p.reconcileLogRetention(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	puts := retentionPuts(m)

	for _, group := range []string{"/convox/rack/system", "/convox/rack/plain", "/convox/rack/gone"} {
		if puts[group] != 30 {
			t.Errorf("%s: got %d, want 30", group, puts[group])
		}
	}

	for _, group := range []string{"/convox/rack/keep", "/convox/rack/never", "/convox/rack/settled"} {
		if _, ok := puts[group]; ok {
			t.Errorf("%s: retention was rewritten to %d", group, puts[group])
		}
	}

	want := []string{"/convox/rack/", "/aws/eks/rack/cluster"}
	if got := describePrefixes(m); !reflect.DeepEqual(got, want) {
		t.Errorf("describe prefixes: got %v, want %v", got, want)
	}
}

// One group that cannot be written must not stop the groups after it.
func TestReconcileLogRetentionContinuesPastAFailedPut(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	m.On("PutRetentionPolicy", mock.MatchedBy(func(in *cloudwatchlogs.PutRetentionPolicyInput) bool {
		return aws.StringValue(in.LogGroupName) == "/convox/rack/bad"
	})).Return((*cloudwatchlogs.PutRetentionPolicyOutput)(nil), throttleError())
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	pageLogGroups(m, []*cloudwatchlogs.LogGroup{
		logGroup("/convox/rack/bad", 0),
		logGroup("/convox/rack/after", 0),
	})

	p := retentionProvider(t, m, 30, nil, nil)

	if err := p.reconcileLogRetention(context.Background()); err == nil {
		t.Fatal("expected an error naming the failed group")
	}

	if got := retentionPuts(m)["/convox/rack/after"]; got != 30 {
		t.Errorf("group after the failure: got %d, want 30", got)
	}
}

// A tenant app's namespace carries a tid that its log group name does not, so a
// manifest resolved without it reads the wrong namespace: the override case is
// then skipped for the wrong reason and the plain case is never swept at all.
func TestReconcileLogRetentionTenantApps(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int64
	}{
		{"override", "appSettings:\n  awsLogs:\n    cwRetention: 365\n", 0},
		{"no override", "services:\n  web:\n", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mocks.CloudWatchLogsAPI{}
			m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
			m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
			pageLogGroups(m, []*cloudwatchlogs.LogGroup{
				logGroup("/convox/rack/tenant", 0),
				logGroup("/convox/rack/t1-tenant", 0),
			})

			objs := []runtime.Object{appNamespace("tenant", "RABCDEFGHIJ", "t1")}
			cobjs := []runtime.Object{appRelease("rack-t1-tenant", "rabcdefghij", tc.body)}

			p := retentionProvider(t, m, 7, objs, cobjs)

			if err := p.reconcileLogRetention(context.Background()); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			puts := retentionPuts(m)

			// fluentd names the group from the bare app label; the rack's event
			// controller writes a second one carrying the tid.
			for _, group := range []string{"/convox/rack/tenant", "/convox/rack/t1-tenant"} {
				if got := puts[group]; got != tc.want {
					t.Errorf("%s: got %d, want %d", group, got, tc.want)
				}
			}
		})
	}
}

func TestReconcileLogRetentionSkipsUnreadableManifest(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	pageLogGroups(m, []*cloudwatchlogs.LogGroup{logGroup("/convox/rack/unreadable", 0)})

	objs := []runtime.Object{appNamespace("unreadable", "RABCDEFGHIJ", "")}

	p := retentionProvider(t, m, 7, objs, nil)

	if err := p.reconcileLogRetention(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if _, ok := retentionPuts(m)["/convox/rack/unreadable"]; ok {
		t.Fatalf("group was written despite an unreadable manifest")
	}
}

func TestReconcileEksLogRetention(t *testing.T) {
	for _, tc := range []struct {
		name   string
		groups []*cloudwatchlogs.LogGroup
		want   int64
	}{
		{"absent", nil, 0},
		{"prefix neighbour only", []*cloudwatchlogs.LogGroup{logGroup("/aws/eks/rack/cluster-audit", 0)}, 0},
		{"present and differing", []*cloudwatchlogs.LogGroup{logGroup("/aws/eks/rack/cluster", 1)}, 30},
		{"already settled", []*cloudwatchlogs.LogGroup{logGroup("/aws/eks/rack/cluster", 30)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mocks.CloudWatchLogsAPI{}
			m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
			m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: tc.groups}, nil)
			pageLogGroups(m)

			p := retentionProvider(t, m, 30, nil, nil)

			if err := p.reconcileLogRetention(context.Background()); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			if got := retentionPuts(m)["/aws/eks/rack/cluster"]; got != tc.want {
				t.Errorf("retention: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPutLogGroupRetentionMissingGroup(t *testing.T) {
	fastRetentionPut(t)

	m := &mocks.CloudWatchLogsAPI{}
	m.On("PutRetentionPolicy", mock.Anything).Return((*cloudwatchlogs.PutRetentionPolicyOutput)(nil), rnfError())

	p := &Provider{Provider: &k8s.Provider{Name: "rack"}, CloudWatchLogs: m}

	if err := p.putLogGroupRetention("/convox/rack/gone", 30); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNormalizeRetention(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"", 0},
		{"abc", 0},
		{"0", 0},
		{"-5", 0},
		{"1", 1},
		{"13", 14},
		{"45", 60},
		{"4000", 3653},
		{"99999999999999999999", 3653},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if got := normalizeRetention(tc.value); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestApplyAppLogRetention(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantPut     int64
		wantDeletes int
	}{
		{"no override", "services:\n  web:\n", 0, 0},
		{"zero value", "appSettings:\n  awsLogs:\n    disableRetention: false\n", 0, 0},
		{"cwRetention", "appSettings:\n  awsLogs:\n    cwRetention: 31\n", 60, 0},
		{"disableRetention", "appSettings:\n  awsLogs:\n    disableRetention: true\n", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mocks.CloudWatchLogsAPI{}
			m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
			m.On("DeleteRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.DeleteRetentionPolicyOutput{}, nil)

			p := &Provider{Provider: &k8s.Provider{Name: "rack"}, CloudWatchLogs: m}

			if err := p.applyAppLogRetention("app1", retentionManifest(t, tc.body)); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			if got := retentionPuts(m)["/convox/rack/app1"]; got != tc.wantPut {
				t.Errorf("put: got %d, want %d", got, tc.wantPut)
			}
			m.AssertNumberOfCalls(t, "DeleteRetentionPolicy", tc.wantDeletes)
		})
	}
}

// The atom carries the active release before the namespace annotation mirroring
// it does, so a promote in flight must not read as the previous release.
func TestReconcileLogRetentionPrefersAtomRelease(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	pageLogGroups(m, []*cloudwatchlogs.LogGroup{logGroup("/convox/rack/promoting", 0)})

	objs := []runtime.Object{appNamespace("promoting", "RABCDEFGHIJ", "")}
	cobjs := []runtime.Object{
		appRelease("rack-promoting", "rabcdefghij", "services:\n  web:\n"),
		appRelease("rack-promoting", "rbbcdefghij", "appSettings:\n  awsLogs:\n    cwRetention: 365\n"),
	}

	p := retentionProvider(t, m, 30, objs, cobjs)

	a, ok := p.Atom.(*atom.MockInterface)
	if !ok {
		t.Fatalf("unexpected atom type %T", p.Atom)
	}
	a.ExpectedCalls = nil
	a.On("Status", "rack-promoting", "app").Return("Running", "RBBCDEFGHIJ", nil)

	if err := p.reconcileLogRetention(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if _, written := retentionPuts(m)["/convox/rack/promoting"]; written {
		t.Fatal("the release being promoted declares its own retention and was overwritten")
	}
}

// A panic anywhere under the reconciler would otherwise take the rack API down,
// and an unreachable rack cannot be told to turn the parameter off.
func TestReconcileLogRetentionSafeSurvivesPanic(t *testing.T) {
	m := &mocks.CloudWatchLogsAPI{}
	p := retentionProvider(t, m, 30, nil, nil)

	c, ok := p.Cluster.(*fake.Clientset)
	if !ok {
		t.Fatalf("unexpected cluster type %T", p.Cluster)
	}
	c.PrependReactor("list", "namespaces", func(ktesting.Action) (bool, runtime.Object, error) {
		panic("boom")
	})

	require.NotPanics(t, func() { p.reconcileLogRetentionSafe(context.Background()) })
}

func TestRunLogRetentionReconcilerSweepsThenStops(t *testing.T) {
	prev := logRetentionInterval
	t.Cleanup(func() { logRetentionInterval = prev })
	logRetentionInterval = time.Hour

	m := &mocks.CloudWatchLogsAPI{}
	m.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil)
	m.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, nil)
	pageLogGroups(m, []*cloudwatchlogs.LogGroup{logGroup("/convox/rack/app1", 0)})

	p := retentionProvider(t, m, 30, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		p.runLogRetentionReconciler(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return retentionPuts(m)["/convox/rack/app1"] == 30
	}, 5*time.Second, 5*time.Millisecond, "the first pass must run before the first tick")

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reconciler did not return after the context was cancelled")
	}
}
