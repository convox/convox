package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

var (
	ProviderWaitDuration = 5 * time.Second
)

// safeWriter wraps an io.Writer with a mutex to make Write calls goroutine-safe.
// Used inside WaitForAppWithLogsContext and WaitForRackWithLogs to coordinate
// the streamer goroutine and the calling goroutine that share the caller's
// writer. bytes.Buffer in tests is not goroutine-safe; *os.File on Windows is
// not guaranteed atomic for concurrent writes; POSIX *os.File is only atomic
// up to PIPE_BUF.
type safeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *safeWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// streamerExitedHookForTest, when non-nil, is invoked at the end of the
// streamer goroutine inside the wait-with-logs helpers. It is the test hook
// referenced by provider_race_test.go to assert the streamer-exit-before-
// helper-return invariant. Production binaries leave this nil and the call
// is skipped. See export_test.go for the test-side accessor.
var streamerExitedHookForTest func()

func AppEnvironment(p structs.Provider, app string) (structs.Environment, error) {
	rs, err := ReleaseLatest(p, app)
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return structs.Environment{}, nil
	}

	env := structs.Environment{}

	if err := env.Load([]byte(rs.Env)); err != nil {
		return nil, err
	}

	return env, nil
}

func AppEnvironmentForRelease(p structs.Provider, app, releaseId string) (structs.Environment, error) {
	rs, err := p.ReleaseGet(app, releaseId)
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return structs.Environment{}, nil
	}

	env := structs.Environment{}

	if err := env.Load([]byte(rs.Env)); err != nil {
		return nil, err
	}

	return env, nil
}

func AppManifest(p structs.Provider, app string) (*manifest.Manifest, *structs.Release, error) {
	a, err := p.AppGet(app)
	if err != nil {
		return nil, nil, err
	}

	if a.Release == "" {
		return nil, nil, errors.WithStack(fmt.Errorf("no release for app: %s", app))
	}

	return ReleaseManifest(p, app, a.Release)
}

func ReleaseLatest(p structs.Provider, app string) (*structs.Release, error) {
	rs, err := p.ReleaseList(app, structs.ReleaseListOptions{Limit: options.Int(1)})
	if err != nil {
		return nil, err
	}

	if len(rs) < 1 {
		return nil, nil
	}

	return p.ReleaseGet(app, rs[0].Id)
}

func ReleaseManifest(p structs.Provider, app, release string) (*manifest.Manifest, *structs.Release, error) {
	r, err := p.ReleaseGet(app, release)
	if err != nil {
		return nil, nil, err
	}

	env := structs.Environment{}

	if err := env.Load([]byte(r.Env)); err != nil {
		return nil, nil, err
	}

	m, err := manifest.Load([]byte(r.Manifest), env)
	if err != nil {
		return nil, nil, err
	}

	return m, r, nil
}

// StreamAppProcessStates polls the app's ProcessList every processStatePollInterval
// and emits one line to w each time a pod's status field transitions. Stops
// when ctx is cancelled. ProcessList errors are swallowed silently — the next
// tick retries — so transient rack-API hiccups don't break the deploy wait.
//
// First-observation suppression: a pod that's already in `running` status the
// first time we see it is suppressed (presumed pre-existing healthy pod, not
// part of the in-flight deploy). Pods that first appear in any other state
// (pending, unhealthy, etc.) are emitted immediately so a brand-new deploy's
// first poll cycle isn't silent.
//
// Output column order mirrors `convox ps` (ID + service-name + status), but
// uses fixed-width formatting rather than the auto-sized table — fits a
// streaming context where order/timing matters more than alignment. One
// line per (pod_id, status) transition. A typical rolling deploy of one
// service emits ~3-5 lines per pod (pending → running, or
// pending → unhealthy → crashed for failures). Bounded by the rollout
// strategy's pod count + transition cap.
//
// Quality-of-life addition introduced because V3 racks have AppLogs
// unimplemented (provider/k8s/app.go) — `convox deploy` was silent between
// "Promoting..." and the final OK / rollback. This streamer fills that gap
// for every caller of WaitForAppWithLogs without requiring the per-engine
// log-stream implementation work.
func StreamAppProcessStates(ctx context.Context, p structs.Provider, w io.Writer, app string) {
	seen := map[string]string{} // pod_id → last status emitted
	ticker := time.NewTicker(processStatePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		psList, err := p.ProcessList(app, structs.ProcessListOptions{})
		if err != nil {
			// Transient rack-API hiccup. Don't spam the stream — the next
			// tick retries. If the rack is genuinely unreachable, the
			// outer WaitForAppRunning eventually times out and we exit.
			continue
		}

		for i := range psList {
			ps := &psList[i]
			prev, hadPrev := seen[ps.Id]

			// Suppress pre-existing healthy pods. A pod observed for the
			// first time in `running` is presumed already-up before the
			// deploy started; emitting its "running" status would clutter
			// the deploy log with unrelated noise. New pods that come up
			// fast (pre-pulled image, no startup probe) skip the pending
			// state and would also be suppressed — acceptable tradeoff.
			if !hadPrev && ps.Status == "running" {
				seen[ps.Id] = ps.Status
				continue
			}

			if prev != ps.Status {
				fmt.Fprintf(w, "  %-32s  %-12s  %s\n", ps.Id, ps.Name, ps.Status)
				seen[ps.Id] = ps.Status
			}
		}
	}
}

// processStatePollInterval is the cadence at which StreamAppProcessStates
// queries ProcessList. 3s balances responsiveness (deploy progress visible
// within one tick of any state change) against rack-API load (ProcessList
// touches the K8s API with a label-selector list and node-name resolution
// per pod). Tunable by tests via the test-only seam below.
var processStatePollInterval = 3 * time.Second

func StreamAppLogs(ctx context.Context, p structs.Provider, w io.Writer, app string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		r, err := p.AppLogs(app, structs.LogsOptions{Prefix: options.Bool(true), Since: options.Duration(5 * time.Second)})
		if err != nil {
			return
		}

		// Close the reader on ctx cancel so Scanner.Scan in
		// copySystemLogs unblocks. Without this, the V3 +
		// Console-proxy path holds the websocket open with no data
		// flowing — the streamer stays parked in
		// bufio.Scanner.Scan() and the CLI hangs waiting on
		// `<-logsDone` forever after the deploy reaches running.
		// Close fires in BOTH branches: the cancel branch unblocks
		// Scanner.Scan, and the closed branch cleans up the
		// underlying ReadCloser when copySystemLogs returns by EOF
		// — without that, every iteration past the first would leak
		// one websocket / HTTP-body resource for the lifetime of the
		// outer streamer.
		closed := make(chan struct{})
		go func(rc io.ReadCloser) {
			select {
			case <-ctx.Done():
			case <-closed:
			}
			if rc != nil {
				rc.Close()
			}
		}(r)

		copySystemLogs(ctx, w, r)
		close(closed)

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

func StreamSystemLogs(ctx context.Context, p structs.Provider, w io.Writer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		r, err := p.SystemLogs(structs.LogsOptions{Prefix: options.Bool(true), Since: options.Duration(5 * time.Second)})
		if err != nil {
			return
		}

		// Twin-site of StreamAppLogs: close in BOTH branches so
		// EOF-driven loop iterations don't leak the underlying reader.
		closed := make(chan struct{})
		go func(rc io.ReadCloser) {
			select {
			case <-ctx.Done():
			case <-closed:
			}
			if rc != nil {
				rc.Close()
			}
		}(r)

		copySystemLogs(ctx, w, r)
		close(closed)

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

func WaitForAppDeleted(p structs.Provider, w io.Writer, app string) error {
	time.Sleep(ProviderWaitDuration) // give the stack time to start updating

	return Wait(ProviderWaitDuration, 35*time.Minute, 2, func() (bool, error) {
		_, err := p.AppGet(app)
		if err == nil {
			return false, nil
		}
		if strings.Contains(err.Error(), "no such app") {
			return true, nil
		}
		if strings.Contains(err.Error(), "app not found") {
			return true, nil
		}
		return false, err
	})
}

func WaitForAppRunning(p structs.Provider, app string) error {
	return WaitForAppRunningContext(context.Background(), p, app)
}

func WaitForAppRunningContext(ctx context.Context, p structs.Provider, app string) error {
	time.Sleep(ProviderWaitDuration) // give the stack time to start updating

	var waitError error

	return WaitContext(ctx, ProviderWaitDuration, 35*time.Minute, 2, func() (bool, error) {
		a, err := p.AppGet(app)
		if err != nil {
			return false, err
		}

		if a.Status == "rollback" {
			waitError = fmt.Errorf("rollback")
		}

		return a.Status == "running", waitError
	})
}

func WaitForAppWithLogs(p structs.Provider, w io.Writer, app string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return WaitForAppWithLogsContext(ctx, p, w, app)
}

func WaitForAppWithLogsContext(ctx context.Context, p structs.Provider, w io.Writer, app string) error {
	sw := &safeWriter{w: w}
	streamCtx, streamCancel := context.WithCancel(ctx)

	// Streamer #1: app log stream. On V2 racks this surfaces container
	// stdout/stderr (and the rack's CloudWatch-stitched system events). On
	// V3 racks AppLogs is unimplemented at the engine layer
	// (provider/k8s/app.go::AppLogs returns ErrNotImplemented), so this
	// goroutine returns immediately. Streamer #2 below covers V3.
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		if streamerExitedHookForTest != nil {
			defer streamerExitedHookForTest()
		}
		StreamAppLogs(streamCtx, p, sw, app)
	}()

	// Streamer #2: per-pod state transitions via ProcessList polling. Cheap
	// (1 HTTP call / 3s) and works on every provider that implements
	// ProcessList — which is all of them. Provides the ONLY deploy-progress
	// signal on V3; on V2 it complements the log stream with explicit
	// per-pod state markers (pending → running → crashed/unhealthy/failed).
	statesDone := make(chan struct{})
	go func() {
		defer close(statesDone)
		StreamAppProcessStates(streamCtx, p, sw, app)
	}()

	err := WaitForAppRunningContext(ctx, p, app)
	streamCancel()
	<-logsDone
	<-statesDone
	return err
}

func WaitForProcessRunning(p structs.Provider, w io.Writer, app, pid string) error {
	return Wait(1*time.Second, 5*time.Minute, 2, func() (bool, error) {
		ps, err := p.ProcessGet(app, pid)
		if err != nil {
			return false, err
		}

		return ps.Status == "running", nil
	})
}

func WaitForRackRunning(p structs.Provider, w io.Writer) error {
	time.Sleep(ProviderWaitDuration) // give the stack time to start updating

	return Wait(ProviderWaitDuration, 35*time.Minute, 2, func() (bool, error) {
		s, err := p.SystemGet()
		if err != nil {
			return false, err
		}

		return s.Status == "running", nil
	})
}

func WaitForRackWithLogs(p structs.Provider, w io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sw := &safeWriter{w: w}
	streamCtx, streamCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if streamerExitedHookForTest != nil {
			defer streamerExitedHookForTest()
		}
		StreamSystemLogs(streamCtx, p, sw)
	}()

	err := WaitForRackRunning(p, w)
	streamCancel()
	<-done
	return err
}

func copySystemLogs(ctx context.Context, w io.Writer, r io.Reader) {
	s := bufio.NewScanner(r)

	for s.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		parts := strings.SplitN(s.Text(), " ", 3)

		if len(parts) < 3 {
			continue
		}

		if strings.HasPrefix(parts[1], "system/") {
			w.Write([]byte(fmt.Sprintf("%s\n", s.Text())))
		}
	}
}
