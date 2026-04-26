package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	builder "github.com/convox/convox/pkg/build"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

// resolveSrcCredsPass returns the source registry password from one of the
// three input modes (plaintext flag, env var, stdin) honoring mutual
// exclusion. Emits a deprecation warning to stderr when the plaintext form is
// used. Returns nil + nil error when no form is set (caller leaves
// opts.SrcCredsPass unset; no source-creds path).
//
// Deprecation contract: 3.24.6 warns + continues; 3.25.0 will reject the
// plaintext form. RFC 8594 deprecation pattern.
func resolveSrcCredsPass(c *stdcli.Context) (*string, error) {
	plain := c.String("src-creds-pass")
	envName := c.String("src-creds-pass-env")
	stdinFlag := c.Bool("src-creds-pass-stdin")

	setCount := 0
	if plain != "" {
		setCount++
	}
	if envName != "" {
		setCount++
	}
	if stdinFlag {
		setCount++
	}
	if setCount > 1 {
		return nil, fmt.Errorf("at most one of --src-creds-pass, --src-creds-pass-env, --src-creds-pass-stdin may be specified")
	}
	if setCount == 0 {
		return nil, nil
	}

	if plain != "" {
		// Pin by R3 (UX): exact text reviewed; do NOT alter wording without re-pinning.
		fmt.Fprintln(c.Writer().Stderr, "WARNING: --src-creds-pass=<plaintext> exposes credentials via process listings. Use --src-creds-pass-env <NAME> or --src-creds-pass-stdin instead. Plaintext form will be rejected in 3.25.0.")
		return options.String(plain), nil
	}

	if envName != "" {
		v := os.Getenv(envName)
		if v == "" {
			return nil, fmt.Errorf("--src-creds-pass-env=%s: environment variable not set", envName)
		}
		return options.String(v), nil
	}

	// stdinFlag must be true here.
	raw, err := bufio.NewReader(c.Reader()).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("--src-creds-pass-stdin: read failed: %w", err)
	}
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return nil, fmt.Errorf("--src-creds-pass-stdin: no input received")
	}
	return options.String(raw), nil
}

func init() {
	register("build", "create a build", Build, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.BuildCreateOptions{}), flagRack, flagApp, flagId),
		Usage:    "[dir]",
		Validate: stdcli.ArgsMax(1),
	}, WithCloud())

	register("builds", "list builds", watch(Builds), stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.BuildListOptions{}), flagRack, flagApp, flagWatchInterval),
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("builds export", "export a build", BuildsExport, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			flagApp,
			stdcli.StringFlag("file", "f", "import from file"),
		},
		Usage:    "<build>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("builds import", "import a build", BuildsImport, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			flagApp,
			flagId,
			stdcli.StringFlag("file", "f", "import from file"),
		},
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("builds import-image", "import a prebuilt container image into a new build", BuildsImportImage, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			flagApp,
			stdcli.StringFlag("manifest", "m", "path to convox.yml manifest"),
			stdcli.StringFlag("src-creds-user", "", "source registry username"),
			stdcli.StringFlag("src-creds-pass", "", "source registry password (DEPRECATED — use --src-creds-pass-env or --src-creds-pass-stdin; will be rejected in 3.25.0)"),
			stdcli.StringFlag("src-creds-pass-env", "", "read source registry password from named environment variable"),
			stdcli.BoolFlag("src-creds-pass-stdin", "", "read source registry password from stdin (single line)"),
		},
		Usage:    "<source-image>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("builds info", "get information about a build", BuildsInfo, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "<build>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("builds logs", "get logs for a build", BuildsLogs, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "<build>",
		Validate: stdcli.Args(1),
	}, WithCloud())
}

func Build(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	b, err := build(rack, c, c.Bool("development"))
	if err != nil {
		return err
	}

	c.Writef("Build:   <build>%s</build>\n", b.Id)
	c.Writef("Release: <release>%s</release>\n", b.Release)

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", b.Release)
	}

	return nil
}

func build(rack sdk.Interface, c *stdcli.Context, development bool) (*structs.Build, error) {
	var opts structs.BuildCreateOptions

	if development {
		opts.Development = options.Bool(true)
	}

	if err := c.Options(&opts); err != nil {
		return nil, err
	}

	if opts.Description == nil {
		if _, err := c.Execute("git", "diff", "--quiet"); err == nil {
			if data, err := c.Execute("git", "log", "-n", "1", "--pretty=%h %s", "--abbrev=10"); err == nil {
				opts.Description = options.String(fmt.Sprintf("build %s", strings.TrimSpace(string(data))))
			}
		}
	}

	dir := coalesce(c.Arg(0), ".")

	if data, err := c.Execute("git", "-C", dir, "rev-parse", "HEAD"); err == nil {
		opts.GitSha = options.String(strings.TrimSpace(string(data)))
	}

	if os.Getenv("TEST") == "true" {
		opts.GitSha = nil
	}

	if c.Bool("external") {
		return buildExternal(rack, c, opts)
	}

	c.Startf("Packaging source")

	data, err := common.Tarball(dir)
	if err != nil {
		return nil, err
	}

	c.OK()

	rackVersion := ""
	if rack.ClientType() == "machine" {
		rackVersion = "v3"
	} else {
		s, err := rack.SystemGet()
		if err != nil {
			return nil, err
		}
		rackVersion = s.Version
	}

	var b *structs.Build

	if rackVersion < "20180708231844" {
		c.Startf("Starting build")

		b, err = rack.BuildCreateUpload(app(c), bytes.NewReader(data), opts)
		if err != nil {
			return nil, err
		}
	} else {
		tmp, err := generateTempKey()
		if err != nil {
			return nil, err
		}

		tmp += ".tgz"

		c.Startf("Uploading source")

		o, err := rack.ObjectStore(app(c), tmp, bytes.NewReader(data), structs.ObjectStoreOptions{})
		if err != nil {
			return nil, err
		}

		c.OK()

		c.Startf("Starting build")

		b, err = rack.BuildCreate(app(c), o.Url, opts)
		if err != nil {
			return nil, err
		}
	}

	c.OK()

	r, err := rack.BuildLogs(app(c), b.Id, structs.LogsOptions{})
	if err != nil {
		return nil, err
	}

	count, copyErr := io.Copy(c, r)
	if copyErr != nil {
		c.Writef("warning: build log stream interrupted: %s\n", copyErr)
	}
	defer finalizeBuildLogs(rack, c, b, count)

	buildTimeout := time.After(60 * time.Minute)
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-buildTimeout:
			return nil, fmt.Errorf("build %s timed out after 60 minutes (status: %s). Check: convox builds info %s", b.Id, b.Status, b.Id)
		case <-tick.C:
			b, err = rack.BuildGet(app(c), b.Id)
			if err != nil {
				return nil, err
			}

			switch b.Status {
			case "created", "running":
				// keep waiting
			case "complete":
				return b, nil
			case "failed":
				return nil, fmt.Errorf("build failed")
			default:
				return nil, fmt.Errorf("unexpected build status: %s", b.Status)
			}
		}
	}
}

func buildExternal(rack sdk.Interface, c *stdcli.Context, opts structs.BuildCreateOptions) (*structs.Build, error) {
	dir := coalesce(c.Arg(0), ".")

	s, err := rack.SystemGet()
	if err != nil {
		return nil, err
	}

	b, err := rack.BuildCreate(app(c), "", opts)
	if err != nil {
		return nil, err
	}

	manifest := common.DefaultString(opts.Manifest, "convox.yml")

	data, err := os.ReadFile(filepath.Join(dir, manifest))
	if err != nil {
		return nil, err
	}

	if _, err := rack.BuildUpdate(app(c), b.Id, structs.BuildUpdateOptions{Manifest: options.String(string(data))}); err != nil {
		return nil, err
	}

	u, err := url.Parse(b.Repository)
	if err != nil {
		return nil, err
	}

	auth := ""
	repo := fmt.Sprintf("%s%s", u.Host, u.Path)

	if pass, ok := u.User.Password(); ok {
		auth = fmt.Sprintf(`{%q: { "Username": %q, "Password": %q } }`, repo, u.User.Username(), pass)
	}

	bopts := builder.Options{
		App:         b.App,
		Auth:        auth,
		Cache:       !common.DefaultBool(opts.NoCache, false),
		Development: common.DefaultBool(opts.Development, false),
		Id:          b.Id,
		Manifest:    manifest,
		Push:        repo,
		Rack:        s.Name,
		Source:      fmt.Sprintf("dir://%s", dir),
		Terminal:    true,
	}

	if c.Bool("id") {
		bopts.Output = os.Stderr
		bopts.Terminal = false
	}

	bb, err := builder.New(rack, bopts, &builder.Docker{})
	if err != nil {
		return nil, err
	}

	if err := bb.Execute(); err != nil {
		return nil, err
	}

	ropts := structs.ReleaseCreateOptions{
		Build:       options.String(b.Id),
		Description: options.String(b.Description),
	}

	r, err := rack.ReleaseCreate(b.App, ropts)
	if err != nil {
		return nil, err
	}

	uopts := structs.BuildUpdateOptions{
		Release: options.String(r.Id),
	}

	bu, err := rack.BuildUpdate(b.App, b.Id, uopts)
	if err != nil {
		return nil, err
	}

	return bu, nil
}

func finalizeBuildLogs(rack structs.Provider, c *stdcli.Context, b *structs.Build, count int64) error {
	current, err := rack.BuildGet(b.App, b.Id)
	if err != nil {
		return err
	}
	if current.Status == "running" || current.Status == "created" {
		return nil
	}

	r, err := rack.BuildLogs(current.App, current.Id, structs.LogsOptions{})
	if err != nil {
		return err
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if int64(len(data)) > count {
		c.Write(data[count:])
	}

	return nil
}

func Builds(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.BuildListOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	bs, err := rack.BuildList(app(c), opts)
	if err != nil {
		return err
	}

	t := c.Table("ID", "STATUS", "RELEASE", "STARTED", "ELAPSED", "DESCRIPTION")

	for _, b := range bs {
		started := common.Ago(b.Started)
		elapsed := common.Duration(b.Started, b.Ended)

		t.AddRow(b.Id, b.Status, b.Release, started, elapsed, b.Description)
	}

	return t.Print()
}

func BuildsExport(rack sdk.Interface, c *stdcli.Context) error {
	var w io.Writer

	if file := c.String("file"); file != "" {
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	} else {
		if c.Writer().IsTerminal() {
			return fmt.Errorf("pipe this command into a file or specify --file")
		}
		w = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	c.Startf("Exporting build")

	if err := rack.BuildExport(app(c), c.Arg(0), w); err != nil {
		return err
	}

	return c.OK()
}

func BuildsImport(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	var r io.ReadCloser

	if file := c.String("file"); file != "" {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		r = f
	} else {
		if c.Reader().IsTerminal() {
			return fmt.Errorf("pipe a file into this command or specify --file")
		}
		r = io.NopCloser(c.Reader())
	}

	defer r.Close()

	rackVersion := ""
	var err error
	if rack.ClientType() == "machine" {
		rackVersion = "v3"
	} else {
		s, err := rack.SystemGet()
		if err != nil {
			return err
		}
		rackVersion = s.Version
	}

	c.Startf("Importing build")

	var b *structs.Build

	switch {
	case rackVersion <= "20180416200237":
		b, err = rack.BuildImportMultipart(app(c), r)
	case rackVersion <= "20180708231844":
		b, err = rack.BuildImportUrl(app(c), r)
	default:
		b, err = rack.BuildImport(app(c), r)
	}
	if err != nil {
		return err
	}

	c.OK(b.Release)

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", b.Release)
	}

	return nil
}

func BuildsImportImage(rack sdk.Interface, c *stdcli.Context) error {
	source := c.Arg(0)

	manifestPath := coalesce(c.String("manifest"), "convox.yml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	c.Startf("Creating build")
	b, err := rack.BuildCreate(app(c), "", structs.BuildCreateOptions{
		External: options.Bool(true),
	})
	if err != nil {
		return err
	}

	if _, err := rack.BuildUpdate(app(c), b.Id, structs.BuildUpdateOptions{
		Manifest: options.String(string(data)),
	}); err != nil {
		return err
	}
	if err := c.OK(b.Id); err != nil {
		return err
	}

	var opts structs.BuildImportImageOptions
	if u := c.String("src-creds-user"); u != "" {
		opts.SrcCredsUser = options.String(u)
	}
	pass, err := resolveSrcCredsPass(c)
	if err != nil {
		return err
	}
	if pass != nil {
		opts.SrcCredsPass = pass
	}

	c.Startf("Relaying image %s", source)
	if err := rack.BuildImportImage(app(c), b.Id, source, opts); err != nil {
		return err
	}
	if err := c.OK(); err != nil {
		return err
	}

	c.Startf("Waiting for import to complete")
	// 2 hours covers a multi-service manifest where the rack's 30-min
	// per-service skopeo timeout can legitimately stack (e.g. 3 services
	// ⇒ 90 min worst case). Single-service wizard flows complete in minutes.
	deadline := time.After(2 * time.Hour)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	var consecutiveFails int
	for {
		select {
		case <-deadline:
			return fmt.Errorf("import %s timed out after 2 hours; check: convox builds info %s", b.Id, b.Id)
		case <-tick.C:
			cur, err := rack.BuildGet(app(c), b.Id)
			if err != nil {
				consecutiveFails++
				if consecutiveFails > 5 {
					return fmt.Errorf("build %s still running on rack; check: convox builds info %s (last poll error: %w)", b.Id, b.Id, err)
				}
				continue
			}
			consecutiveFails = 0
			switch cur.Status {
			case "created", "running":
				continue
			case "complete":
				if err := c.OK(); err != nil {
					return err
				}
				c.Startf("Creating release")
				r, err := rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
					Build: options.String(cur.Id),
				})
				if err != nil {
					return err
				}
				if _, err := rack.BuildUpdate(app(c), cur.Id, structs.BuildUpdateOptions{
					Release: options.String(r.Id),
				}); err != nil {
					return err
				}
				if err := c.OK(r.Id); err != nil {
					return err
				}
				if err := c.Writef("Build:   <build>%s</build>\n", cur.Id); err != nil {
					return err
				}
				return c.Writef("Release: <release>%s</release>\n", r.Id)
			case "failed":
				if cur.Reason != "" {
					return fmt.Errorf("import failed: %s", cur.Reason)
				}
				return fmt.Errorf("import failed")
			default:
				return fmt.Errorf("unexpected build status: %s", cur.Status)
			}
		}
	}
}

func BuildsInfo(rack sdk.Interface, c *stdcli.Context) error {
	b, err := rack.BuildGet(app(c), c.Arg(0))
	if err != nil {
		return err
	}

	i := c.Info()

	i.Add("Id", b.Id)
	i.Add("Status", b.Status)
	i.Add("Release", b.Release)
	i.Add("Description", b.Description)
	i.Add("Started", common.Ago(b.Started))
	i.Add("Elapsed", common.Duration(b.Started, b.Ended))

	return i.Print()
}

func BuildsLogs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.LogsOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	r, err := rack.BuildLogs(app(c), c.Arg(0), opts)
	if err != nil {
		return err
	}

	io.Copy(c, r)

	return nil
}
