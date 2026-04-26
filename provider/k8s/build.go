package k8s

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) buildImage(provider string) string {
	img := fmt.Sprintf("%s-build", p.Image)
	if p.buildPrivileged(provider) {
		img = fmt.Sprintf("%s-build-privileged", p.Image)
	}
	return img
}

func (*Provider) buildPrivileged(provider string) bool {
	return strings.Contains("do gcp aws azure local", provider) // skipcq
}

func (p *Provider) BuildCreate(app, url string, opts structs.BuildCreateOptions) (*structs.Build, error) {
	appObj, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	b := structs.NewBuild(app)

	b.Description = common.DefaultString(opts.Description, "")
	b.GitSha = common.DefaultString(opts.GitSha, "")
	b.Started = time.Now()

	if _, err := p.buildCreate(b); err != nil {
		return nil, errors.WithStack(err)
	}

	if common.DefaultBool(opts.External, false) {
		b, err := p.BuildGet(app, b.Id)
		if err != nil {
			return nil, err
		}

		b.Repository = fmt.Sprintf("https://convox:%s@api.%s/%s%s", p.Password, p.Domain, p.Engine.RepositoryPrefix(), app)

		return b, nil
	}

	auth, err := p.buildAuth(b)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cache := common.DefaultBool(opts.NoCache, true)

	env := map[string]string{
		"BUILD_APP":                       app,
		"BUILD_AUTH":                      string(auth),
		"BUILD_DEVELOPMENT":               fmt.Sprintf("%t", common.DefaultBool(opts.Development, false)),
		"BUILD_GENERATION":                "2",
		"BUILD_ID":                        b.Id,
		"BUILD_MANIFEST":                  common.DefaultString(opts.Manifest, "convox.yml"),
		"BUILD_RACK":                      p.Name,
		"BUILD_URL":                       url,
		"BUILD_GIT_SHA":                   b.GitSha,
		"BUILDKIT_ENABLED":                p.BuildkitEnabled,
		"PROVIDER":                        os.Getenv("PROVIDER"),
		"DISABLE_IMAGE_MANIFEST_CACHE":    os.Getenv("DISABLE_IMAGE_MANIFEST_CACHE"),
		"BUILDKIT_HOST_PATH_CACHE_ENABLE": os.Getenv("BUILDKIT_HOST_PATH_CACHE_ENABLE"),
		"RACK_URL":                        fmt.Sprintf("https://convox:%s@api.%s.svc.cluster.local:5443", p.Password, p.Namespace),
		"CONVOX_TID":                      p.ContextTID(),
	}

	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	env["BUILD_PUSH"] = repo

	buildCmd := fmt.Sprintf("build -method tgz -cache %t", cache)
	if opts.BuildArgs != nil {
		for _, v := range *opts.BuildArgs {
			if len(strings.SplitN(v, "=", 2)) > 2 {
				return nil, structs.ErrBadRequest("invalid build args:%s", v)
			}
			buildCmd = fmt.Sprintf("%s -build-args %s", buildCmd, v)
		}
	}

	psOpts := structs.ProcessRunOptions{
		Command:     options.String(buildCmd),
		Environment: env,
		IsBuild:     true,
	}

	if nlbs := appObj.Parameters[structs.AppParamBuildLabels]; nlbs != "" {
		psOpts.NodeLabels = options.String(nlbs)
	}

	if arch := appObj.Parameters[structs.AppParamBuildArch]; arch != "" {
		if arch != "amd64" && arch != "arm64" {
			return nil, fmt.Errorf("invalid BuildArch: %s, must be amd64 or arm64", arch)
		}
		psOpts.BuildArch = options.String(arch)
	}

	if cpu := appObj.Parameters[structs.AppParamBuildCpu]; cpu != "" {
		v, err := strconv.ParseInt(cpu, 10, 32)
		if err != nil {
			return nil, structs.ErrBadRequest("invalid build cpu: %s, err: %s", cpu, err)
		}
		psOpts.Cpu = options.Int(int(v))
	}

	if mem := appObj.Parameters[structs.AppParamBuildMem]; mem != "" {
		v, err := strconv.ParseInt(mem, 10, 32)
		if err != nil {
			return nil, structs.ErrBadRequest("invalid build mem: %s, err: %s", mem, err)
		}
		psOpts.Memory = options.Int(int(v))
	}

	if p.BuildkitEnabled == "true" {
		psOpts.Image = options.String(p.buildImage(os.Getenv("PROVIDER")))
		psOpts.Privileged = options.Bool(p.buildPrivileged(os.Getenv("PROVIDER")))
	} else {
		psOpts.Image = options.String(p.Image)
		psOpts.Volumes = map[string]string{
			p.Socket: "/var/run/docker.sock",
		}
	}

	ps, err := p.ProcessRun(app, "build", psOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	b, err = p.BuildGet(app, b.Id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	b.Process = ps.Id
	b.Status = "running"

	if _, err := p.buildUpdate(b); err != nil {
		return nil, errors.WithStack(err)
	}

	return b, nil
}

func (p *Provider) BuildExport(app, id string, w io.Writer) error {
	build, err := p.BuildGet(app, id)
	if err != nil {
		return errors.WithStack(err)
	}

	var services []string

	r, err := p.ReleaseGet(app, build.Release)
	if err != nil {
		return errors.WithStack(err)
	}

	env := structs.Environment{}

	if err := env.Load([]byte(r.Env)); err != nil {
		return errors.WithStack(err)
	}

	m, err := manifest.Load([]byte(build.Manifest), env)
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range m.Services {
		services = append(services, m.Services[i].Name)
	}

	if len(services) < 1 {
		return errors.WithStack(structs.ErrBadRequest("no services found to export"))
	}

	bjson, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	dataHeader := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "build.json",
		Mode:     0600,
		Size:     int64(len(bjson)),
	}

	if err := tw.WriteHeader(dataHeader); err != nil {
		return errors.WithStack(err)
	}

	if _, err := tw.Write(bjson); err != nil {
		return errors.WithStack(err)
	}

	tmp, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		return errors.WithStack(err)
	}

	defer os.Remove(tmp)

	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return errors.WithStack(err)
	}

	user, pass, err := p.Engine.RepositoryAuth(app)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, service := range services {

		from := fmt.Sprintf("docker://%s:%s.%s", repo, service, build.Id)
		to := fmt.Sprintf("oci-archive:%s/%s.%s.tar", tmp, service, build.Id)

		if err := exec.Command("skopeo", "copy", "--src-creds", fmt.Sprintf("%s:%s", user, pass), from, to).Run(); err != nil {
			return errors.WithStack(err)
		}
	}

	filepath.Walk(tmp, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// strip tmp dir preffix
		ff := strings.Split(file, "/")
		fname := strings.Join(ff[3:], "/")

		header.Name = fname

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}

			// skipcq
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err := tw.Close(); err != nil {
		return errors.WithStack(err)
	}

	if err := gz.Close(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) BuildGet(app, id string) (*structs.Build, error) {
	b, err := p.buildGet(app, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return b, nil
}

// skopeoExec wraps the external `skopeo` binary so tests can substitute a fake
// without shelling out. Production code runs the binary baked into the rack API
// image (Dockerfile installs it under apt). The ctx deadline bounds long copies
// so a hung registry cannot pin the goroutine indefinitely.
var skopeoExec = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "skopeo", args...).CombinedOutput()
}

// skopeoCopyTimeout caps a single `skopeo copy` invocation. Multi-GB inference
// images over a constrained rack uplink can legitimately need 10+ minutes; 30
// minutes is the upper bound we'll wait before failing the build.
const skopeoCopyTimeout = 30 * time.Minute

// maxConcurrentImports caps the number of in-flight BuildImportImage calls per
// rack API pod. Skopeo copies are network/CPU bound (multi-GB image relays);
// without a cap a misbehaving caller can saturate the rack with parallel
// goroutines. 4 covers the realistic deploy-wizard concurrency for a single
// app + a co-deployed CI pipeline; importing more services in parallel exceeds
// what a single rack uplink can sustain anyway.
//
// Exposed as a var so tests can lower it for parallel-cap verification.
var maxConcurrentImports = 4

// importSlots is a counting semaphore via a buffered channel; capacity matches
// maxConcurrentImports. Acquire = send; release = receive. Initialized lazily
// so a test override of maxConcurrentImports can take effect before any call.
// importSlotsMu guards reassignment of the channel itself (test resets); reads
// take a local snapshot under the mutex and operate on that, so a concurrent
// reset can never race with an in-flight acquire/release.
var (
	importSlotsMu   sync.Mutex
	importSlotsOnce sync.Once
	importSlots     chan struct{}
)

// snapshotImportSlots returns the current channel under the mutex. Callers
// receive a stable reference even if the channel is later reassigned by a
// test reset; sends/receives on a buffered channel are inherently
// goroutine-safe.
func snapshotImportSlots() chan struct{} {
	importSlotsMu.Lock()
	defer importSlotsMu.Unlock()
	importSlotsOnce.Do(func() {
		importSlots = make(chan struct{}, maxConcurrentImports)
	})
	return importSlots
}

// acquireImportSlotOn attempts a non-blocking send on the supplied channel.
// Returns true if a slot was reserved, false if the rack is at capacity.
// Callers obtain ch via snapshotImportSlots so a paired release uses the
// same channel instance.
func acquireImportSlotOn(ch chan struct{}) bool {
	if ch == nil {
		return false
	}
	select {
	case ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// releaseImportSlot frees one slot on the snapshot held by the goroutine that
// originally acquired it. Safe to call when no slot was acquired (no-op via
// the default branch).
func releaseImportSlot(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
	}
}

// validImageRef is a permissive docker-reference allowlist. The character set
// covers registry hosts (`.`, `:`), image paths (`/`), tags (`:`), and
// digests (`@`). The leading anchor blocks values starting with `-` which
// skopeo could parse as a flag. isDangerousImageRef below rejects sequences
// that still look valid per the allowlist but would confuse skopeo or the
// registry URL parser.
var validImageRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]*$`)

// isDangerousImageRef catches sequences that pass validImageRef but could
// either smuggle a flag into skopeo (`@-`, `:-`, `/-`), produce a malformed
// docker reference (double separators, trailing separators) that skopeo
// would spend minutes failing to resolve, or open a path-traversal-shaped
// registry URL (`//`).
func isDangerousImageRef(image string) bool {
	bad := []string{"//", "@-", ":-", "/-", "::", "@@", ":@", "/@"}
	for _, b := range bad {
		if strings.Contains(image, b) {
			return true
		}
	}
	if strings.HasSuffix(image, ":") || strings.HasSuffix(image, "@") || strings.HasSuffix(image, "/") {
		return true
	}
	return false
}

// credScrub matches docker-style inline credentials in URLs (user:pass@host),
// anchored on the `://` scheme prefix so benign `service:port@host` strings
// that appear in K8s event messages aren't redacted. bearerScrub catches
// Authorization: Bearer tokens echoed by skopeo during auth challenges.
// registryAuthScrub catches the X-Registry-Auth header form used by some
// registry middleware. All run before storing error text in Build.Reason
// (visible via `kubectl get build -o yaml`).
var credScrub = regexp.MustCompile(`(://)[A-Za-z0-9._+-]*:[^\s@/]+@`)
var bearerScrub = regexp.MustCompile(`([Bb]earer\s+)[A-Za-z0-9._~+/=-]{10,}`)
var registryAuthScrub = regexp.MustCompile(`([Xx]-[Rr]egistry-[Aa]uth:\s*)[A-Za-z0-9._~+/=-]+`)

func scrubCredentials(s string) string {
	s = credScrub.ReplaceAllString(s, "${1}[REDACTED]@")
	s = bearerScrub.ReplaceAllString(s, "${1}[REDACTED]")
	s = registryAuthScrub.ReplaceAllString(s, "${1}[REDACTED]")
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// parseRegistryHost extracts the registry host from a docker image reference.
// Defaults to docker.io when the first path component has no dot/colon (i.e.
// `user/image` or `image`), matching Docker's canonical interpretation.
func parseRegistryHost(image string) string {
	if i := strings.Index(image, "/"); i != -1 {
		first := image[:i]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			return first
		}
	}
	return "docker.io"
}

// writeImportAuthFile writes a docker-style auth.json containing BOTH source
// and destination registry credentials to a 0600-mode temp file and returns
// its path. Using a unified `--authfile` instead of `--src-creds` / `--dest-creds`
// on argv keeps all secrets out of /proc/<pid>/cmdline where they could be read
// by any sidecar or co-located process. Skopeo resolves per-registry entries
// by host at copy time so one authfile correctly authenticates both directions.
// Source entries are optional (public pulls need no auth); destination entries
// are required whenever the destination host writer needs credentials.
// Callers must os.Remove(path) when done.
func writeImportAuthFile(srcImage, srcUser, srcPass, destHost, destUser, destPass string) (string, error) {
	auths := map[string]map[string]string{}
	if srcUser != "" && srcPass != "" {
		srcHost := parseRegistryHost(srcImage)
		auths[srcHost] = map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(srcUser + ":" + srcPass))}
	}
	if destUser != "" && destHost != "" {
		// destHost returned by Engine.RepositoryHost may include a repository
		// path (e.g. `acct.dkr.ecr.region.amazonaws.com/rack/app`); strip to
		// the bare host so skopeo's auth lookup matches.
		bareDest := destHost
		if i := strings.Index(bareDest, "/"); i != -1 {
			bareDest = bareDest[:i]
		}
		auths[bareDest] = map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(destUser + ":" + destPass))}
	}
	data, err := json.Marshal(map[string]map[string]map[string]string{"auths": auths})
	if err != nil {
		return "", errors.WithStack(err)
	}
	f, err := os.CreateTemp("", "convox-import-auth-*.json")
	if err != nil {
		return "", errors.WithStack(err)
	}
	path := f.Name()
	if err := os.Chmod(path, 0600); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", errors.WithStack(err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", errors.WithStack(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", errors.WithStack(err)
	}
	return path, nil
}

// finalizeImportImageBuild records the terminal state of an import-image build.
// Runs in the goroutine's happy path and (via defer/recover) on panic. Falls
// back to the in-memory Build captured before the goroutine started if a fresh
// BuildGet fails, so the record is never left at "running" forever.
func (p *Provider) finalizeImportImageBuild(app string, captured *structs.Build, runErr error) *structs.Build {
	fresh, getErr := p.BuildGet(app, captured.Id)
	if getErr != nil {
		fmt.Printf("err: BuildImportImage refresh %s: %+v\n", captured.Id, getErr)
		fresh = captured
	}
	fresh.Ended = time.Now().UTC()
	if runErr != nil {
		fresh.Status = "failed"
		fresh.Reason = scrubCredentials(runErr.Error())
	} else {
		fresh.Status = "complete"
		fresh.Reason = ""
	}
	if _, err := p.buildUpdate(fresh); err != nil {
		fmt.Printf("err: BuildImportImage final update %s: %+v\n", captured.Id, err)
	}
	return fresh
}

func (p *Provider) BuildImportImage(app, id, image string, opts structs.BuildImportImageOptions) error {
	if image == "" {
		return errors.WithStack(structs.ErrUnprocessable("image ref required"))
	}
	if !validImageRef.MatchString(image) || isDangerousImageRef(image) {
		return errors.WithStack(structs.ErrUnprocessable("invalid image ref: must start with a letter or digit, contain only A-Z a-z 0-9 . _ : / @ + -, and not include // @- :- or /-"))
	}

	b, err := p.BuildGet(app, id)
	if err != nil {
		return errors.WithStack(err)
	}

	// Reject re-invocation on a build already in flight. The informer may be a
	// few hundred ms behind reality; the buildUpdate below closes any remaining
	// TOCTOU window via K8s optimistic concurrency on ResourceVersion.
	if b.Status == "running" {
		return errors.WithStack(structs.ErrConflict("build %s is already importing; wait for completion before retrying", id))
	}

	if strings.TrimSpace(b.Manifest) == "" {
		return errors.WithStack(structs.ErrUnprocessable("build %s has no manifest; populate via BuildUpdate before BuildImportImage", id))
	}

	m, err := manifest.Load([]byte(b.Manifest), map[string]string{})
	if err != nil {
		return errors.WithStack(structs.ErrUnprocessable("manifest parse failed: %s", err.Error()))
	}
	if len(m.Services) == 0 {
		return errors.WithStack(structs.ErrUnprocessable("manifest has no services to relay"))
	}

	// Acquire a concurrency slot BEFORE flipping Status to running so a
	// rejection leaves no half-mutated build behind. Snapshot the channel so
	// the goroutine's release operates on the same instance even if a test
	// reset reassigns the package-level pointer mid-flight (production code
	// never reassigns it). Released inside the goroutine's outermost defer
	// (and on the buildUpdate-fail path below).
	importSlot := snapshotImportSlots()
	if !acquireImportSlotOn(importSlot) {
		return errors.WithStack(structs.ErrConflict("rack at concurrent-import cap (%d in flight); wait and retry", maxConcurrentImports))
	}

	b.Status = "running"
	b.Started = time.Now().UTC()
	b.Reason = ""
	if _, err := p.buildUpdate(b); err != nil {
		// We acquired a slot but the buildUpdate failed; release before
		// returning so a Conflict here doesn't leak a slot.
		releaseImportSlot(importSlot)
		// K8s returns a Conflict (HTTP 409) when the informer-cached
		// ResourceVersion is stale — a concurrent writer (including a racing
		// BuildImportImage caller) beat us to the update. Surface as a clean
		// 409 so clients see the same status they'd get from the running guard.
		if k8serrors.IsConflict(err) {
			return errors.WithStack(structs.ErrConflict("build %s is currently being modified; wait and retry", id))
		}
		return errors.WithStack(err)
	}

	// Capture the opts values into locals so the goroutine doesn't hold pointers
	// into the caller's request-scoped memory once the HTTP handler returns.
	var srcUser, srcPass string
	if opts.SrcCredsUser != nil {
		srcUser = *opts.SrcCredsUser
	}
	if opts.SrcCredsPass != nil {
		srcPass = *opts.SrcCredsPass
	}

	// Snapshot the audit actor BEFORE the synchronous :start emit. The same
	// captured value is passed to the asynchronous :done emit so a future
	// refactor that swaps p.ctx between launch and goroutine entry still pairs
	// :start/:done with the same actor (the goroutine must NOT re-read
	// p.ContextActor() after launch — see TestBuildImportImage_DoneEventReadsCapturedNotReReadFromCtx).
	capturedActor := p.ContextActor()

	// Emit :start synchronously before launching the goroutine so subscribers
	// cannot observe :done before :start when the webhook dispatcher fans out
	// on its own goroutines.
	_ = p.EventSend("build:import-image:start", structs.EventSendOptions{
		Data: map[string]string{"actor": capturedActor, "app": app, "build": b.Id, "image": image},
	})

	// Copy the build by value for the goroutine. Tags is a map — we nil it
	// defensively rather than deep-copy because buildMarshal does not persist
	// it and we do not want the goroutine aliasing the caller's map.
	captured := *b
	captured.Tags = nil
	go func() {
		var runErr error
		defer func() {
			// Outermost cleanup: release the concurrency slot LAST so even a
			// panic inside finalize or EventSend does not leak the slot.
			defer releaseImportSlot(importSlot)
			if r := recover(); r != nil {
				fmt.Printf("panic in BuildImportImage for build %s: %v\nstack: %s\n", captured.Id, r, debug.Stack())
				runErr = fmt.Errorf("panic during image import: %v", r)
			}
			final := p.finalizeImportImageBuild(app, &captured, runErr)
			_ = p.EventSend("build:import-image:done", structs.EventSendOptions{
				Data: map[string]string{"actor": capturedActor, "app": app, "build": captured.Id, "status": final.Status},
			})
		}()

		runErr = p.buildImportImageRun(app, &captured, m, image, srcUser, srcPass)
	}()

	return nil
}

func (p *Provider) buildImportImageRun(app string, b *structs.Build, m *manifest.Manifest, imageRef, srcUser, srcPass string) error {
	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return errors.WithStack(err)
	}
	destUser, destPass, err := p.Engine.RepositoryAuth(app)
	if err != nil {
		return errors.WithStack(err)
	}

	// Unified authfile covers both source (optional) and destination creds so
	// neither touches /proc/<pid>/cmdline. `--authfile` works for both lookups.
	authPath, err := writeImportAuthFile(imageRef, srcUser, srcPass, repo, destUser, destPass)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = os.Remove(authPath) }()

	for i := range m.Services {
		svc := &m.Services[i]
		src := imageRef
		if svc.Image != "" {
			src = svc.Image
		}
		if !validImageRef.MatchString(src) || isDangerousImageRef(src) {
			return fmt.Errorf("invalid image ref for service %s", svc.Name)
		}
		dst := fmt.Sprintf("%s:%s.%s", repo, svc.Name, b.Id)

		args := []string{
			"copy",
			"--authfile", authPath,
			"--",
			fmt.Sprintf("docker://%s", src),
			fmt.Sprintf("docker://%s", dst),
		}

		ctx, cancel := context.WithTimeout(context.Background(), skopeoCopyTimeout)
		out, runErr := skopeoExec(ctx, args...)
		cancel()
		if runErr != nil {
			return fmt.Errorf("image relay failed for service %s: %s (output: %s)", svc.Name, runErr, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func (p *Provider) BuildImport(app string, r io.Reader) (*structs.Build, error) {
	var source structs.Build

	// set up the new build
	target := structs.NewBuild(app)
	target.Started = time.Now().UTC()

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tr := tar.NewReader(gz)
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	imgBySvc := map[string]string{}

	if err != nil {
		return nil, fmt.Errorf("failed to create img tmp directory - %s", err.Error())
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.WithStack(err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if strings.HasSuffix(header.Name, ".tar") {
			f, err := os.Create(fmt.Sprintf("%s/%s", tmp, header.Name))
			if err != nil {
				return nil, errors.Errorf("failed to untar image - %s", err.Error())
			}

			// skipcq
			_, err = io.Copy(f, tr)
			if err != nil {
				errors.Errorf("failed to write image - %s", err.Error())
			}

			svc := strings.Split(header.Name, ".")[0]
			imgBySvc[svc] = f.Name()
		}

		if header.Name == "build.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			if err := json.Unmarshal(data, &source); err != nil {
				return nil, errors.WithStack(err)
			}

			target.Id = structs.NewBuild(app).Id
		}
	}

	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	user, pass, err := p.Engine.RepositoryAuth(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for svc, img := range imgBySvc {
		dst := fmt.Sprintf("%s:%s.%s", repo, svc, target.Id)

		b, err := exec.Command("skopeo", "copy", "--dest-creds", fmt.Sprintf("%s:%s", user, pass), fmt.Sprintf("oci-archive:%s", img), fmt.Sprintf("docker://%s", dst)).CombinedOutput()
		if err != nil {
			errors.Errorf("failed to push image - %s\n%s", err.Error(), string(b))
		}
	}

	target.Description = source.Description
	target.Ended = time.Now().UTC()
	target.Logs = source.Logs
	target.Manifest = source.Manifest

	if _, err := p.buildCreate(target); err != nil {
		return nil, errors.WithStack(err)
	}

	rr, err := p.ReleaseCreate(app, structs.ReleaseCreateOptions{Build: options.String(target.Id)})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	target.Status = "complete"
	target.Release = rr.Id

	if _, err := p.buildUpdate(target); err != nil {
		return nil, errors.WithStack(err)
	}

	return target, nil
}

func (p *Provider) BuildLogs(app, id string, opts structs.LogsOptions) (io.ReadCloser, error) {
	b, err := p.BuildGet(app, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	opts.Since = nil

	switch b.Status {
	case "running":
		if b.Process == "" {
			return nil, fmt.Errorf("build %s has running status but no process ID", id)
		}
		return p.ProcessLogs(app, b.Process, opts)
	case "created":
		return p.buildLogsStreamFromCreated(app, id, opts)
	default:
		return p.buildLogsFromStorage(app, id, b)
	}
}

func (p *Provider) buildLogsStreamFromCreated(app, id string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()
	go p.streamBuildLogsFromCreated(w, app, id, opts)
	return r, nil
}

func (p *Provider) streamBuildLogsFromCreated(w io.WriteCloser, app, id string, opts structs.LogsOptions) {
	defer w.Close()

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("panic in streamBuildLogsFromCreated for build %s: %v\nstack: %s\n", id, r, debug.Stack())
		}
	}()

	timeout := time.After(5 * time.Minute)
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	startTime := time.Now()
	useDirectAPI := false

	for {
		select {
		case <-timeout:
			fmt.Fprintf(w, "build %s did not start within 5 minutes\n", id)
			return

		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, "waiting for build to start...\n"); err != nil {
				return
			}

		case <-tick.C:
			if !useDirectAPI && time.Since(startTime) >= 10*time.Second {
				useDirectAPI = true
			}

			b, err := p.getBuildStatus(app, id, useDirectAPI)
			if err != nil {
				fmt.Printf("err: build status check for %s: %+v\n", id, err)
				continue
			}

			switch b.Status {
			case "running":
				if b.Process == "" {
					continue
				}
				tick.Stop()
				heartbeat.Stop()
				p.streamProcessLogs(w, app, b.Process, opts)
				return
			case "failed", "complete":
				p.writeStoredBuildLogs(w, app, id, b)
				return
			case "created":
				continue
			default:
				fmt.Fprintf(w, "unexpected build status: %s\n", b.Status)
				return
			}
		}
	}
}

func (p *Provider) getBuildStatus(app, id string, useDirectAPI bool) (*structs.Build, error) {
	if useDirectAPI {
		kb, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(app)).Get(
			strings.ToLower(id), am.GetOptions{},
		)
		if err != nil {
			return nil, err
		}
		return p.buildUnmarshal(kb)
	}
	return p.BuildGet(app, id)
}

func (p *Provider) writeStoredBuildLogs(w io.Writer, app, id string, b *structs.Build) {
	if b.Logs == "" {
		if b.Status == "failed" {
			fmt.Fprintf(w, "build failed -- no build logs available\n")
		}
		return
	}

	u, err := url.Parse(b.Logs)
	if err != nil {
		fmt.Printf("err: parse build %s logs URL: %+v\n", id, err)
		return
	}

	switch u.Scheme {
	case "object":
		r, err := p.ObjectFetch(u.Hostname(), u.Path)
		if err != nil {
			fmt.Printf("err: fetch stored logs for build %s: %+v\n", id, err)
			return
		}
		defer r.Close()
		io.Copy(w, r)
	default:
		fmt.Printf("err: unknown log scheme %q for build %s\n", u.Scheme, id)
	}
}

func (p *Provider) buildLogsFromStorage(app, id string, b *structs.Build) (io.ReadCloser, error) {
	u, err := url.Parse(b.Logs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	switch u.Scheme {
	case "object":
		return p.ObjectFetch(u.Hostname(), u.Path)
	default:
		return nil, errors.WithStack(fmt.Errorf("unable to read logs for build: %s", id))
	}
}

func (p *Provider) BuildList(app string, opts structs.BuildListOptions) (structs.Builds, error) {
	_, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	limit := common.DefaultInt(opts.Limit, 10)

	bs, err := p.buildList(app, limit)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sort.Slice(bs, func(i, j int) bool { return bs[i].Started.After(bs[j].Started) })

	if len(bs) > limit {
		bs = bs[0:limit]
	}

	return bs, nil
}

func (p *Provider) BuildUpdate(app, id string, opts structs.BuildUpdateOptions) (*structs.Build, error) {
	b, err := p.BuildGet(app, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if opts.Ended != nil {
		b.Ended = *opts.Ended
	}

	if opts.Entrypoint != nil {
		b.Entrypoint = *opts.Entrypoint
	}

	if opts.Logs != nil {
		b.Logs = *opts.Logs
	}

	if opts.Manifest != nil {
		b.Manifest = *opts.Manifest
	}

	if opts.Release != nil {
		b.Release = *opts.Release
	}

	if opts.Started != nil {
		b.Started = *opts.Started
	}

	if opts.Status != nil {
		b.Status = *opts.Status
	}

	if _, err := p.buildUpdate(b); err != nil {
		return nil, errors.WithStack(err)
	}

	return b, nil
}

func (p *Provider) buildAuth(b *structs.Build) ([]byte, error) {
	type authEntry struct {
		Username string
		Password string
	}

	auth := map[string]authEntry{}

	rs, err := p.RegistryList()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, r := range rs {
		un, pw, err := p.Engine.RegistryAuth(r.Server, r.Username, r.Password)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		auth[r.Server] = authEntry{
			Username: un,
			Password: pw,
		}
	}

	repo, remote, err := p.Engine.RepositoryHost(b.App)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if remote {
		user, pass, err := p.Engine.RepositoryAuth(b.App)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if user != "" {
			auth[repo] = authEntry{
				Username: user,
				Password: pass,
			}
		}
	}

	data, err := json.Marshal(auth)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) buildCreate(b *structs.Build) (*structs.Build, error) {
	kb, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(b.App)).Create(p.buildMarshal(b))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.buildUnmarshal(kb)
}

// skipcq
func (p *Provider) buildGet(app, id string) (*structs.Build, error) {
	kb, err := p.GetBuildFromInformer(strings.ToLower(id), p.AppNamespace(app))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.buildUnmarshal(kb)
}

// skipcq
func (p *Provider) buildList(app string, limit int) (structs.Builds, error) {
	kbs, err := p.ListBuildsFromInformer(p.AppNamespace(app), "", limit)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	bs := structs.Builds{}

	for _, kb := range kbs.Items {
		b, err := p.buildUnmarshal(&kb)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		bs = append(bs, *b)
	}

	return bs, nil
}

// skipcq
func (p *Provider) buildMarshal(b *structs.Build) *ca.Build {
	return &ca.Build{
		ObjectMeta: am.ObjectMeta{
			Annotations: map[string]string{
				"git-sha": b.GitSha,
			},
			Namespace: p.AppNamespace(b.App),
			Name:      strings.ToLower(b.Id),
			Labels: map[string]string{
				"system": "convox",
				"rack":   p.Name,
				"app":    b.App,
			},
		},
		Spec: ca.BuildSpec{
			Description: b.Description,
			Ended:       b.Ended.UTC().Format(common.SortableTime),
			Entrypoint:  b.Entrypoint,
			Logs:        b.Logs,
			Manifest:    b.Manifest,
			Process:     b.Process,
			Reason:      b.Reason,
			Release:     b.Release,
			Started:     b.Started.UTC().Format(common.SortableTime),
			Status:      b.Status,
		},
	}
}

// skipcq
func (p *Provider) buildUnmarshal(kb *ca.Build) (*structs.Build, error) {
	started, err := time.Parse(common.SortableTime, kb.Spec.Started)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ended, err := time.Parse(common.SortableTime, kb.Spec.Ended)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	b := &structs.Build{
		App:         kb.ObjectMeta.Labels["app"],
		Description: kb.Spec.Description,
		Ended:       ended,
		Entrypoint:  kb.Spec.Entrypoint,
		GitSha:      kb.ObjectMeta.Annotations["git-sha"],
		Id:          strings.ToUpper(kb.ObjectMeta.Name),
		Logs:        kb.Spec.Logs,
		Manifest:    kb.Spec.Manifest,
		Process:     kb.Spec.Process,
		Reason:      kb.Spec.Reason,
		Release:     kb.Spec.Release,
		Started:     started,
		Status:      kb.Spec.Status,
	}

	return b, nil
}

// skipcq
func (p *Provider) buildUpdate(b *structs.Build) (*structs.Build, error) {
	kbo, err := p.GetBuildFromInformer(strings.ToLower(b.Id), p.AppNamespace(b.App))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	kbn := p.buildMarshal(b)

	kbn.ObjectMeta = kbo.ObjectMeta

	kb, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(b.App)).Update(kbn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.buildUnmarshal(kb)
}
