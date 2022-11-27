package build

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	shellquote "github.com/kballard/go-shellquote"
)

type Docker struct{}

func (d *Docker) Build(bb *Build, dir string) error {
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

	benv, err := bb.buildEnvs()
	if err != nil {
		return err
	}

	for k, v := range benv {
		env[k] = v
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

		if bb.Push != "" {
			pushes[to] = fmt.Sprintf("%s:%s.%s", bb.Push, s.Name, bb.Id)
		}
	}

	for hash, b := range builds {
		bb.Printf("Building: %s\n", b.Path)

		if err := d.build(bb, filepath.Join(dir, b.Path), b.Manifest, hash, env); err != nil {
			return err
		}
	}

	for image := range pulls {
		if err := d.pull(bb, image); err != nil {
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
			if err := d.tag(bb, from, to); err != nil {
				return err
			}

			if bb.EnvWrapper {
				if err := d.injectConvoxEnv(bb, to); err != nil {
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

		if err := d.tag(bb, from, to); err != nil {
			return err
		}

		if err := d.push(bb, to); err != nil {
			return err
		}
	}

	return nil
}

func (*Docker) Login(bb *Build) error {
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

func (d *Docker) build(bb *Build, path, dockerfile, tag string, env map[string]string) error {
	if path == "" {
		return fmt.Errorf("must have path to build")
	}

	df := filepath.Join(path, dockerfile)

	args := []string{"build"}

	if !bb.Cache {
		args = append(args, "--no-cache")
	}

	args = append(args, "-t", tag)
	args = append(args, "-f", df)
	args = append(args, "--network", "host")

	ba, err := bb.buildArgs(df, env)
	if err != nil {
		return err
	}

	args = append(args, ba...)

	args = append(args, path)

	if bb.Terminal {
		if err := bb.Exec.Terminal("docker", args...); err != nil {
			return err
		}
	} else {
		if err := bb.Exec.Run(bb.writer, "docker", args...); err != nil {
			return err
		}
	}

	data, err := bb.Exec.Execute("docker", "inspect", tag, "--format", "{{json .Config.Entrypoint}}")
	if err != nil {
		return err
	}

	var ep []string

	if err := json.Unmarshal(data, &ep); err != nil {
		return err
	}

	if ep != nil {
		opts := structs.BuildUpdateOptions{
			Entrypoint: options.String(shellquote.Join(ep...)),
		}

		if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, opts); err != nil {
			return err
		}
	}

	return nil
}

func (bb *Build) buildArgs(dockerfile string, env map[string]string) ([]string, error) {
	fd, err := os.Open(dockerfile)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	s := bufio.NewScanner(fd)

	args := []string{}

	for s.Scan() {
		fields := strings.Fields(strings.TrimSpace(s.Text()))

		if len(fields) < 2 {
			continue
		}

		parts := strings.Split(fields[1], "=")

		switch fields[0] {
		case "FROM":
			if bb.Development && strings.Contains(strings.ToLower(s.Text()), "as development") {
				args = append(args, "--target", "development")
			}
		case "ARG":
			k := strings.TrimSpace(parts[0])
			if v, ok := env[k]; ok {
				args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return args, nil
}

func (*Docker) injectConvoxEnv(bb *Build, tag string) error {
	fmt.Fprintf(bb.writer, "Injecting: convox-env\n")

	var cmd []string
	var entrypoint []string

	data, err := bb.Exec.Execute("docker", "inspect", tag, "--format", "{{json .Config.Cmd}}")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}

	data, err = bb.Exec.Execute("docker", "inspect", tag, "--format", "{{json .Config.Entrypoint}}")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &entrypoint); err != nil {
		return err
	}

	epb, err := json.Marshal(append([]string{"/convox-env"}, entrypoint...))
	if err != nil {
		return err
	}

	epdfs := fmt.Sprintf("FROM %s\nCOPY ./convox-env /convox-env\nENTRYPOINT %s\n", tag, epb)

	if cmd != nil {
		cmdb, err := json.Marshal(cmd)
		if err != nil {
			return err
		}

		epdfs += fmt.Sprintf("CMD %s\n", cmdb)
	}

	tmp, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		return err
	}

	if _, err := bb.Exec.Execute("cp", "/go/bin/convox-env", filepath.Join(tmp, "convox-env")); err != nil {
		return err
	}

	epdf := filepath.Join(tmp, "Dockerfile")

	if err := os.WriteFile(epdf, []byte(epdfs), 0600); err != nil {
		return err
	}

	if _, err = bb.Exec.Execute("docker", "build", "-t", tag, tmp); err != nil {
		return err
	}

	return nil
}

func (*Docker) pull(bb *Build, tag string) error {
	fmt.Fprintf(bb.writer, "Running: docker pull %s\n", tag)

	data, err := bb.Exec.Execute("docker", "pull", tag)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}

func (*Docker) push(bb *Build, tag string) error {
	fmt.Fprintf(bb.writer, "Running: docker push %s\n", tag)

	data, err := bb.Exec.Execute("docker", "push", tag)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}

func (*Docker) tag(bb *Build, from, to string) error {
	fmt.Fprintf(bb.writer, "Running: docker tag %s %s\n", from, to)

	data, err := bb.Exec.Execute("docker", "tag", from, to)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}
