package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("releases", "list releases for an app", watch(Releases), stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.ReleaseListOptions{}), flagRack, flagApp, flagWatchInterval),
		Validate: stdcli.Args(0),
	})

	register("releases create-from", "create a new release using the build from one release and the environment from another for an app", ReleasesCreateFrom, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.ReleaseCreateFromOptions{}), flagRack, flagApp),
		Validate: stdcli.Args(0),
	})

	register("releases info", "get information about a release", ReleasesInfo, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Validate: stdcli.Args(1),
	})

	register("releases manifest", "get manifest for a release", ReleasesManifest, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Validate: stdcli.Args(1),
	})

	register("releases promote", "promote a release", ReleasesPromote, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack, flagForce},
		Validate: stdcli.ArgsMax(1),
	})

	register("releases rollback", "copy an old release forward and promote it", ReleasesRollback, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagId, flagRack, flagForce},
		Validate: stdcli.Args(1),
	})
}

func Releases(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.ReleaseListOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	a, err := rack.AppGet(app(c))
	if err != nil {
		return err
	}

	rs, err := rack.ReleaseList(app(c), opts)
	if err != nil {
		return err
	}

	t := c.Table("ID", "STATUS", "BUILD", "CREATED", "DESCRIPTION")

	for _, r := range rs {
		status := ""

		if a.Release == r.Id {
			status = "active"
		}

		t.AddRow(r.Id, status, r.Build, common.Ago(r.Created), r.Description)
	}

	return t.Print()
}

func ReleasesCreateFrom(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.ReleaseCreateFromOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if (opts.BuildFrom == nil && opts.UseActiveReleaseBuild == nil) || (opts.BuildFrom != nil && opts.UseActiveReleaseBuild != nil) {
		return fmt.Errorf("must specify either --build-from or --use-active-release-build")
	}

	if (opts.EnvFrom == nil && opts.UseActiveReleaseEnv == nil) || (opts.EnvFrom != nil && opts.UseActiveReleaseEnv != nil) {
		return fmt.Errorf("must specify either --env-from or --use-active-release-env")
	}

	a, err := rack.AppGet(app(c))
	if err != nil {
		return err
	}

	buildRelease := a.Release
	if opts.BuildFrom != nil {
		buildRelease = *opts.BuildFrom
	}

	buildRs, err := rack.ReleaseGet(app(c), buildRelease)
	if err != nil {
		return err
	}

	envRelease := a.Release
	if opts.EnvFrom != nil {
		envRelease = *opts.EnvFrom
	}

	envRs, err := rack.ReleaseGet(app(c), envRelease)
	if err != nil {
		return err
	}

	c.Writef("Using build from release: %s\n", buildRs.Id)
	c.Writef("Using env from release: %s\n", envRs.Id)

	result, err := rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
		Build:       options.String(buildRs.Build),
		Env:         options.String(envRs.Env),
		Description: options.String(fmt.Sprintf("Created from build release: %s and environment release: %s", buildRs.Id, envRs.Id)),
	})
	if err != nil {
		return err
	}

	c.Writef("Created release: %s\n", result.Id)

	if opts.Promote != nil && *opts.Promote {
		return releasePromote(rack, c, app(c), result.Id, false)
	}

	return c.OK()
}

func ReleasesInfo(rack sdk.Interface, c *stdcli.Context) error {
	r, err := rack.ReleaseGet(app(c), c.Arg(0))
	if err != nil {
		return err
	}

	i := c.Info()

	i.Add("Id", r.Id)
	i.Add("Build", r.Build)
	i.Add("Created", r.Created.Format(time.RFC3339))
	i.Add("Description", r.Description)
	i.Add("Env", r.Env)

	return i.Print()
}

func ReleasesManifest(rack sdk.Interface, c *stdcli.Context) error {
	release := c.Arg(0)

	r, err := rack.ReleaseGet(app(c), release)
	if err != nil {
		return err
	}

	if r.Build == "" {
		return fmt.Errorf("no build for release: %s", release)
	}

	b, err := rack.BuildGet(app(c), r.Build)
	if err != nil {
		return err
	}

	fmt.Fprintf(c, "%s\n", strings.TrimSpace(b.Manifest))

	return nil
}

func ReleasesPromote(rack sdk.Interface, c *stdcli.Context) error {
	release := c.Arg(0)

	if release == "" {
		rs, err := rack.ReleaseList(app(c), structs.ReleaseListOptions{Limit: options.Int(1)})
		if err != nil {
			return err
		}

		if len(rs) == 0 {
			return fmt.Errorf("no releases to promote")
		}

		release = rs[0].Id
	}

	return releasePromote(rack, c, app(c), release, c.Bool("force"))
}

func releasePromote(rack sdk.Interface, c *stdcli.Context, app, id string, force bool) error {
	if id == "" {
		return fmt.Errorf("no release to promote")
	}

	a, err := rack.AppGet(app)
	if err != nil {
		return err
	}

	if !force && a.Status != "running" {
		c.Startf("Waiting for app to be ready")

		if err := common.WaitForAppRunning(rack, app); err != nil {
			return err
		}

		c.OK()
	}

	c.Startf("Promoting <release>%s</release>", id)
	c.Writef("\n")

	ctx, cancel := context.WithCancel(c.Context)

	go printPromotingInProgress(ctx, c)

	if err := rack.ReleasePromote(app, id, structs.ReleasePromoteOptions{
		Force: &force,
	}); err != nil {
		cancel()
		return err
	}

	cancel()

	if err := common.WaitForAppWithLogs(rack, c, app); err != nil {
		return err
	}

	a, err = rack.AppGet(app)
	if err != nil {
		return err
	}

	if a.Release != id {
		return fmt.Errorf("rollback")
	}

	return c.OK()
}

func ReleasesRollback(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	release := c.Arg(0)

	c.Startf("Rolling back to <release>%s</release>", release)

	ro, err := rack.ReleaseGet(app(c), release)
	if err != nil {
		return err
	}

	rn, err := rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
		Build: options.String(ro.Build),
		Env:   options.String(ro.Env),
	})
	if err != nil {
		return err
	}

	c.OK(rn.Id)

	c.Startf("Promoting <release>%s</release>", rn.Id)

	force := c.Bool("force")
	if err := rack.ReleasePromote(app(c), rn.Id, structs.ReleasePromoteOptions{
		Force: &force,
	}); err != nil {
		return err
	}

	c.Writef("\n")

	if err := common.WaitForAppWithLogs(rack, c, app(c)); err != nil {
		return err
	}

	a, err := rack.AppGet(app(c))
	if err != nil {
		return err
	}

	if a.Release != rn.Id {
		return fmt.Errorf("rollback")
	}

	if c.Bool("id") {
		fmt.Fprintf(stdout, rn.Id)
	}

	return c.OK()
}
