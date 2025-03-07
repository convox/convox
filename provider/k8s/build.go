package k8s

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	"github.com/pkg/errors"
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
		"BUILD_APP":                    app,
		"BUILD_AUTH":                   string(auth),
		"BUILD_DEVELOPMENT":            fmt.Sprintf("%t", common.DefaultBool(opts.Development, false)),
		"BUILD_GENERATION":             "2",
		"BUILD_ID":                     b.Id,
		"BUILD_MANIFEST":               common.DefaultString(opts.Manifest, "convox.yml"),
		"BUILD_RACK":                   p.Name,
		"BUILD_URL":                    url,
		"BUILDKIT_ENABLED":             p.BuildkitEnabled,
		"PROVIDER":                     os.Getenv("PROVIDER"),
		"DISABLE_IMAGE_MANIFEST_CACHE": os.Getenv("DISABLE_IMAGE_MANIFEST_CACHE"),
		"RACK_URL":                     fmt.Sprintf("https://convox:%s@api.%s.svc.cluster.local:5443", p.Password, p.Namespace),
	}

	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	env["BUILD_PUSH"] = repo

	buildCmd := fmt.Sprintf("build -method tgz -cache %t", cache)
	if opts.BuildArgs != nil {
		for _, v := range *opts.BuildArgs {
			if len(strings.SplitN(v, "=", 2)) != 2 {
				return nil, errors.New("invalid build args:" + v)
			}
			buildCmd = fmt.Sprintf("%s -build-args %s", buildCmd, v)
		}
	}

	psOpts := structs.ProcessRunOptions{
		Command:     options.String(buildCmd),
		Cpu:         options.Int(512),
		Environment: env,
	}

	if nlbs := appObj.Parameters[structs.AppParamBuildLabels]; nlbs != "" {
		psOpts.NodeLabels = options.String(nlbs)
	}

	if cpu := appObj.Parameters[structs.AppParamBuildCpu]; cpu != "" {
		v, err := strconv.ParseInt(cpu, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid build cpu: %s, err: %s", cpu, err)
		}
		psOpts.Cpu = options.Int(int(v))
	}

	if mem := appObj.Parameters[structs.AppParamBuildMem]; mem != "" {
		v, err := strconv.ParseInt(mem, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid build mem: %s, err: %s", mem, err)
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
		return errors.WithStack(fmt.Errorf("no services found to export"))
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
		return p.ProcessLogs(app, b.Process, opts)
	default:
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
}

func (p *Provider) BuildList(app string, opts structs.BuildListOptions) (structs.Builds, error) {
	_, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	bs, err := p.buildList(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sort.Slice(bs, func(i, j int) bool { return bs[i].Started.After(bs[j].Started) })

	if limit := common.DefaultInt(opts.Limit, 10); len(bs) > limit {
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
	kb, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(app)).Get(strings.ToLower(id), am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.buildUnmarshal(kb)
}

// skipcq
func (p *Provider) buildList(app string) (structs.Builds, error) {
	kbs, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(app)).List(am.ListOptions{})
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
		Release:     kb.Spec.Release,
		Started:     started,
		Status:      kb.Spec.Status,
	}

	return b, nil
}

// skipcq
func (p *Provider) buildUpdate(b *structs.Build) (*structs.Build, error) {
	kbo, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(b.App)).Get(strings.ToLower(b.Id), am.GetOptions{})
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
