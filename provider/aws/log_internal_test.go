package aws

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	mocks "github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
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
