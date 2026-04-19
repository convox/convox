package cli

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

const maskedEnvKeysConfigName = "cli-env-mask"

// envKeyPattern accepts any non-empty string without whitespace or '='.
// This matches the permissiveness of Environment.Load in pkg/structs/environment.go.
// A stricter POSIX regex would reject keys like "FOO.BAR" that `env set` accepts.
var envKeyPattern = regexp.MustCompile(`^[^\s=]+$`)

// containsControlChar rejects keys containing ASCII control chars (< 0x20 or 0x7F).
// Control chars pass the whitespace-and-equals regex above but, when printed through
// the CLI's display paths (`convox env mask`, `<info>%s</info>` wraps, masked `KEY=****`
// output), would render as terminal escape sequences. A user with app-write could
// poison the mask list with ANSI codes that another user's terminal would interpret
// on display. Reject at write time to close this.
func containsControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7F {
			return true
		}
	}
	return false
}

func init() {
	register("env", "list env vars", watch(Env), stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack, flagApp, flagWatchInterval,
			stdcli.StringFlag("release", "", "id of the release"),
			stdcli.BoolFlag("reveal", "", "show unmasked env values"),
		},
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("env edit", "edit env interactively", EnvEdit, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagRack,
			stdcli.BoolFlag("promote", "p", "promote the release"),
			stdcli.StringFlag("release", "", "id of the release"),
		},
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("env get", "get an env var", EnvGet, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack, flagApp,
			stdcli.StringFlag("release", "", "id of the release"),
		},
		Usage:    "<var>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("env set", "set env var(s)", EnvSet, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagId,
			flagRack,
			stdcli.BoolFlag("replace", "", "replace all environment variables with given ones"),
			stdcli.BoolFlag("promote", "p", "promote the release"),
			stdcli.StringFlag("release", "", "id of the release"),
		},
		Usage: "<key=value> [key=value]...",
	}, WithCloud())

	register("env unset", "unset env var(s)", EnvUnset, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagId,
			flagRack,
			stdcli.BoolFlag("promote", "p", "promote the release"),
			stdcli.StringFlag("release", "", "id of the release"),
		},
		Usage:    "<key> [key]...",
		Validate: stdcli.ArgsMin(1),
	}, WithCloud())

	register("env mask", "list masked env keys", EnvMask, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("env mask set", "mark env keys as masked", EnvMaskSet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "<key> [key]...",
		Validate: stdcli.ArgsMin(1),
	}, WithCloud())

	register("env mask unset", "unmark masked env keys", EnvMaskUnset, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "<key> [key]...",
		Validate: stdcli.ArgsMin(1),
	}, WithCloud())
}

func Env(rack sdk.Interface, c *stdcli.Context) error {
	releaseId := c.String("release")

	env, err := getEnvHelper(rack, app(c), releaseId)
	if err != nil {
		return err
	}

	// Short-circuit: skip masking path entirely when --reveal is set OR stdout is not a TTY.
	// Avoids an extra AppConfigGet call and keeps existing TestEnv mocks unchanged.
	if !c.Bool("reveal") && IsTerminalFn(c) {
		if masked := maskedKeysSet(rack, app(c)); masked != nil {
			_ = c.Writef("%s\n", env.StringMasked(masked))
			return nil
		}
	}

	_ = c.Writef("%s\n", env.String())

	return nil
}

func EnvEdit(rack sdk.Interface, c *stdcli.Context) error {
	releaseId := c.String("release")

	env, err := getEnvHelper(rack, app(c), releaseId)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	file := filepath.Join(tmp, fmt.Sprintf("%s.env", app(c)))

	fd, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	if _, err := fd.Write([]byte(env.String())); err != nil {
		return err
	}

	fd.Close()

	editor := "vi"

	if e := os.Getenv("EDITOR"); e != "" {
		editor = e
	}

	if err := c.Terminal(editor, file); err != nil {
		return err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	nenv := structs.Environment{}

	if err := nenv.Load(bytes.TrimSpace(data)); err != nil {
		return err
	}

	nks := []string{}

	for k := range nenv {
		nks = append(nks, fmt.Sprintf("<info>%s</info>", k))
	}

	sort.Strings(nks)

	c.Startf(fmt.Sprintf("Setting %s", strings.Join(nks, ", ")))

	var r *structs.Release

	rackVersion := ""
	if rack.ClientType() == "machine" {
		rackVersion = "v3"
	} else {
		s, err := rack.SystemGet()
		if err != nil {
			return err
		}
		rackVersion = s.Version
	}

	if rackVersion <= "20180708231844" {
		r, err = rack.EnvironmentSet(app(c), []byte(nenv.String()))
		if err != nil {
			return err
		}
	} else {
		r, err = rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
			Env:           options.String(nenv.String()),
			ParentRelease: options.StringOrNil(releaseId),
		})
		if err != nil {
			return err
		}
	}

	c.OK()

	c.Writef("Release: <release>%s</release>\n", r.Id)

	if c.Bool("promote") {
		if err := releasePromote(rack, c, app(c), r.Id, false); err != nil {
			return err
		}
	}

	return nil
}

func EnvGet(rack sdk.Interface, c *stdcli.Context) error {
	releaseId := c.String("release")
	env, err := getEnvHelper(rack, app(c), releaseId)
	if err != nil {
		return err
	}

	k := c.Arg(0)

	v, ok := env[k]
	if !ok {
		return fmt.Errorf("env not found: %s", k)
	}

	c.Writef("%s\n", v)

	return nil
}

func EnvSet(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	releaseId := c.String("release")

	env := structs.Environment{}
	var err error

	if !c.Bool("replace") {
		env, err = getEnvHelper(rack, app(c), releaseId)
		if err != nil {
			return err
		}
	}

	args := []string(c.Args)
	keys := []string{}

	if !c.Reader().IsTerminal() {
		s := bufio.NewScanner(c.Reader())
		for s.Scan() {
			args = append(args, s.Text())
		}
	}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			keys = append(keys, fmt.Sprintf("<info>%s</info>", parts[0]))
			env[parts[0]] = parts[1]
		}
	}

	sort.Strings(keys)

	c.Startf(fmt.Sprintf("Setting %s", strings.Join(keys, ", ")))

	var r *structs.Release

	rackVersion := ""
	if rack.ClientType() == "machine" {
		rackVersion = "v3"
	} else {
		s, err := rack.SystemGet()
		if err != nil {
			return err
		}
		rackVersion = s.Version
	}

	if rackVersion <= "20180708231844" {
		r, err = rack.EnvironmentSet(app(c), []byte(env.String()))
		if err != nil {
			return err
		}
	} else {
		r, err = rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
			Env:           options.String(env.String()),
			ParentRelease: options.StringOrNil(releaseId),
		})
		if err != nil {
			return err
		}
	}

	c.OK()

	c.Writef("Release: <release>%s</release>\n", r.Id)

	if c.Bool("promote") {
		if err := releasePromote(rack, c, app(c), r.Id, false); err != nil {
			return err
		}
	}

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", r.Id)
	}

	return nil
}

func EnvUnset(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	releaseId := c.String("release")

	env, err := getEnvHelper(rack, app(c), releaseId)
	if err != nil {
		return err
	}

	keys := []string{}

	for _, arg := range c.Args {
		keys = append(keys, fmt.Sprintf("<info>%s</info>", arg))
		delete(env, arg)
	}

	sort.Strings(keys)

	c.Startf(fmt.Sprintf("Unsetting %s", strings.Join(keys, ", ")))

	var r *structs.Release

	rackVersion := ""
	if rack.ClientType() == "machine" {
		rackVersion = "v3"
	} else {
		s, err := rack.SystemGet()
		if err != nil {
			return err
		}
		rackVersion = s.Version
	}

	if rackVersion <= "20180708231844" {
		for _, e := range c.Args {
			r, err = rack.EnvironmentUnset(app(c), e)
			if err != nil {
				return err
			}
		}
	} else {
		r, err = rack.ReleaseCreate(app(c), structs.ReleaseCreateOptions{
			Env:           options.String(env.String()),
			ParentRelease: options.StringOrNil(releaseId),
		})
		if err != nil {
			return err
		}
	}

	c.OK()

	c.Writef("Release: <release>%s</release>\n", r.Id)

	if c.Bool("promote") {
		if err := releasePromote(rack, c, app(c), r.Id, false); err != nil {
			return err
		}
	}

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", r.Id)
	}

	return nil
}

func getEnvHelper(rack sdk.Interface, appName, releaseId string) (structs.Environment, error) {
	if releaseId != "" {
		return common.AppEnvironmentForRelease(rack, appName, releaseId)
	}

	return common.AppEnvironment(rack, appName)
}

func EnvMask(rack sdk.Interface, c *stdcli.Context) error {
	keys, err := getMaskedEnvKeys(rack, app(c))
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		sort.Strings(keys)
		for _, k := range keys {
			_ = c.Writef("%s\n", k)
		}
	}

	return nil
}

func EnvMaskSet(rack sdk.Interface, c *stdcli.Context) error {
	for _, arg := range c.Args {
		if strings.Contains(arg, "=") {
			return fmt.Errorf("use `convox env set KEY=VALUE` to set env values. `convox env mask set` takes key names only")
		}
		if !envKeyPattern.MatchString(arg) {
			return fmt.Errorf("invalid env key name: %q (must not contain whitespace or '=')", arg)
		}
		if containsControlChar(arg) {
			return fmt.Errorf("invalid env key name: %q (must not contain control characters)", arg)
		}
	}

	existing, _ := getMaskedEnvKeys(rack, app(c))

	keySet := map[string]bool{}
	for _, k := range existing {
		keySet[k] = true
	}
	for _, k := range c.Args {
		keySet[k] = true
	}

	if len(keySet) > 500 {
		return fmt.Errorf("too many masked keys (max 500, got %d)", len(keySet))
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	display := make([]string, 0, len(c.Args))
	seen := map[string]bool{}
	for _, k := range c.Args {
		if seen[k] {
			continue
		}
		seen[k] = true
		display = append(display, fmt.Sprintf("<info>%s</info>", k))
	}
	sort.Strings(display)
	c.Startf("Setting masked env keys %s", strings.Join(display, ", "))

	value := strings.Join(keys, ",")
	encoded := base64.StdEncoding.EncodeToString([]byte(value))

	if err := rack.AppConfigSet(app(c), maskedEnvKeysConfigName, encoded); err != nil {
		if isAppConfigUnsupported(err) {
			_ = c.Writef("\nWarning: this rack version may not support env masking\n")
			//nolint:nilerr // intentional: old rack warning is user-friendly exit-0, not an error
			return nil
		}
		return err
	}

	cfg, err := rack.AppConfigGet(app(c), maskedEnvKeysConfigName)
	if err != nil || cfg == nil || strings.TrimSpace(cfg.Value) != value {
		_ = c.Writef("\nWarning: this rack version may not support env masking\n")
		//nolint:nilerr // intentional: unverified write warning, exit-0 is user-friendly
		return nil
	}

	return c.OK()
}

func EnvMaskUnset(rack sdk.Interface, c *stdcli.Context) error {
	existing, _ := getMaskedEnvKeys(rack, app(c))

	remove := map[string]bool{}
	for _, k := range c.Args {
		remove[k] = true
	}

	keys := []string{}
	for _, k := range existing {
		if !remove[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	display := make([]string, 0, len(c.Args))
	seen := map[string]bool{}
	for _, k := range c.Args {
		if seen[k] {
			continue
		}
		seen[k] = true
		display = append(display, fmt.Sprintf("<info>%s</info>", k))
	}
	sort.Strings(display)
	c.Startf("Unsetting masked env keys %s", strings.Join(display, ", "))

	value := strings.Join(keys, ",")
	encoded := base64.StdEncoding.EncodeToString([]byte(value))

	if err := rack.AppConfigSet(app(c), maskedEnvKeysConfigName, encoded); err != nil {
		if isAppConfigUnsupported(err) {
			_ = c.Writef("\nWarning: this rack version may not support env masking\n")
			//nolint:nilerr // intentional: old rack warning is user-friendly exit-0, not an error
			return nil
		}
		return err
	}

	cfg, err := rack.AppConfigGet(app(c), maskedEnvKeysConfigName)
	if err != nil || cfg == nil || strings.TrimSpace(cfg.Value) != value {
		_ = c.Writef("\nWarning: this rack version may not support env masking\n")
		//nolint:nilerr // intentional: unverified write warning, exit-0 is user-friendly
		return nil
	}

	return c.OK()
}

// getMaskedEnvKeys reads the mask list from AppConfig. Returns empty slice on any error
// (old racks, missing config, transient failures) — never surfaces an error to the caller.
func getMaskedEnvKeys(rack sdk.Interface, appName string) ([]string, error) {
	cfg, err := rack.AppConfigGet(appName, maskedEnvKeysConfigName)
	if err != nil || cfg == nil {
		//nolint:nilerr // intentional: swallow read errors; absent mask list = no masking
		return nil, nil
	}

	trimmed := strings.TrimSpace(cfg.Value)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out, nil
}

// maskedKeysSet converts the mask list to a lookup map. Returns nil (not empty map)
// when the list is empty — callers use `if masked := maskedKeysSet(...); masked != nil`
// to branch on "masking configured" vs "no masking".
func maskedKeysSet(rack sdk.Interface, appName string) map[string]bool {
	keys, _ := getMaskedEnvKeys(rack, appName)
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}

// isAppConfigUnsupported detects errors from racks that don't have the AppConfig
// endpoint (V2 racks, V3 pre-3.19.7). The SDK flattens HTTP errors to plain Go
// errors.
//
// We match two specific substrings only:
//   - "response status 404" — stdsdk's fallback string when the server returns
//     a bare 404 with no body (route truly missing on V2 / old V3).
//   - "app config not found" — the provider-level error (pkg/structs ErrNotFound)
//     when the endpoint exists but the config key doesn't. Safe to treat as
//     "unsupported" in the write path: if the rack supports AppConfig, the first
//     write creates the key; getting this error on a SET is odd but benign.
//
// We DO NOT match bare "404" or bare "not found" because those substrings appear
// in errors we must NOT swallow — notably `namespaces "my-app" not found` when the
// user mistypes the app name. Hiding that behind a version-warning would mislead.
func isAppConfigUnsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "response status 404") ||
		strings.Contains(msg, "app config not found")
}
