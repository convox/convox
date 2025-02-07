package k8s

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func (p *Provider) FilesDelete(app, pid string, files []string) error {
	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	command := []string{"rm", "-f"}
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

	if err := exec.Stream(remotecommand.StreamOptions{Stdout: ioutil.Discard}); err != nil {
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
		exec.Stream(remotecommand.StreamOptions{Stdout: w})
		w.Close()
	}()

	return r, nil
}

func (p *Provider) FilesUpload(app, pid string, r io.Reader, opts structs.FileTransterOptions) error {
	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	cmd := []string{"tar"}
	if opts.TarExtraFlags != nil {
		cmd = append(cmd, strings.Split(*opts.TarExtraFlags, ",")...)
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

	if err := exec.Stream(remotecommand.StreamOptions{Stdin: r}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
