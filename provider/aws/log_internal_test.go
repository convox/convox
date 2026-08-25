package aws

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/mock"
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
