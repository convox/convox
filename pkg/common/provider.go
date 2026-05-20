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

// safeWriter wraps an io.Writer with a mutex for concurrent goroutine safety.
type safeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *safeWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// streamerExitedHookForTest is a test-only hook; see export_test.go.
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

// StreamAppProcessStates polls ProcessList and emits one line per pod status
// transition. Fills the V3 deploy-progress gap where AppLogs is unimplemented.
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
			continue // transient error; next tick retries
		}

		for i := range psList {
			ps := &psList[i]
			prev, hadPrev := seen[ps.Id]

			// Suppress pre-existing healthy pods on first observation.
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

// processStatePollInterval controls StreamAppProcessStates tick rate.
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

		// Close reader on cancel or EOF to unblock Scanner.Scan and avoid leaks.
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

		// Same close-on-cancel/EOF pattern as StreamAppLogs.
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

	// App log stream (V2 only; V3 AppLogs is unimplemented).
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		if streamerExitedHookForTest != nil {
			defer streamerExitedHookForTest()
		}
		StreamAppLogs(streamCtx, p, sw, app)
	}()

	// Per-pod state transitions via ProcessList polling (primary signal on V3).
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
