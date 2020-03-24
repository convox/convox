package k8s

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

func (p *Provider) BuildCreate(app, url string, opts structs.BuildCreateOptions) (*structs.Build, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, errors.WithStack(err)
	}

	b := structs.NewBuild(app)

	b.Description = common.DefaultString(opts.Description, "")
	b.Started = time.Now()

	if _, err := p.buildCreate(b); err != nil {
		return nil, errors.WithStack(err)
	}

	auth, err := p.buildAuth(b)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cache := common.DefaultBool(opts.NoCache, true)

	env := map[string]string{
		"BUILD_APP":         app,
		"BUILD_AUTH":        string(auth),
		"BUILD_DEVELOPMENT": fmt.Sprintf("%t", common.DefaultBool(opts.Development, false)),
		"BUILD_GENERATION":  "2",
		"BUILD_ID":          b.Id,
		"BUILD_MANIFEST":    common.DefaultString(opts.Manifest, "convox.yml"),
		"BUILD_RACK":        p.Name,
		"BUILD_URL":         url,
		"RACK_URL":          fmt.Sprintf("https://convox:%s@api.%s.svc.cluster.local:5443", p.Password, p.Namespace),
	}

	repo, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	env["BUILD_PUSH"] = repo

	ps, err := p.ProcessRun(app, "build", structs.ProcessRunOptions{
		Command:     options.String(fmt.Sprintf("build -method tgz -cache %t", cache)),
		Environment: env,
		Image:       options.String(p.Image),
		Volumes: map[string]string{
			p.Socket: "/var/run/docker.sock",
		},
	})
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

	services := []string{}

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

	for _, s := range m.Services {
		services = append(services, s.Name)
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

	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return errors.WithStack(err)
	}

	defer os.Remove(tmp)

	images := []string{}

	for _, service := range services {
		repo, remote, err := p.Engine.RepositoryHost(app)
		if err != nil {
			return errors.WithStack(err)
		}

		from := fmt.Sprintf("%s:%s.%s", repo, service, build.Id)

		if remote {
			if err := exec.Command("docker", "pull", from).Run(); err != nil {
				return errors.WithStack(err)
			}
		}

		to := fmt.Sprintf("registry:%s.%s", service, build.Id)

		if err := exec.Command("docker", "tag", from, to).Run(); err != nil {
			return errors.WithStack(err)
		}

		images = append(images, to)
	}

	name := fmt.Sprintf("%s.%s.tar", app, build.Id)
	file := filepath.Join(tmp, name)
	args := []string{"save", "-o", file}
	args = append(args, images...)

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return errors.WithStack(fmt.Errorf("%s: %s", strings.TrimSpace(string(out)), err.Error()))
	}

	defer os.Remove(file)

	stat, err := os.Stat(file)
	if err != nil {
		return errors.WithStack(err)
	}

	header := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0600,
		Size:     stat.Size(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return errors.WithStack(err)
	}

	fd, err := os.Open(file)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := io.Copy(tw, fd); err != nil {
		return errors.WithStack(err)
	}

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

	var manifest imageManifest

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

		if header.Name == "build.json" {
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			if err := json.Unmarshal(data, &source); err != nil {
				return nil, errors.WithStack(err)
			}

			target.Id = structs.NewBuild(app).Id
		}

		if strings.HasSuffix(header.Name, ".tar") {
			cmd := exec.Command("docker", "load")

			pr, pw := io.Pipe()
			tee := io.TeeReader(tr, pw)
			outb := &bytes.Buffer{}

			cmd.Stdin = pr
			cmd.Stdout = outb

			if err := cmd.Start(); err != nil {
				return nil, errors.WithStack(err)
			}

			manifest, err = extractImageManifest(tee)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			if err := pw.Close(); err != nil {
				return nil, errors.WithStack(err)
			}

			if err := cmd.Wait(); err != nil {
				out := strings.TrimSpace(outb.String())
				return nil, errors.WithStack(fmt.Errorf("%s: %s", out, err.Error()))
			}

			if len(manifest) == 0 {
				return nil, errors.WithStack(fmt.Errorf("invalid image manifest: no data"))
			}
		}
	}

	// TODO push if needed
	for _, tags := range manifest {
		for _, from := range tags.RepoTags {
			parts := strings.SplitN(from, ":", 2)
			if len(parts) != 2 {
				return nil, errors.WithStack(fmt.Errorf("invalid image manifest: invalid repo tag"))
			}

			serviceid := strings.Split(parts[1], ".")
			if len(serviceid) != 2 {
				return nil, errors.WithStack(fmt.Errorf("invalid image manifest: invalid repo tag"))
			}

			service := serviceid[0]

			repo, remote, err := p.Engine.RepositoryHost(app)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			to := fmt.Sprintf("%s:%s.%s", repo, service, target.Id)

			if err := exec.Command("docker", "tag", from, to).Run(); err != nil {
				return nil, errors.WithStack(err)
			}

			if remote {
				if err := exec.Command("docker", "push", to).Run(); err != nil {
					return nil, errors.WithStack(err)
				}
			}
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

func (p *Provider) buildGet(app, id string) (*structs.Build, error) {
	kb, err := p.Convox.ConvoxV1().Builds(p.AppNamespace(app)).Get(strings.ToLower(id), am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.buildUnmarshal(kb)
}

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

func (p *Provider) buildMarshal(b *structs.Build) *ca.Build {
	return &ca.Build{
		ObjectMeta: am.ObjectMeta{
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
