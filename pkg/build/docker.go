package build

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	shellquote "github.com/kballard/go-shellquote"
)

func (bb *Build) buildDocker(path, dockerfile string, tag string, env map[string]string) error {
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

func (bb *Build) cacheProvider(provider string) bool {
	return provider != "" && strings.Contains("do az", provider)
}

func (bb *Build) buildBuildKit(path, dockerfile string, tag string, env map[string]string) error {
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

	if bb.cacheProvider(os.Getenv("PROVIDER")) {
		reg := strings.Split(tag, ":")[0]
		args = append(args, "--export-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg))
		args = append(args, "--import-cache", fmt.Sprintf("type=registry,ref=%s:buildcache", reg))
	}

	println(strings.Join(args, " "))

	df := filepath.Join(path, dockerfile)

	ba, err := bb.buildArgs2(df, env)
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

	f, err := ioutil.ReadFile(df)
	if err != nil {
		return fmt.Errorf("could not open Dockerfile")
	}

	r := regexp.MustCompile(`(?i)entrypoint \[(?P<arr>.*)\]|entrypoint (?P<str>".*")`)
	groups := map[string]string{}
	gnames := r.SubexpNames()

	for ix, xp := range r.FindStringSubmatch(string(f)) {
		groups[gnames[ix]] = xp
	}

	ep := ""
	// join into a []string and quote according to sh rules
	if groups["str"] != "" {
		ep = groups["str"]
		ep = strings.ReplaceAll(ep, "\"", "")
		ep = shellquote.Join(strings.Split(ep, " ")...)
	} else if groups["arr"] != "" {
		ep = groups["arr"]
		ep = strings.ReplaceAll(ep, "\"", "")
		ep = shellquote.Join(strings.Split(ep, ",")...)
	}

	if ep != "" {
		opts := structs.BuildUpdateOptions{
			Entrypoint: options.String(ep),
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

func (bb *Build) buildArgs2(dockerfile string, env map[string]string) ([]string, error) {
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
				args = append(args, "--build-arg:", fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return args, nil
}

func (bb *Build) injectConvoxEnv(tag string) error {
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

	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	if _, err := bb.Exec.Execute("cp", "/go/bin/convox-env", filepath.Join(tmp, "convox-env")); err != nil {
		return err
	}

	epdf := filepath.Join(tmp, "Dockerfile")

	if err := ioutil.WriteFile(epdf, []byte(epdfs), 0644); err != nil {
		return err
	}

	if _, err = bb.Exec.Execute("docker", "build", "-t", tag, tmp); err != nil {
		return err
	}

	return nil
}

func (bb *Build) pull(tag string) error {
	fmt.Fprintf(bb.writer, "Running: docker pull %s\n", tag)

	data, err := bb.Exec.Execute("docker", "pull", tag)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}

func (bb *Build) push(tag string) error {
	fmt.Fprintf(bb.writer, "Running: docker push %s\n", tag)

	data, err := bb.Exec.Execute("docker", "push", tag)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}

func (bb *Build) tag(from, to string) error {
	fmt.Fprintf(bb.writer, "Running: docker tag %s %s\n", from, to)

	data, err := bb.Exec.Execute("docker", "tag", from, to)
	if err != nil {
		return errors.New(strings.TrimSpace(string(data)))
	}

	return nil
}
