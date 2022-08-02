package k8s

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os/exec"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/creack/pty"
	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ResourceConsole(app, name string, rw io.ReadWriter, opts structs.ResourceConsoleOptions) error {
	r, err := p.ResourceGet(app, name)
	if err != nil {
		return errors.WithStack(err)
	}

	u, err := p.resourceUrl(app, name)
	if err != nil {
		return err
	}

	cn, err := parseResourceURL(u)
	if err != nil {
		return errors.WithStack(err)
	}

	switch r.Type {
	case "mariadb":
		return resourceConsoleCommand(rw, opts, "mysql", "-h", cn.Host, "-P", cn.Port, "-u", cn.Username, fmt.Sprintf("-p%s", cn.Password), "-D", cn.Database)
	case "memcached":
		return resourceConsoleCommand(rw, opts, "telnet", cn.Host, cn.Port)
	case "mysql":
		return resourceConsoleCommand(rw, opts, "mysql", "-h", cn.Host, "-P", cn.Port, "-u", cn.Username, fmt.Sprintf("-p%s", cn.Password), "-D", cn.Database)
	case "postgis":
		return resourceConsoleCommand(rw, opts, "psql", u)
	case "postgres":
		return resourceConsoleCommand(rw, opts, "psql", u)
	case "redis":
		return resourceConsoleCommand(rw, opts, "redis-cli", "-u", u)
	default:
		return errors.WithStack(fmt.Errorf("console not available for resources of type: %s", r.Type))
	}
}

func (p *Provider) ResourceExport(app, name string) (io.ReadCloser, error) {
	r, err := p.ResourceGet(app, name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	u, err := p.resourceUrl(app, name)
	if err != nil {
		return nil, err
	}

	switch r.Type {
	case "mariadb":
		return resourceExportMysql(u)
	case "mysql":
		return resourceExportMysql(u)
	case "postgis":
		return resourceExportPostgres(u)
	case "postgres":
		return resourceExportPostgres(u)
	default:
		return nil, errors.WithStack(fmt.Errorf("export not available for resources of type: %s", r.Type))
	}
}

func (p *Provider) ResourceGet(app, name string) (*structs.Resource, error) {
	m, _, err := common.AppManifest(p, app)
	if err != nil {
		return nil, err
	}

	mr, err := m.Resource(name)
	if err != nil {
		return nil, err
	}

	u, err := p.resourceUrl(app, name)
	if err != nil {
		return nil, err
	}

	r := &structs.Resource{
		Name:   name,
		Status: "external",
		Type:   mr.Type,
		Url:    u,
	}

	overlay, err := p.resourceOverlay(app, name)
	if err != nil {
		return nil, err
	}

	if overlay {
		return r, nil
	}

	d, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Get(context.TODO(), fmt.Sprintf("resource-%s", nameFilter(name)), am.GetOptions{})
	if err != nil {
		return nil, err
	}

	if d.Status.ReadyReplicas < 1 {
		r.Status = "pending"
	} else {
		r.Status = "running"
	}

	return r, nil
}

func (p *Provider) ResourceImport(app, name string, r io.Reader) error {
	rr, err := p.ResourceGet(app, name)
	if err != nil {
		return errors.WithStack(err)
	}

	switch rr.Type {
	case "mariadb":
		return resourceImportMysql(rr, r)
	case "mysql":
		return resourceImportMysql(rr, r)
	case "postgis":
		return resourceImportPostgres(rr, r)
	case "postgres":
		return resourceImportPostgres(rr, r)
	default:
		return errors.WithStack(fmt.Errorf("import not available for resources of type: %s", rr.Type))
	}
}

func (p *Provider) ResourceList(app string) (structs.Resources, error) {
	lopts := am.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s,type=resource", app),
	}

	ds, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).List(context.TODO(), lopts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rs := structs.Resources{}

	for _, d := range ds.Items {
		r, err := p.ResourceGet(app, d.ObjectMeta.Labels["resource"])
		if err != nil {
			return nil, errors.WithStack(err)
		}

		rs = append(rs, *r)
	}

	return rs, nil
}

func (p *Provider) SystemResourceCreate(kind string, opts structs.ResourceCreateOptions) (*structs.Resource, error) {
	switch kind {
	case "webhook":
		return p.systemResourceCreateWebhook(opts)
	default:
		return nil, fmt.Errorf("rack resource type unknown: %s", kind)
	}
}

func (p *Provider) systemResourceCreateWebhook(opts structs.ResourceCreateOptions) (*structs.Resource, error) {
	url, ok := opts.Parameters["Url"]
	if !ok {
		return nil, fmt.Errorf("parameter required: Url")
	}

	key := fmt.Sprintf("%s-%d", url, rand.Int63())
	name := common.DefaultString(opts.Name, fmt.Sprintf("webhook-%s", fmt.Sprintf("%x", sha256.Sum256([]byte(key)))[0:6]))

	if err := p.webhookCreate(name, url); err != nil {
		return nil, err
	}

	return p.SystemResourceGet(name)
}

func (p *Provider) SystemResourceDelete(name string) error {
	r, err := p.SystemResourceGet(name)
	if err != nil {
		return err
	}

	switch r.Type {
	case "webhook":
		return p.webhookDelete(r.Name)
	default:
		return fmt.Errorf("rack resource type unknown: %s", r.Type)
	}
}

func (p *Provider) SystemResourceGet(name string) (*structs.Resource, error) {
	rs, err := p.SystemResourceList()
	if err != nil {
		return nil, err
	}

	for _, r := range rs {
		if r.Name == name {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no such resource: %s", name)
}

func (p *Provider) SystemResourceLink(name, app string) (*structs.Resource, error) {
	return nil, errors.WithStack(fmt.Errorf("unavailable on v3 racks"))
}

func (p *Provider) SystemResourceList() (structs.Resources, error) {
	rs := structs.Resources{}

	ws, err := p.webhookList()
	if err != nil {
		return nil, err
	}

	for _, w := range ws {
		rs = append(rs, structs.Resource{
			Name: w.Name,
			Parameters: map[string]string{
				"Url": w.URL,
			},
			Status: "running",
			Type:   "webhook",
		})
	}

	return rs, nil
}

func (p *Provider) SystemResourceTypes() (structs.ResourceTypes, error) {
	rst := structs.ResourceTypes{
		structs.ResourceType{
			Name: "webhook",
			Parameters: structs.ResourceParameters{
				structs.ResourceParameter{
					Default:     "",
					Description: "url to which to post rack events",
					Name:        "Url",
				},
			},
		},
	}

	return rst, nil
}

func (p *Provider) SystemResourceUnlink(name, app string) (*structs.Resource, error) {
	return nil, errors.WithStack(fmt.Errorf("unavailable on v3 racks"))
}

func (p *Provider) SystemResourceUpdate(name string, opts structs.ResourceUpdateOptions) (*structs.Resource, error) {
	return nil, errors.WithStack(fmt.Errorf("unavailable on v3 racks"))
}

func (p *Provider) resourceOverlay(app, name string) (bool, error) {
	m, rel, err := common.AppManifest(p, app)
	if err != nil {
		return false, err
	}

	r, err := m.Resource(name)
	if err != nil {
		return false, err
	}

	env, err := structs.NewEnvironment([]byte(rel.Env))
	if err != nil {
		return false, err
	}

	return env[r.DefaultEnv()] != "", nil
}

func (p *Provider) resourceUrl(app, name string) (string, error) {
	cm, err := p.Cluster.CoreV1().ConfigMaps(p.AppNamespace(app)).Get(context.TODO(), fmt.Sprintf("resource-%s", nameFilter(name)), am.GetOptions{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return cm.Data["URL"], nil
}

type resourceConnection struct {
	Database string
	Host     string
	Password string
	Port     string
	Username string
}

func parseResourceURL(url_ string) (*resourceConnection, error) {
	u, err := url.Parse(url_)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cn := &resourceConnection{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Username: u.User.Username(),
	}

	if pw, ok := u.User.Password(); ok {
		cn.Password = pw
	}

	if len(u.Path) > 0 {
		cn.Database = u.Path[1:]
	}

	return cn, nil
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
		return errors.WithStack(err)
	}

	go io.Copy(fd, rw)
	io.Copy(rw, fd)

	return nil
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

func resourceExportMysql(url string) (io.ReadCloser, error) {
	cn, err := parseResourceURL(url)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rr, ww := io.Pipe()

	go resourceExportCommand(ww, "mysqldump", "-h", cn.Host, "-P", cn.Port, "-u", cn.Username, fmt.Sprintf("-p%s", cn.Password), cn.Database)

	return rr, nil
}

func resourceExportPostgres(url string) (io.ReadCloser, error) {
	rr, ww := io.Pipe()

	go resourceExportCommand(ww, "pg_dump", "--no-acl", "--no-owner", url)

	return rr, nil
}

func resourceImportMysql(rr *structs.Resource, r io.Reader) error {
	cn, err := parseResourceURL(rr.Url)
	if err != nil {
		return errors.WithStack(err)
	}

	cmd := exec.Command("mysql", "-h", cn.Host, "-P", cn.Port, "-u", cn.Username, fmt.Sprintf("-p%s", cn.Password), "-D", cn.Database)

	cmd.Stdin = r

	data, err := cmd.CombinedOutput()
	fmt.Printf("string(data): %+v\n", string(data))
	if err != nil {
		return errors.WithStack(fmt.Errorf("ERROR: import failed"))
	}

	return nil
}

func resourceImportPostgres(rr *structs.Resource, r io.Reader) error {
	cmd := exec.Command("psql", rr.Url)

	cmd.Stdin = r

	data, err := cmd.CombinedOutput()
	fmt.Printf("string(data): %+v\n", string(data))
	if err != nil {
		return errors.WithStack(fmt.Errorf("ERROR: import failed"))
	}

	return nil
}
