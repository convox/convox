package build

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	type build struct {
		Build manifest.ServiceBuild
		Image string
		Tag   string
	}

	var builds []build

	for i := range m.Services {
		b := build{
			Build: m.Services[i].Build,
			Image: m.Services[i].Image,
		}

		if bb.Push != "" {
			b.Tag = fmt.Sprintf("%s:%s.%s", bb.Push, m.Services[i].Name, bb.Id)
		}

		builds = append(builds, b)
	}

	for ix, build := range builds {
		if build.Image != "" {
			os.WriteFile(fmt.Sprintf("%s/Dockerfile.%d", dir, ix), []byte(fmt.Sprintf("FROM %s", build.Image)), 0600)

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

func (*BuildKit) Login(bb *Build) error {
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

	err = os.WriteFile(fmt.Sprintf("%s/.docker/config.json", os.Getenv("HOME")), f, 0600)
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to create registry credentials file - %s", err.Error()))
	}

	return nil
}

func (*BuildKit) cacheProvider(provider string) bool {
	return provider != "" && strings.Contains("do az", provider) // skipcq
}

func (*BuildKit) imageManifestCacheProvider(provider string) bool {
	if disable, err := strconv.ParseBool(os.Getenv("DISABLE_IMAGE_MANIFEST_CACHE")); err != nil || disable {
		// until this release: https://github.com/moby/buildkit/pull/4336
		return false
	}
	return provider != "" && strings.Contains("aws", provider) // skipcq
}

func (*BuildKit) buildArgs(development bool, dockerfile string, env map[string]string) ([]string, error) {
	fd, err := os.Open(dockerfile)
	if err != nil {
		return nil, err
	}
	defer fd.Close() // skipcq

	s := bufio.NewScanner(fd)

	var args []string

	for s.Scan() {
		fields := strings.Fields(strings.TrimSpace(s.Text()))

		if len(fields) < 2 {
			continue
		}

		parts := strings.Split(fields[1], "=")

		switch fields[0] {
		case "FROM":
			if development && strings.Contains(strings.ToLower(s.Text()), "as development") {
				args = append(args, "--opt", "target=development")
			}
		case "ARG":
			k := strings.TrimSpace(parts[0])
			if v, ok := env[k]; ok {
				args = append(args, "--opt", fmt.Sprintf("build-arg:%s=%s", k, v))
			}
		}
	}

	return args, nil
}

func (*BuildKit) entrypoint(bb *Build, tag string) []string {
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

// skipcq
func (bk *BuildKit) build(bb *Build, path, dockerfile, tag string, env map[string]string) error {
	if path == "" {
		return fmt.Errorf("must have path to build")
	}

	args := []string{"build"}
	args = append(args, "--frontend", "dockerfile.v0")                                // skipcq
	args = append(args, "--local", fmt.Sprintf("context=%s", path))                   // skipcq
	args = append(args, "--local", fmt.Sprintf("dockerfile=%s", path))                // skipcq
	args = append(args, "--opt", fmt.Sprintf("filename=%s", dockerfile))              // skipcq
	args = append(args, "--output", fmt.Sprintf("type=image,name=%s,push=true", tag)) // skipcq

	if bk.cacheProvider(os.Getenv("PROVIDER")) {
		reg := strings.Split(tag, ":")[0]
		args = append(args, "--export-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg)) // skipcq
		args = append(args, "--import-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg)) // skipcq
	} else if bk.imageManifestCacheProvider(os.Getenv("PROVIDER")) {
		reg := strings.Split(tag, ":")[0]
		args = append(args, "--export-cache", fmt.Sprintf("mode=max,image-manifest=true,oci-mediatypes=true,type=registry,ref=%s:buildcache", reg)) // skipcq
		args = append(args, "--import-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg))                                                  // skipcq
	} else {
		// keep a local cache for services using the same Dockerfile
		args = append(args, "--export-cache", "type=local,dest=/var/lib/buildkit") // skipcq
		args = append(args, "--import-cache", "type=local,src=/var/lib/buildkit")  // skipcq
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
