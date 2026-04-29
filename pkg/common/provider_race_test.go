package common

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
)

// TestSafeWriter_ConcurrentWrites verifies safeWriter serializes writes from
// multiple goroutines. Without the mutex, go test -race would detect a race
// on the underlying bytes.Buffer.
func TestSafeWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	sw := &safeWriter{w: &buf}

	const goroutines = 100
	const message = "line\n"
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sw.Write([]byte(message))
		}()
	}
	wg.Wait()

	if got, want := buf.Len(), goroutines*len(message); got != want {
		t.Errorf("safeWriter lost bytes: got %d, want %d", got, want)
	}
}

// fakeProvider satisfies just enough of structs.Provider for the wait+log
// helpers. All other methods come from the embedded zero-value (a nil
// interface), which panics if called — those panics surface mistakes
// cleanly if a future change adds an unexpected provider call to one of
// the helpers under test.
type fakeProvider struct {
	structs.Provider // embedded; all unset methods will panic on call
	appReturns       []*structs.App
	appCallNum       int
	appMu            sync.Mutex
	systemReturns    []*structs.System
	systemCallNum    int
	systemMu         sync.Mutex
	logsBody         string
}

func (f *fakeProvider) AppGet(name string) (*structs.App, error) {
	f.appMu.Lock()
	defer f.appMu.Unlock()
	a := f.appReturns[f.appCallNum]
	if f.appCallNum < len(f.appReturns)-1 {
		f.appCallNum++
	}
	return a, nil
}

func (f *fakeProvider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.logsBody)), nil
}

func (f *fakeProvider) SystemGet() (*structs.System, error) {
	f.systemMu.Lock()
	defer f.systemMu.Unlock()
	s := f.systemReturns[f.systemCallNum]
	if f.systemCallNum < len(f.systemReturns)-1 {
		f.systemCallNum++
	}
	return s, nil
}

func (f *fakeProvider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.logsBody)), nil
}

// TestWaitForAppWithLogsContext_RaceFree exercises the twin-site-1 fix.
// Run with `go test -race ./pkg/common/...` — must report no data races.
//
// Pins THE invariant: WaitForAppWithLogsContext returns ONLY after the
// streamer goroutine exits. Uses a channel signal closed by the streamer-
// exit hook (NOT a timeout sleep) so the assertion is deterministic — a
// regression where wait returns while the streamer is still running
// surfaces immediately as a select default-branch hit.
func TestWaitForAppWithLogsContext_RaceFree(t *testing.T) {
	prev := ProviderWaitDuration
	ProviderWaitDuration = 10 * time.Millisecond
	defer func() { ProviderWaitDuration = prev }()

	p := &fakeProvider{
		appReturns: []*structs.App{
			{Status: "updating"}, {Status: "updating"},
			{Status: "running"}, {Status: "running"},
		},
		logsBody: "T0 system/aws/component log1\nT0 system/aws/component log2\n",
	}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	streamerExited := make(chan struct{})
	SetStreamerExitedHookForTest(func() { close(streamerExited) })
	defer ClearStreamerExitedHookForTest()

	if err := WaitForAppWithLogsContext(ctx, p, &buf, "app1"); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	// THE assertion: after wait() returns, the streamer-exit hook MUST
	// already have fired. A non-blocking select catches the regression:
	// if wait returned while the streamer was still running, the default
	// branch fires and the test fails with a clear message.
	select {
	case <-streamerExited:
		// streamer exited before helper returned — invariant holds
	default:
		t.Fatal("WaitForAppWithLogsContext returned before streamer goroutine exited (race-window regression)")
	}

	// After helper returns AND streamer has exited, the underlying buffer
	// is safely readable from the test goroutine — under -race this read
	// would fire a DATA RACE warning if the streamer goroutine were still
	// writing to it, providing belt-and-suspenders coverage of the same
	// invariant via the race detector.
	_ = buf.String()
}

// TestWaitForRackWithLogs_RaceFree exercises the twin-site-2 fix. Pins the
// same invariant as the twin-site-1 test above.
func TestWaitForRackWithLogs_RaceFree(t *testing.T) {
	prev := ProviderWaitDuration
	ProviderWaitDuration = 10 * time.Millisecond
	defer func() { ProviderWaitDuration = prev }()

	p := &fakeProvider{
		systemReturns: []*structs.System{
			{Status: "updating"}, {Status: "updating"},
			{Status: "running"}, {Status: "running"},
		},
		logsBody: "T0 system/aws/component log1\nT0 system/aws/component log2\n",
	}
	var buf bytes.Buffer

	streamerExited := make(chan struct{})
	SetStreamerExitedHookForTest(func() { close(streamerExited) })
	defer ClearStreamerExitedHookForTest()

	if err := WaitForRackWithLogs(p, &buf); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	// THE assertion: streamer-exit-before-helper-return invariant.
	select {
	case <-streamerExited:
		// streamer exited before helper returned — invariant holds
	default:
		t.Fatal("WaitForRackWithLogs returned before streamer goroutine exited (race-window regression)")
	}

	_ = buf.String()
}

// TestStreamAppLogs_CancelAwareSleep is a regression test for the cancel-
// aware sleep at edit 4 of item-05-race-fix.md. Confirms that ctx
// cancellation propagates into the streamer's poll-loop sleep within
// <100ms — NOT the full 1-second sleep duration. A regression where
// someone reverts the select-block back to time.Sleep would surface as a
// wall-clock measurement >900ms here.
func TestStreamAppLogs_CancelAwareSleep(t *testing.T) {
	p := &fakeProvider{
		appReturns: []*structs.App{{Status: "updating"}},
		logsBody:   "T0 system/aws/component sleeping\n",
	}

	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	sw := &safeWriter{w: &buf}
	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamAppLogs(ctx, p, sw, "app1")
	}()

	// Let the streamer spin one iteration and enter the sleep. The fake
	// provider's logs body is small; one scan + copy + enter-select
	// completes in <50ms on any modern CPU.
	time.Sleep(50 * time.Millisecond)

	// Trigger cancellation and measure how long it takes the streamer to
	// observe it. Pre-fix (time.Sleep), this would be up to 1s. Post-fix
	// (select on ctx.Done + time.After), it is <10ms in practice; the
	// 100ms gate is a generous threshold for slow CI hosts.
	t0 := time.Now()
	cancel()
	select {
	case <-done:
		elapsed := time.Since(t0)
		if elapsed > 100*time.Millisecond {
			t.Fatalf("streamer took %s to observe ctx cancel; expected <100ms (regression: poll-loop sleep is not cancel-aware)", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("streamer never exited after ctx cancel")
	}
}

// TestStreamSystemLogs_CancelAwareSleep is the twin-site-2 equivalent of
// the regression test above. Same invariant, same threshold.
func TestStreamSystemLogs_CancelAwareSleep(t *testing.T) {
	p := &fakeProvider{
		systemReturns: []*structs.System{{Status: "updating"}},
		logsBody:      "T0 system/aws/component sleeping\n",
	}

	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	sw := &safeWriter{w: &buf}
	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamSystemLogs(ctx, p, sw)
	}()

	time.Sleep(50 * time.Millisecond)

	t0 := time.Now()
	cancel()
	select {
	case <-done:
		elapsed := time.Since(t0)
		if elapsed > 100*time.Millisecond {
			t.Fatalf("streamer took %s to observe ctx cancel; expected <100ms (regression: poll-loop sleep is not cancel-aware)", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("streamer never exited after ctx cancel")
	}
}
