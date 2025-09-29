package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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
		fmt.Fprintf(stdout, b.Release)
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

	count, _ := io.Copy(c, r)
	defer finalizeBuildLogs(rack, c, b, count)

	for {
		b, err = rack.BuildGet(app(c), b.Id)
		if err != nil {
			return nil, err
		}

		if b.Status == "failed" {
			return nil, fmt.Errorf("build failed")
		}

		if b.Status != "running" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return b, nil
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

	data, err := ioutil.ReadFile(filepath.Join(dir, manifest))
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
	r, err := rack.BuildLogs(b.App, b.Id, structs.LogsOptions{})
	if err != nil {
		return err
	}
	defer r.Close()

	data, err := ioutil.ReadAll(r)
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
		r = ioutil.NopCloser(c.Reader())
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
		fmt.Fprintf(stdout, b.Release)
	}

	return nil
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
