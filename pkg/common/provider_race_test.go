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
// helpers. Unset embedded methods panic on call.
type fakeProvider struct {
	structs.Provider // embedded; all unset methods will panic on call
	appReturns       []*structs.App
	appCallNum       int
	appMu            sync.Mutex
	systemReturns    []*structs.System
	systemCallNum    int
	systemMu         sync.Mutex
	logsBody         string
	// processReturns scripts per-call ProcessList results.
	processReturns []structs.Processes
	processCallNum int
	processMu      sync.Mutex
}

// WithContext returns the receiver unchanged.
func (f *fakeProvider) WithContext(ctx context.Context) structs.Provider {
	return f
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

func (f *fakeProvider) ProcessList(app string, opts structs.ProcessListOptions) (structs.Processes, error) {
	f.processMu.Lock()
	defer f.processMu.Unlock()
	if len(f.processReturns) == 0 {
		return structs.Processes{}, nil
	}
	ps := f.processReturns[f.processCallNum]
	if f.processCallNum < len(f.processReturns)-1 {
		f.processCallNum++
	}
	return ps, nil
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

// TestWaitForAppWithLogsContext_RaceFree verifies wait returns only after
// the streamer goroutine exits. Run with `go test -race ./pkg/common/...`.
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

	// Streamer-exit hook must have fired before wait returned.
	select {
	case <-streamerExited:
	default:
		t.Fatal("WaitForAppWithLogsContext returned before streamer goroutine exited (race-window regression)")
	}

	// Read buf under -race to catch any concurrent write from a still-running streamer.
	_ = buf.String()
}

// TestWaitForRackWithLogs_RaceFree — same invariant as App variant above.
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

	select {
	case <-streamerExited:
	default:
		t.Fatal("WaitForRackWithLogs returned before streamer goroutine exited (race-window regression)")
	}

	_ = buf.String()
}

// TestStreamAppLogs_CancelAwareSleep verifies ctx cancellation propagates
// into the streamer's poll-loop sleep within <100ms.
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

// TestStreamSystemLogs_CancelAwareSleep — same invariant as App variant.
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

// TestStreamAppProcessStates_EmitsTransitions verifies transition dedup,
// first-observation suppression for running pods, and terminal state emission.
func TestStreamAppProcessStates_EmitsTransitions(t *testing.T) {
	prev := processStatePollInterval
	processStatePollInterval = 5 * time.Millisecond
	defer func() { processStatePollInterval = prev }()

	p := &fakeProvider{
		processReturns: []structs.Processes{
			{
				{Id: "new-abc123", Name: "gpu-burn", Status: "pending"},
				{Id: "old-xyz789", Name: "gpu-burn", Status: "running"},
			},
			{
				{Id: "new-abc123", Name: "gpu-burn", Status: "running"},
				{Id: "old-xyz789", Name: "gpu-burn", Status: "running"},
			},
			{
				{Id: "new-abc123", Name: "gpu-burn", Status: "running"},
				{Id: "new-def456", Name: "gpu-burn", Status: "crashed"},
			},
			{
				{Id: "new-abc123", Name: "gpu-burn", Status: "running"},
				{Id: "new-def456", Name: "gpu-burn", Status: "crashed"},
			},
		},
	}

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	StreamAppProcessStates(ctx, p, &buf, "gpu-burn")

	out := buf.String()

	if got := strings.Count(out, "new-abc123"); got < 2 {
		t.Errorf("expected new-abc123 to emit at least twice (pending + running); got %d in:\n%s", got, out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("missing initial pending emission; got:\n%s", out)
	}
	if strings.Contains(out, "old-xyz789") {
		t.Errorf("first-observation-running suppression failed for pre-existing pod; got:\n%s", out)
	}
	if !strings.Contains(out, "new-def456") || !strings.Contains(out, "crashed") {
		t.Errorf("missing crashed terminal-state emission for new-def456; got:\n%s", out)
	}
	if got := strings.Count(out, "running\n"); got > 1 {
		t.Errorf("expected exactly one running line (no dedup spam); got %d in:\n%s", got, out)
	}
}

// TestStreamAppProcessStates_ProcessListErrorsAreSilent verifies empty
// ProcessList responses don't break the streamer.
func TestStreamAppProcessStates_ProcessListErrorsAreSilent(t *testing.T) {
	prev := processStatePollInterval
	processStatePollInterval = 5 * time.Millisecond
	defer func() { processStatePollInterval = prev }()

	p := &fakeProvider{}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	StreamAppProcessStates(ctx, p, &buf, "app1")

	if got := buf.String(); got != "" {
		t.Errorf("expected no output for empty ProcessList; got %q", got)
	}
}

// TestStreamAppProcessStates_CancelStopsImmediately verifies ctx cancel
// is observed within one tick.
func TestStreamAppProcessStates_CancelStopsImmediately(t *testing.T) {
	prev := processStatePollInterval
	processStatePollInterval = 50 * time.Millisecond
	defer func() { processStatePollInterval = prev }()

	p := &fakeProvider{}
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamAppProcessStates(ctx, p, &buf, "app1")
	}()

	time.Sleep(60 * time.Millisecond)
	t0 := time.Now()
	cancel()

	select {
	case <-done:
		elapsed := time.Since(t0)
		if elapsed > 100*time.Millisecond {
			t.Fatalf("StreamAppProcessStates took %s to observe ctx cancel; expected <100ms", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAppProcessStates never exited after ctx cancel")
	}
}

// blockingReader blocks on Read until Close, simulating a never-EOF websocket.
type blockingReader struct {
	once   sync.Once
	closed chan struct{}
}

func newBlockingReader() *blockingReader {
	return &blockingReader{closed: make(chan struct{})}
}

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.closed
	return 0, io.EOF
}

func (b *blockingReader) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

// blockingLogsProvider returns a never-EOF reader from AppLogs/SystemLogs.
type blockingLogsProvider struct {
	fakeProvider
	r *blockingReader
}

func (b *blockingLogsProvider) AppLogs(string, structs.LogsOptions) (io.ReadCloser, error) {
	return b.r, nil
}

func (b *blockingLogsProvider) SystemLogs(structs.LogsOptions) (io.ReadCloser, error) {
	return b.r, nil
}

// TestStreamAppLogs_CtxCancelClosesBlockedReader verifies the close-on-cancel
// goroutine unblocks Scanner.Scan on a never-EOF reader.
func TestStreamAppLogs_CtxCancelClosesBlockedReader(t *testing.T) {
	p := &blockingLogsProvider{
		fakeProvider: fakeProvider{appReturns: []*structs.App{{Status: "updating"}}},
		r:            newBlockingReader(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamAppLogs(ctx, p, &buf, "app1")
	}()

	time.Sleep(30 * time.Millisecond)
	t0 := time.Now()
	cancel()

	select {
	case <-done:
		if d := time.Since(t0); d > 500*time.Millisecond {
			t.Fatalf("StreamAppLogs took %s after cancel — close-on-cancel goroutine missing", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAppLogs blocked indefinitely on Scanner.Scan after cancel — close-on-cancel goroutine missing")
	}
}

// TestStreamSystemLogs_CtxCancelClosesBlockedReader — same as App variant.
func TestStreamSystemLogs_CtxCancelClosesBlockedReader(t *testing.T) {
	p := &blockingLogsProvider{
		fakeProvider: fakeProvider{systemReturns: []*structs.System{{Status: "running"}}},
		r:            newBlockingReader(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamSystemLogs(ctx, p, &buf)
	}()

	time.Sleep(30 * time.Millisecond)
	t0 := time.Now()
	cancel()

	select {
	case <-done:
		if d := time.Since(t0); d > 500*time.Millisecond {
			t.Fatalf("StreamSystemLogs took %s after cancel — close-on-cancel goroutine missing", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamSystemLogs blocked indefinitely on Scanner.Scan after cancel — close-on-cancel goroutine missing")
	}
}
