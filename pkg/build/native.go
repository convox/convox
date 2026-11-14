package build

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
)

var (
	helperReadyTimeout  = 5 * time.Minute
	helperReadyInterval = 5 * time.Second
	helperWatchInterval = 10 * time.Second
	helperWatchFailures = 3
	helperProbeTimeout  = 8 * time.Second

	runContext = func(ctx context.Context, w io.Writer, name string, args ...string) error {
		cmd := osexec.CommandContext(ctx, name, args...)
		cmd.Stdout = w
		cmd.Stderr = w
		cmd.WaitDelay = 5 * time.Second

		err := cmd.Run()
		if errors.Is(err, osexec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
			return nil
		}

		return err
	}
)

// helper is a buildkitd on a node of the other architecture of a two-architecture build.
type helper struct {
	bk       *BuildKit
	arch     string
	dir      string
	flags    []string
	deadline time.Time
	ready    bool
	lost     bool
}

func newHelper(bk *BuildKit, bb *Build) *helper {
	addr, arch := os.Getenv("BUILD_HELPER_ADDR"), os.Getenv("BUILD_HELPER_ARCH")
	ca, cert, key := os.Getenv("BUILD_HELPER_CA"), os.Getenv("BUILD_HELPER_CERT"), os.Getenv("BUILD_HELPER_KEY")

	archs := common.BuildArchs(os.Getenv("BUILD_ARCHS"))
	if len(archs) != 2 || (arch != "amd64" && arch != "arm64") || bb.Development || bb.Terminal || bb.Push == "" {
		return nil
	}

	if u, err := url.Parse(addr); err != nil || u.Scheme != "tcp" || u.Hostname() == "" || u.Port() == "" {
		return nil
	}
	if _, err := tls.X509KeyPair([]byte(cert), []byte(key)); err != nil {
		return nil
	}
	if !x509.NewCertPool().AppendCertsFromPEM([]byte(ca)) {
		return nil
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil
	}

	flags := []string{"--addr", addr}
	for _, f := range [][3]string{{"--tlscacert", "ca.crt", ca}, {"--tlscert", "tls.crt", cert}, {"--tlskey", "tls.key", key}} {
		file := filepath.Join(dir, f[1])
		if err := os.WriteFile(file, []byte(f[2]), 0600); err != nil {
			os.RemoveAll(dir)
			return nil
		}
		flags = append(flags, f[0], file)
	}

	return &helper{bk: bk, arch: arch, dir: dir, flags: flags, deadline: time.Now().Add(helperReadyTimeout)}
}

// hasOnbuild reports an instruction that a FROM of the built image would run again.
func hasOnbuild(dockerfile string) bool {
	fd, err := os.Open(dockerfile)
	if err != nil {
		return false
	}
	defer fd.Close()

	s := bufio.NewScanner(fd)
	for s.Scan() {
		if fields := strings.Fields(s.Text()); len(fields) > 0 && strings.EqualFold(fields[0], "ONBUILD") {
			return true
		}
	}

	return false
}

func (h *helper) command(args ...string) []string {
	return append(append([]string{}, h.flags...), args...)
}

func (h *helper) probe(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, helperProbeTimeout)
	defer cancel()

	return runContext(ctx, io.Discard, "buildctl", h.command("--timeout", "5", "debug", "workers")...) == nil
}

func (h *helper) wait(ctx context.Context) bool {
	for !h.ready {
		if h.probe(ctx) {
			h.ready = true
			break
		}
		if !time.Now().Before(h.deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(helperReadyInterval):
		}
	}

	return true
}

// run reports whether it stopped the invocation because the helper went silent, which buildctl does not notice.
func (h *helper) run(ctx context.Context, w io.Writer, args []string) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	silent := make(chan bool, 1)

	go func() {
		t := time.NewTicker(helperWatchInterval)
		defer t.Stop()

		for fails := 0; fails < helperWatchFailures; {
			select {
			case <-ctx.Done():
			case <-t.C:
				if h.probe(ctx) {
					fails = 0
				} else {
					fails++
				}
			}
			if ctx.Err() != nil {
				silent <- false
				return
			}
		}

		cancel()
		silent <- true
	}()

	err := runContext(ctx, w, "buildctl", h.command(args...)...)
	cancel()

	return <-silent, err
}

// lineWriter gives each buildctl its own prefix on a writer that several of them share.
type lineWriter struct {
	mu          *sync.Mutex
	w           io.Writer
	prefix      string
	buf         []byte
	unavailable bool
}

func (l *lineWriter) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)

	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			if len(l.buf) > 64*1024 {
				l.flush()
			}
			return len(p), nil
		}
		l.line(l.buf[:i+1])
		l.buf = l.buf[i+1:]
	}
}

func (l *lineWriter) line(line []byte) {
	if bytes.HasPrefix(line, []byte("error: ")) && bytes.Contains(line, []byte("code = Unavailable")) {
		l.unavailable = true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.w, "%s%s", l.prefix, line)
}

func (l *lineWriter) flush() {
	if len(l.buf) > 0 {
		l.line(append(l.buf, '\n'))
		l.buf = nil
	}
}

func (l *lineWriter) warn(msg string) {
	l.flush()

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.w, "WARNING: %s\n", msg)
}

func (h *helper) build(bb *Build, path, dockerfile, tag, prev string, env map[string]string) error {
	ba, err := h.bk.buildArgs(bb.Development, filepath.Join(path, dockerfile), env)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	repo := tag[:strings.LastIndex(tag, ":")]

	primary := "amd64"
	if h.arch == primary {
		primary = "arm64"
	}

	args := func(arch string, local bool) []string {
		a := []string{
			"build",
			"--frontend", "dockerfile.v0",
			"--local", fmt.Sprintf("context=%s", path),
			"--local", fmt.Sprintf("dockerfile=%s", path),
			"--opt", fmt.Sprintf("filename=%s", dockerfile),
			"--output", fmt.Sprintf("type=image,name=%s,push-by-digest=true,name-canonical=true,push=true", repo),
			"--opt", fmt.Sprintf("platform=linux/%s", arch),
			"--metadata-file", filepath.Join(tmp, arch+".json"),
		}
		a = append(a, h.bk.cacheArgs(bb, tag, prev, "-"+arch, local)...)
		a = append(a, ba...)
		if !bb.Cache {
			a = append(a, "--no-cache")
		}
		return a
	}

	primaryArgs, otherArgs := args(primary, true), args(h.arch, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mu := &sync.Mutex{}
	errs := make(chan error, 2)

	go func() {
		w := &lineWriter{mu: mu, w: bb.writer, prefix: fmt.Sprintf("[%s] ", primary)}
		err := runContext(ctx, w, "buildctl", primaryArgs...)
		w.flush()
		errs <- err
	}()

	go func() {
		w := &lineWriter{mu: mu, w: bb.writer, prefix: fmt.Sprintf("[%s] ", h.arch)}
		err := h.other(ctx, w, otherArgs)
		w.flush()
		errs <- err
	}()

	var failed error
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil && failed == nil {
			failed = err
			cancel()
		}
	}
	if failed != nil {
		return platformError(bb, []string{"amd64", "arm64"}, failed)
	}

	if err := combine(ctx, bb, tmp, repo, tag); err != nil {
		return err
	}

	return h.bk.recordEntrypoint(bb, tag)
}

// other builds the helper's architecture on the helper, or on this pod's daemon once the helper is out of reach.
func (h *helper) other(ctx context.Context, w *lineWriter, args []string) error {
	if !h.lost {
		if h.wait(ctx) {
			silent, err := h.run(ctx, w, args)
			w.flush()

			if err == nil || ctx.Err() != nil {
				return err
			}
			if !silent && !w.unavailable && h.probe(ctx) {
				return err
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		h.lost = true
	}

	w.warn(fmt.Sprintf("no %s build node is reachable, building %s on this node under emulation", h.arch, h.arch))

	return runContext(ctx, w, "buildctl", args...)
}

func combine(ctx context.Context, bb *Build, tmp, repo, tag string) error {
	var df strings.Builder

	for _, arch := range []string{"amd64", "arm64"} {
		data, err := os.ReadFile(filepath.Join(tmp, arch+".json"))
		if err != nil {
			return err
		}

		var md struct {
			Digest string `json:"containerimage.digest"`
		}
		if err := json.Unmarshal(data, &md); err != nil {
			return err
		}
		if md.Digest == "" {
			return fmt.Errorf("no image digest for %s", arch)
		}

		fmt.Fprintf(&df, "FROM %s@%s AS %s\n", repo, md.Digest, arch)
	}

	df.WriteString("FROM ${TARGETARCH}\n")

	dir := filepath.Join(tmp, "index")
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(df.String()), 0600); err != nil {
		return err
	}

	return runContext(ctx, bb.writer, "buildctl",
		"build",
		"--frontend", "dockerfile.v0",
		"--local", fmt.Sprintf("context=%s", dir),
		"--local", fmt.Sprintf("dockerfile=%s", dir),
		"--opt", "filename=Dockerfile",
		"--opt", "platform=linux/amd64,linux/arm64",
		"--output", fmt.Sprintf("type=image,name=%s,push=true", tag),
	)
}
