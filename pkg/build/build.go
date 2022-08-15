package build

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"sort"
	"strings"

	"path/filepath"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/exec"
	"github.com/pkg/errors"
)

type Engine interface {
	Build(bb *Build, dir string) error
	Login(bb *Build) error
}

type Options struct {
	App         string
	Auth        string
	BuildArgs   []string
	Cache       bool
	Development bool
	EnvWrapper  bool
	Id          string
	Manifest    string
	Output      io.Writer
	Push        string
	Rack        string
	Source      string
	Terminal    bool
}

type Build struct {
	Options
	Exec     exec.Interface
	Provider structs.Provider
	Engine   Engine
	logs     bytes.Buffer
	writer   io.Writer
}

func New(rack structs.Provider, opts Options, engine Engine) (*Build, error) {
	b := &Build{Options: opts}

	b.Exec = &exec.Exec{}

	b.Engine = engine

	b.Manifest = common.CoalesceString(b.Manifest, "convox.yml")

	b.Provider = rack

	b.logs = bytes.Buffer{}

	if opts.Output != nil {
		b.writer = io.MultiWriter(opts.Output, &b.logs)
	} else {
		b.writer = io.MultiWriter(os.Stdout, &b.logs)
	}

	return b, nil
}

func (bb *Build) Execute() error {
	if err := bb.execute(); err != nil {
		return bb.fail(err)
	}

	return nil
}

func (bb *Build) Printf(format string, args ...interface{}) {
	fmt.Fprintf(bb.writer, format, args...)
}

func (bb *Build) execute() error {
	if _, err := bb.Provider.BuildGet(bb.App, bb.Id); err != nil {
		return err
	}

	if err := bb.Engine.Login(bb); err != nil {
		return err
	}

	dir, err := bb.prepareSource()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filepath.Join(dir, bb.Manifest))
	if err != nil {
		return err
	}

	if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, structs.BuildUpdateOptions{Manifest: options.String(string(data))}); err != nil {
		return err
	}

	// bb.Engine.Build
	if err := bb.Engine.Build(bb, dir); err != nil {
		return err
	}

	if err := bb.success(); err != nil {
		return err
	}

	return nil
}

func (bb *Build) prepareSource() (string, error) {
	u, err := url.Parse(bb.Source)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "dir":
		return u.Path, nil
	case "object":
		return bb.prepareSourceObject(u.Host, u.Path)
	default:
		return "", fmt.Errorf("unknown source type")
	}
}

func (bb *Build) prepareSourceObject(app, key string) (string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if err := os.Chdir(dir); err != nil {
		return "", err
	}
	defer os.Chdir(cwd)

	r, err := bb.Provider.ObjectFetch(app, key)
	if err != nil {
		return "", err
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}

	if err := common.Unarchive(gz, "."); err != nil {
		return "", err
	}

	return dir, nil
}

func (bb *Build) login() error {
	var auth map[string]struct {
		Username string
		Password string
	}

	if err := json.Unmarshal([]byte(bb.Auth), &auth); err != nil {
		return err
	}

	for host, entry := range auth {
		buf := &bytes.Buffer{}

		err := bb.Exec.Stream(buf, strings.NewReader(entry.Password), "docker", "login", "-u", entry.Username, "--password-stdin", host)

		bb.Printf("Authenticating %s: %s\n", host, strings.TrimSpace(buf.String()))

		if err != nil {
			return err
		}
	}

	return nil
}

func (bb *Build) login2() error {
	var registries map[string]struct {
		Username string
		Password string
	}

	type auth struct {
		Auth string `json:"auth"`
	}

	type authConfig struct {
		Auths map[string]auth
	}

	if err := json.Unmarshal([]byte(bb.Auth), &registries); err != nil {
		return err
	}

	ac := authConfig{Auths: make(map[string]auth)}
	for host, entry := range registries {
		ac.Auths[host] = auth{
			Auth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", entry.Username, entry.Password))),
		}
	}

	f, err := json.Marshal(ac)
	if err != nil {
		return err
	}

	home := os.Getenv("HOME")
	err = os.WriteFile(fmt.Sprintf("%s/.docker/config.json", home), f, 0755)
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to create registry credentials file - %s", err.Error()))
	}

	return nil
}

func (bb *Build) build2(dir string) error {
	config := filepath.Join(dir, bb.Manifest)

	if _, err := os.Stat(config); os.IsNotExist(err) {
		return fmt.Errorf("no such file: %s", bb.Manifest)
	}

	data, err := ioutil.ReadFile(config)
	if err != nil {
		return err
	}

	env, err := common.AppEnvironment(bb.Provider, bb.App)
	if err != nil {
		return err
	}

	m, err := manifest.Load(data, env)
	if err != nil {
		return err
	}

	if err := m.Validate(); err != nil {
		return err
	}

	type build struct {
		Build manifest.ServiceBuild
		Image string
		Tag   string
	}

	builds := []build{}

	for _, s := range m.Services {
		b := build{
			Build: s.Build,
			Image: s.Image,
		}

		if bb.Push != "" {
			b.Tag = fmt.Sprintf("%s:%s.%s", bb.Push, s.Name, bb.Id)
		}

		builds = append(builds, b)
	}

	for ix, build := range builds {
		if build.Image != "" {
			os.WriteFile(fmt.Sprintf("%s/Dockerfile.%d", dir, ix), []byte(fmt.Sprintf("FROM %s", build.Image)), 0755)

			if err := bb.buildBuildKit(dir, fmt.Sprintf("Dockerfile.%d", ix), build.Tag, env); err != nil {
				return err
			}
		} else {
			if err := bb.buildBuildKit(filepath.Join(dir, build.Build.Path), build.Build.Manifest, build.Tag, env); err != nil {
				return err
			}
		}
	}

	return nil
}

func (bb *Build) build(dir string) error {
	config := filepath.Join(dir, bb.Manifest)

	if _, err := os.Stat(config); os.IsNotExist(err) {
		return fmt.Errorf("no such file: %s", bb.Manifest)
	}

	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	env, err := common.AppEnvironment(bb.Provider, bb.App)
	if err != nil {
		return err
	}

	for _, v := range bb.BuildArgs {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid build args: %s", v)
		}
		env[parts[0]] = parts[1]
	}

	m, err := manifest.Load(data, env)
	if err != nil {
		return err
	}

	if err := m.Validate(); err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s/%s", bb.Rack, bb.App)

	builds := map[string]manifest.ServiceBuild{}
	pulls := map[string]bool{}
	pushes := map[string]string{}
	tags := map[string][]string{}

	type build struct {
		Build manifest.ServiceBuild
		Image string
		Tag   string
	}

	all := []build{}

	for _, s := range m.Services {
		hash := s.BuildHash(bb.Id)
		to := fmt.Sprintf("%s:%s.%s", prefix, s.Name, bb.Id)

		if s.Image != "" {
			pulls[s.Image] = true
			tags[s.Image] = append(tags[s.Image], to)
		} else {
			builds[hash] = s.Build
			tags[hash] = append(tags[hash], to)
		}

		b := build{
			Build: s.Build,
			Image: s.Image,
		}

		if bb.Push != "" {
			pushes[to] = fmt.Sprintf("%s:%s.%s", bb.Push, s.Name, bb.Id)
			b.Tag = fmt.Sprintf("%s:%s.%s", bb.Push, s.Name, bb.Id)
		}

		all = append(all, b)
	}

	for hash, b := range builds {
		bb.Printf("Building: %s\n", b.Path)

		if err := bb.buildDocker(filepath.Join(dir, b.Path), b.Manifest, hash, env); err != nil {
			return err
		}
	}

	for image := range pulls {
		if err := bb.pull(image); err != nil {
			return err
		}
	}

	tagfroms := []string{}

	for from := range tags {
		tagfroms = append(tagfroms, from)
	}

	sort.Strings(tagfroms)

	for _, from := range tagfroms {
		tos := tags[from]

		for _, to := range tos {
			if err := bb.tag(from, to); err != nil {
				return err
			}

			if bb.EnvWrapper {
				if err := bb.injectConvoxEnv(to); err != nil {
					return err
				}
			}
		}
	}

	pushfroms := []string{}

	for from := range pushes {
		pushfroms = append(pushfroms, from)
	}

	sort.Strings(pushfroms)

	for _, from := range pushfroms {
		to := pushes[from]

		if err := bb.tag(from, to); err != nil {
			return err
		}

		if err := bb.push(to); err != nil {
			return err
		}
	}

	return nil
}

func (bb *Build) success() error {
	logs, err := bb.Provider.ObjectStore(bb.App, fmt.Sprintf("build/%s/logs", bb.Id), bytes.NewReader(bb.logs.Bytes()), structs.ObjectStoreOptions{})
	if err != nil {
		return err
	}

	opts := structs.BuildUpdateOptions{
		Ended: options.Time(time.Now().UTC()),
		Logs:  options.String(logs.Url),
	}

	if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, opts); err != nil {
		return err
	}

	r, err := bb.Provider.ReleaseCreate(bb.App, structs.ReleaseCreateOptions{Build: options.String(bb.Id)})
	if err != nil {
		return err
	}

	opts = structs.BuildUpdateOptions{
		Release: options.String(r.Id),
		Status:  options.String("complete"),
	}

	if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, opts); err != nil {
		return err
	}

	bb.Provider.EventSend("build:create", structs.EventSendOptions{Data: map[string]string{"app": bb.App, "id": bb.Id, "release_id": r.Id}})

	return nil
}

func (bb *Build) fail(buildError error) error {
	bb.Printf("ERROR: %s\n", buildError)

	bb.Provider.EventSend("build:create", structs.EventSendOptions{Data: map[string]string{"app": bb.App, "id": bb.Id}, Error: options.String(buildError.Error())})

	logs, err := bb.Provider.ObjectStore(bb.App, fmt.Sprintf("build/%s/logs", bb.Id), bytes.NewReader(bb.logs.Bytes()), structs.ObjectStoreOptions{})
	if err != nil {
		return err
	}

	opts := structs.BuildUpdateOptions{
		Ended:  options.Time(time.Now().UTC()),
		Logs:   options.String(logs.Url),
		Status: options.String("failed"),
	}

	if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, opts); err != nil {
		return err
	}

	return buildError
}
