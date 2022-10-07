package build

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
)

type BuildKit struct{}

func (bk *BuildKit) Build(bb *Build, dir string) error {
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

			if err := bk.build(bb, dir, fmt.Sprintf("Dockerfile.%d", ix), build.Tag, env); err != nil {
				return err
			}
		} else {
			if err := bk.build(bb, filepath.Join(dir, build.Build.Path), build.Build.Manifest, build.Tag, env); err != nil {
				return err
			}
		}
	}

	return nil
}

func (bk *BuildKit) Login(bb *Build) error {
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

	err = os.WriteFile(fmt.Sprintf("%s/.docker/config.json", os.Getenv("HOME")), f, 0755)
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to create registry credentials file - %s", err.Error()))
	}

	return nil
}

func (bk *BuildKit) cacheProvider(provider string) bool {
	return provider != "" && strings.Contains("do az", provider)
}

func (bk *BuildKit) buildArgs(development bool, dockerfile string, env map[string]string) ([]string, error) {
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
			if development && strings.Contains(strings.ToLower(s.Text()), "as development") {
				args = append(args, "--target", "development")
			}
		case "ARG":
			k := strings.TrimSpace(parts[0])
			if v, ok := env[k]; ok {
				args = append(args, "--build-arg:", fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return args, nil
}

func (bk BuildKit) entrypoint(bb *Build, tag string) []string {
	data, err := bb.Exec.Execute("skopeo", "inspect", "--config", fmt.Sprintf("docker://%s", tag))
	if err != nil {
		fmt.Fprint(bb.writer, "failed to retrieve image entrypoint", "-", err.Error())
		return nil
	}

	inspect := struct {
		Config struct {
			Entrypoint []string
		}
	}{}

	err = json.Unmarshal(data, &inspect)
	if err != nil {
		fmt.Fprint(bb.writer, "failed to retrieve image entrypoint", "-", err.Error())
		return nil
	}

	return inspect.Config.Entrypoint
}

func (bk *BuildKit) build(bb *Build, path, dockerfile string, tag string, env map[string]string) error {
	if path == "" {
		return fmt.Errorf("must have path to build")
	}

	// buildctl build --frontend dockerfile.v0 --local context=. --local dockerfile=. --opt filename=Dockerfile --export-cache type=inline --import-cache type=registry,ref=4707781
	// 23668.dkr.ecr.us-east-1.amazonaws.com/dev11/nodejs --output type=image,name=470778123668.dkr.ecr.us-east-1.amazonaws.com/dev11/nodejs,push=true
	args := []string{"build"}
	args = append(args, "--frontend", "dockerfile.v0")
	args = append(args, "--local", fmt.Sprintf("context=%s", path))
	args = append(args, "--local", fmt.Sprintf("dockerfile=%s", path))
	args = append(args, "--opt", fmt.Sprintf("filename=%s", dockerfile))
	args = append(args, "--output", fmt.Sprintf("type=image,name=%s,push=true", tag))

	if bk.cacheProvider(os.Getenv("PROVIDER")) {
		reg := strings.Split(tag, ":")[0]
		args = append(args, "--export-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg))
		args = append(args, "--import-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg))
	}

	df := filepath.Join(path, dockerfile)

	ba, err := bk.buildArgs(bb.Development, df, env)
	if err != nil {
		return err
	}

	args = append(args, ba...)

	if !bb.Cache {
		args = append(args, "--no-cache")
	}

	if bb.Terminal {
		if err := bb.Exec.Terminal("buildctl", args...); err != nil {
			return err
		}
	} else {
		if err := bb.Exec.Run(bb.writer, "buildctl", args...); err != nil {
			return err
		}
	}

	ep := bk.entrypoint(bb, tag)
	if len(ep) > 0 {
		opts := structs.BuildUpdateOptions{
			Entrypoint: options.String(shellquote.Join(ep...)),
		}

		if _, err := bb.Provider.BuildUpdate(bb.App, bb.Id, opts); err != nil {
			return err
		}
	}

	return nil
}
