package k8s

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/convox/convox/pkg/structs"
	"github.com/creack/pty"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ResourceConsole(app, name string, rw io.ReadWriter, opts structs.ResourceConsoleOptions) error {
	r, err := p.ResourceGet(app, name)
	if err != nil {
		return err
	}

	switch r.Type {
	case "postgres":
		return resourceConsoleCommand(rw, opts, "psql", r.Url)
	default:
		return fmt.Errorf("can not export resources of type: %s", r.Type)
	}
}

func resourceConsoleCommand(rw io.ReadWriter, opts structs.ResourceConsoleOptions, command string, args ...string) error {
	cmd := exec.Command(command, args...)

	size := &pty.Winsize{}

	if opts.Height != nil {
		size.Rows = uint16(*opts.Height)
	}

	if opts.Width != nil {
		size.Cols = uint16(*opts.Width)
	}

	fd, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return err
	}

	go io.Copy(fd, rw)
	io.Copy(rw, fd)

	return nil
}

func (p *Provider) ResourceExport(app, name string) (io.ReadCloser, error) {
	r, err := p.ResourceGet(app, name)
	if err != nil {
		return nil, err
	}

	switch r.Type {
	case "postgres":
		return resourceExportPostgres(r)
	default:
		return nil, fmt.Errorf("can not export resources of type: %s", r.Type)
	}
}

func resourceExportCommand(w io.WriteCloser, command string, args ...string) {
	defer w.Close()

	cmd := exec.Command(command, args...)

	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(w, "ERROR: could not export: %v\n", err)
	}
}

func resourceExportPostgres(r *structs.Resource) (io.ReadCloser, error) {
	rr, ww := io.Pipe()

	go resourceExportCommand(ww, "pg_dump", "--no-acl", "--no-owner", r.Url)

	return rr, nil
}

func (p *Provider) ResourceGet(app, name string) (*structs.Resource, error) {
	d, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Get(fmt.Sprintf("resource-%s", name), am.GetOptions{})
	if err != nil {
		return nil, err
	}

	cm, err := p.Cluster.CoreV1().ConfigMaps(p.AppNamespace(app)).Get(fmt.Sprintf("resource-%s", name), am.GetOptions{})
	if err != nil {
		return nil, err
	}

	status := "running"

	if d.Status.ReadyReplicas < 1 {
		status = "pending"
	}

	r := &structs.Resource{
		Name:   name,
		Status: status,
		Type:   d.ObjectMeta.Labels["kind"],
		Url:    cm.Data["URL"],
	}

	return r, nil
}

func (p *Provider) ResourceImport(app, name string, r io.Reader) error {
	rr, err := p.ResourceGet(app, name)
	if err != nil {
		return err
	}

	switch rr.Type {
	case "postgres":
		return resourceImportPostgres(rr, r)
	default:
		return fmt.Errorf("can not import resources of type: %s", rr.Type)
	}
}

func resourceImportPostgres(rr *structs.Resource, r io.Reader) error {
	cmd := exec.Command("psql", rr.Url)

	cmd.Stdin = r

	data, err := cmd.CombinedOutput()
	fmt.Printf("string(data): %+v\n", string(data))
	if err != nil {
		return fmt.Errorf("ERROR: import failed")
	}

	return nil
}

func (p *Provider) ResourceList(app string) (structs.Resources, error) {
	lopts := am.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s,type=resource", app),
	}

	ds, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).List(lopts)
	if err != nil {
		return nil, err
	}

	rs := structs.Resources{}

	for _, d := range ds.Items {
		r, err := p.ResourceGet(app, d.ObjectMeta.Labels["resource"])
		if err != nil {
			return nil, err
		}

		rs = append(rs, *r)
	}

	return rs, nil
}

func (p *Provider) SystemResourceCreate(kind string, opts structs.ResourceCreateOptions) (*structs.Resource, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceDelete(name string) error {
	return fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceGet(name string) (*structs.Resource, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceLink(name, app string) (*structs.Resource, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceList() (structs.Resources, error) {
	return structs.Resources{}, nil
}

func (p *Provider) SystemResourceTypes() (structs.ResourceTypes, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceUnlink(name, app string) (*structs.Resource, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemResourceUpdate(name string, opts structs.ResourceUpdateOptions) (*structs.Resource, error) {
	return nil, fmt.Errorf("unimplemented")
}
