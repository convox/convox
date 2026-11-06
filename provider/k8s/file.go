package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var allowedTarFlags = map[string]bool{
	"--no-same-owner":       true,
	"--no-same-permissions": true,
}

func isAllowedTarFlag(flag string) bool {
	if allowedTarFlags[flag] {
		return true
	}
	if strings.HasPrefix(flag, "--strip-components=") {
		v := strings.TrimPrefix(flag, "--strip-components=")
		if len(v) == 0 {
			return false
		}
		for _, c := range v {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func (p *Provider) FilesDelete(app, pid string, files []string) error {
	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	command := []string{"rm", "-f", "--"}
	command = append(command, files...)

	eo := &ac.PodExecOptions{
		Container: app,
		Command:   command,
		Stdout:    true,
	}

	req.VersionedParams(eo, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.Config, "POST", req.URL())
	if err != nil {
		return errors.WithStack(err)
	}

	if err := exec.StreamWithContext(p.ctx, remotecommand.StreamOptions{Stdout: io.Discard}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) FilesDownload(app, pid, file string) (io.Reader, error) {
	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	eo := &ac.PodExecOptions{
		Container: app,
		Command:   []string{"tar", "-cf", "-", file},
		Stdout:    true,
	}

	req.VersionedParams(eo, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.Config, "POST", req.URL())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	r, w := io.Pipe()

	go func() {
		exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{Stdout: w}) //nolint:errcheck // fire-and-forget stream; pipe EOF signals completion
		w.Close()
	}()

	return r, nil
}

func (p *Provider) FilesUpload(app, pid string, r io.Reader, opts structs.FileTransterOptions) error {
	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	cmd := []string{"tar"}
	if opts.TarExtraFlags != nil {
		flags := strings.Split(*opts.TarExtraFlags, ",")
		for _, f := range flags {
			if !isAllowedTarFlag(f) {
				return fmt.Errorf("unsupported tar flag: %s", f)
			}
		}
		cmd = append(cmd, flags...)
	}

	cmd = append(cmd, []string{"-C", "/", "-xf", "-"}...)

	eo := &ac.PodExecOptions{
		Container: app,
		Command:   cmd,
		Stdin:     true,
	}

	req.VersionedParams(eo, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.Config, "POST", req.URL())
	if err != nil {
		return errors.WithStack(err)
	}

	if err := exec.StreamWithContext(p.ctx, remotecommand.StreamOptions{Stdin: r}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
