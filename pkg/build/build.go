package build

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"

	"path/filepath"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/exec"
)

type Engine interface {
	Build(bb *Build, dir string) error
	Login(bb *Build) error
}

type Options struct {
	App         string
	Auth        string
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
	dir := os.TempDir()

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
