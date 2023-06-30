package sdk

import (
	"fmt"
	"io"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
)

func (c *Client) AppCancel(name string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Post(fmt.Sprintf("/apps/%s/cancel", name), ro, nil)

	return err
}

func (c *Client) AppCreate(name string, opts structs.AppCreateOptions) (*structs.App, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	ro.Params["name"] = name

	var v *structs.App

	err = c.Post("/apps", ro, &v)

	return v, err
}

func (c *Client) AppDelete(name string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/apps/%s", name), ro, nil)

	return err
}

func (c *Client) AppGet(name string) (*structs.App, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.App

	err = c.Get(fmt.Sprintf("/apps/%s", name), ro, &v)

	return v, err
}

func (c *Client) AppList() (structs.Apps, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Apps

	err = c.Get("/apps", ro, &v)

	return v, err
}

func (c *Client) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v io.ReadCloser

	r, err := c.Websocket(fmt.Sprintf("/apps/%s/logs", name), ro)
	if err != nil {
		return nil, err
	}

	v = r

	return v, err
}

func (c *Client) AppMetrics(name string, opts structs.MetricsOptions) (structs.Metrics, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Metrics

	err = c.Get(fmt.Sprintf("/apps/%s/metrics", name), ro, &v)

	return v, err
}

func (c *Client) AppUpdate(name string, opts structs.AppUpdateOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	err = c.Put(fmt.Sprintf("/apps/%s", name), ro, nil)

	return err
}

func (c *Client) BalancerList(app string) (structs.Balancers, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Balancers

	err = c.Get(fmt.Sprintf("/apps/%s/balancers", app), ro, &v)

	return v, err
}

func (c *Client) BuildCreate(app, url string, opts structs.BuildCreateOptions) (*structs.Build, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	ro.Params["url"] = url

	var v *structs.Build

	err = c.Post(fmt.Sprintf("/apps/%s/builds", app), ro, &v)

	return v, err
}

func (c *Client) BuildExport(app, id string, w io.Writer) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	res, err := c.GetStream(fmt.Sprintf("/apps/%s/builds/%s.tgz", app, id), ro)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if _, err := io.Copy(w, res.Body); err != nil {
		return err
	}

	return err
}

func (c *Client) BuildGet(app, id string) (*structs.Build, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Build

	err = c.Get(fmt.Sprintf("/apps/%s/builds/%s", app, id), ro, &v)

	return v, err
}

func (c *Client) BuildImport(app string, r io.Reader) (*structs.Build, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Body = r

	var v *structs.Build

	err = c.Post(fmt.Sprintf("/apps/%s/builds/import", app), ro, &v)

	return v, err
}

func (c *Client) BuildList(app string, opts structs.BuildListOptions) (structs.Builds, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Builds

	err = c.Get(fmt.Sprintf("/apps/%s/builds", app), ro, &v)

	return v, err
}

func (c *Client) BuildLogs(app, id string, opts structs.LogsOptions) (io.ReadCloser, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v io.ReadCloser

	r, err := c.Websocket(fmt.Sprintf("/apps/%s/builds/%s/logs", app, id), ro)
	if err != nil {
		return nil, err
	}

	v = r

	return v, err
}

func (c *Client) BuildUpdate(app, id string, opts structs.BuildUpdateOptions) (*structs.Build, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v *structs.Build

	err = c.Put(fmt.Sprintf("/apps/%s/builds/%s", app, id), ro, &v)

	return v, err
}

func (c *Client) CapacityGet() (*structs.Capacity, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Capacity

	err = c.Get("/system/capacity", ro, &v)

	return v, err
}

func (c *Client) CertificateApply(app, service string, port int, id string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Params["id"] = id

	err = c.Put(fmt.Sprintf("/apps/%s/ssl/%s/%d", app, service, port), ro, nil)

	return err
}

func (c *Client) CertificateCreate(pub, key string, opts structs.CertificateCreateOptions) (*structs.Certificate, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	ro.Params["pub"] = pub
	ro.Params["key"] = key

	var v *structs.Certificate

	err = c.Post("/certificates", ro, &v)

	return v, err
}

func (c *Client) CertificateDelete(id string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/certificates/%s", id), ro, nil)

	return err
}

func (c *Client) CertificateGenerate(domains []string) (*structs.Certificate, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Params["domains"] = strings.Join(domains, ",")

	var v *structs.Certificate

	err = c.Post("/certificates/generate", ro, &v)

	return v, err
}

func (c *Client) CertificateRenew(id string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Post(fmt.Sprintf("/certificates/%s/renew", id), ro, nil)

	return err
}

func (c *Client) CertificateList() (structs.Certificates, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Certificates

	err = c.Get("/certificates", ro, &v)

	return v, err
}

func (c *Client) EventSend(action string, opts structs.EventSendOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	ro.Params["action"] = action

	err = c.Post("/events", ro, nil)

	return err
}

func (c *Client) FilesDelete(app, pid string, files []string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Query["files"] = strings.Join(files, ",")

	err = c.Delete(fmt.Sprintf("/apps/%s/processes/%s/files", app, pid), ro, nil)

	return err
}

func (c *Client) FilesDownload(app, pid, file string) (io.Reader, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Query["file"] = file

	var v io.Reader

	res, err := c.GetStream(fmt.Sprintf("/apps/%s/processes/%s/files", app, pid), ro)
	if err != nil {
		return nil, err
	}

	v = res.Body

	return v, err
}

func (c *Client) FilesUpload(app, pid string, r io.Reader) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Body = r

	err = c.Post(fmt.Sprintf("/apps/%s/processes/%s/files", app, pid), ro, nil)

	return err
}

// skipcq
func (*Client) Initialize(opts structs.ProviderOptions) error {
	err := fmt.Errorf("not available via api")
	return err
}

func (c *Client) InstanceKeyroll() (*structs.KeyPair, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.KeyPair

	err = c.Post("/instances/keyroll", ro, &v)
	if err != nil && strings.Contains(err.Error(), "unexpected end") {
		// only v3 return the body for keyroll
		err = nil
	}

	return &v, err
}

func (c *Client) InstanceList() (structs.Instances, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Instances

	err = c.Get("/instances", ro, &v)

	return v, err
}

func (c *Client) InstanceShell(id string, rw io.ReadWriter, opts structs.InstanceShellOptions) (int, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return 0, err
	}

	ro.Body = rw

	var v int

	v, err = c.WebsocketExit(fmt.Sprintf("/instances/%s/shell", id), ro, rw)
	if err != nil {
		return 0, err
	}

	return v, err
}

func (c *Client) InstanceTerminate(id string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/instances/%s", id), ro, nil)

	return err
}

func (c *Client) ObjectDelete(app, key string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/apps/%s/objects/%s", app, key), ro, nil)

	return err
}

func (c *Client) ObjectExists(app, key string) (bool, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v bool

	err = c.Head(fmt.Sprintf("/apps/%s/objects/%s", app, key), ro, &v)

	return v, err
}

func (c *Client) ObjectFetch(app, key string) (io.ReadCloser, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v io.ReadCloser

	res, err := c.GetStream(fmt.Sprintf("/apps/%s/objects/%s", app, key), ro)
	if err != nil {
		return nil, err
	}

	v = res.Body

	return v, err
}

func (c *Client) ObjectList(app, prefix string) ([]string, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Query["prefix"] = prefix

	var v []string

	err = c.Get(fmt.Sprintf("/apps/%s/objects", app), ro, &v)

	return v, err
}

func (c *Client) ObjectStore(app, key string, r io.Reader, opts structs.ObjectStoreOptions) (*structs.Object, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	ro.Body = r

	var v *structs.Object

	err = c.Post(fmt.Sprintf("/apps/%s/objects/%s", app, key), ro, &v)

	return v, err
}

func (c *Client) ProcessExec(app, pid, command string, rw io.ReadWriter, opts structs.ProcessExecOptions) (int, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return 0, err
	}

	ro.Headers["command"] = command
	ro.Body = rw

	var v int

	v, err = c.WebsocketExit(fmt.Sprintf("/apps/%s/processes/%s/exec", app, pid), ro, rw)
	if err != nil {
		return 0, err
	}

	return v, err
}

func (c *Client) ProcessGet(app, pid string) (*structs.Process, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Process

	err = c.Get(fmt.Sprintf("/apps/%s/processes/%s", app, pid), ro, &v)

	return v, err
}

func (c *Client) ProcessList(app string, opts structs.ProcessListOptions) (structs.Processes, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Processes

	err = c.Get(fmt.Sprintf("/apps/%s/processes", app), ro, &v)

	return v, err
}

func (c *Client) ProcessLogs(app, pid string, opts structs.LogsOptions) (io.ReadCloser, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v io.ReadCloser

	r, err := c.Websocket(fmt.Sprintf("/apps/%s/processes/%s/logs", app, pid), ro)
	if err != nil {
		return nil, err
	}

	v = r

	return v, err
}

func (c *Client) ProcessRun(app, service string, opts structs.ProcessRunOptions) (*structs.Process, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v *structs.Process

	err = c.Post(fmt.Sprintf("/apps/%s/services/%s/processes", app, service), ro, &v)

	return v, err
}

func (c *Client) ProcessStop(app, pid string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/apps/%s/processes/%s", app, pid), ro, nil)

	return err
}

func (c *Client) Proxy(host string, port int, rw io.ReadWriter, opts structs.ProxyOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	ro.Body = rw

	r, err := c.Websocket(fmt.Sprintf("/proxy/%s/%d", host, port), ro)
	if err != nil {
		return err
	}

	if _, err := io.Copy(rw, r); err != nil {
		return err
	}

	return err
}

func (c *Client) RegistryAdd(server, username, password string) (*structs.Registry, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Params["server"] = server
	ro.Params["username"] = username
	ro.Params["password"] = password

	var v *structs.Registry

	err = c.Post("/registries", ro, &v)

	return v, err
}

func (c *Client) RegistryList() (structs.Registries, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Registries

	err = c.Get("/registries", ro, &v)

	return v, err
}

// skipcq
func (*Client) RegistryProxy(ctx *stdapi.Context) error {
	err := fmt.Errorf("not available via api")
	return err
}

func (c *Client) RegistryRemove(server string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/registries/%s", server), ro, nil)

	return err
}

func (c *Client) ReleaseCreate(app string, opts structs.ReleaseCreateOptions) (*structs.Release, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v *structs.Release

	err = c.Post(fmt.Sprintf("/apps/%s/releases", app), ro, &v)

	return v, err
}

func (c *Client) ReleaseGet(app, id string) (*structs.Release, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Release

	err = c.Get(fmt.Sprintf("/apps/%s/releases/%s", app, id), ro, &v)

	return v, err
}

func (c *Client) ReleaseList(app string, opts structs.ReleaseListOptions) (structs.Releases, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Releases

	err = c.Get(fmt.Sprintf("/apps/%s/releases", app), ro, &v)

	return v, err
}

func (c *Client) ReleasePromote(app, id string, opts structs.ReleasePromoteOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	err = c.Post(fmt.Sprintf("/apps/%s/releases/%s/promote", app, id), ro, nil)

	return err
}

func (c *Client) ResourceConsole(app, name string, rw io.ReadWriter, opts structs.ResourceConsoleOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	ro.Body = rw

	r, err := c.Websocket(fmt.Sprintf("/apps/%s/resources/%s/console", app, name), ro)
	if err != nil {
		return err
	}

	if _, err := io.Copy(rw, r); err != nil {
		return err
	}

	return err
}

func (c *Client) ResourceExport(app, name string) (io.ReadCloser, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v io.ReadCloser

	res, err := c.GetStream(fmt.Sprintf("/apps/%s/resources/%s/data", app, name), ro)
	if err != nil {
		return nil, err
	}

	v = res.Body

	return v, err
}

func (c *Client) ResourceGet(app, name string) (*structs.Resource, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Resource

	err = c.Get(fmt.Sprintf("/apps/%s/resources/%s", app, name), ro, &v)

	return v, err
}

func (c *Client) ResourceImport(app, name string, r io.Reader) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Body = r

	err = c.Put(fmt.Sprintf("/apps/%s/resources/%s/data", app, name), ro, nil)

	return err
}

func (c *Client) ResourceList(app string) (structs.Resources, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Resources

	err = c.Get(fmt.Sprintf("/apps/%s/resources", app), ro, &v)

	return v, err
}

func (c *Client) RackHost(rackOrgSlug string) (structs.RackData, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.RackData

	err = c.Get(fmt.Sprintf("/racks/%s/host", rackOrgSlug), ro, &v)

	return v, err
}

func (c *Client) Runtimes(rackOrgSlug string) (structs.Runtimes, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Runtimes

	err = c.Get(fmt.Sprintf("/racks/%s/runtimes", rackOrgSlug), ro, &v)

	return v, err
}

func (c *Client) RuntimeAttach(rackOrgSlug string, opts structs.RuntimeAttachOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	err = c.Put(fmt.Sprintf("/racks/%s/runtimes", rackOrgSlug), ro, nil)

	return err
}

func (c *Client) ServiceList(app string) (structs.Services, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Services

	err = c.Get(fmt.Sprintf("/apps/%s/services", app), ro, &v)

	return v, err
}

func (c *Client) ServiceMetrics(app, name string, opts structs.MetricsOptions) (structs.Metrics, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Metrics

	err = c.Get(fmt.Sprintf("/apps/%s/services/%s/metrics", app, name), ro, &v)

	return v, err
}

func (c *Client) ServiceRestart(app, name string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Post(fmt.Sprintf("/apps/%s/services/%s/restart", app, name), ro, nil)

	return err
}

func (c *Client) ServiceUpdate(app, name string, opts structs.ServiceUpdateOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	err = c.Put(fmt.Sprintf("/apps/%s/services/%s", app, name), ro, nil)

	return err
}

// skipcq
func (*Client) Start() error {
	err := fmt.Errorf("not available via api")
	return err
}

func (c *Client) SystemGet() (*structs.System, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.System

	err = c.Get("/system", ro, &v)

	return v, err
}

// skipcq
func (*Client) SystemInstall(w io.Writer, opts structs.SystemInstallOptions) (string, error) {
	err := fmt.Errorf("not available via api")
	return "", err
}

// skipcq
func (*Client) SystemJwtSignKey() (string, error) {
	err := fmt.Errorf("not available via api")
	return "", err
}

// skipcq
func (c *Client) SystemJwtSignKeyRotate() (string, error) {
	err := c.Put("/system/jwt/rotate", stdsdk.RequestOptions{}, nil)
	return "", err
}

// skipcq
func (c *Client) SystemJwtToken(opts structs.SystemJwtOptions) (*structs.SystemJwt, error) {
	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	v := &structs.SystemJwt{}
	err = c.Post("/system/jwt/token", ro, v)
	return v, err
}

func (c *Client) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v io.ReadCloser

	r, err := c.Websocket("/system/logs", ro)
	if err != nil {
		return nil, err
	}

	v = r

	return v, err
}

func (c *Client) SystemMetrics(opts structs.MetricsOptions) (structs.Metrics, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Metrics

	err = c.Get("/system/metrics", ro, &v)

	return v, err
}

func (c *Client) SystemProcesses(opts structs.SystemProcessesOptions) (structs.Processes, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v structs.Processes

	err = c.Get("/system/processes", ro, &v)

	return v, err
}

func (c *Client) SystemReleases() (structs.Releases, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Releases

	err = c.Get("/system/releases", ro, &v)

	return v, err
}

func (c *Client) SystemResourceCreate(kind string, opts structs.ResourceCreateOptions) (*structs.Resource, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	ro.Params["kind"] = kind

	var v *structs.Resource

	err = c.Post("/resources", ro, &v)

	return v, err
}

func (c *Client) SystemResourceDelete(name string) error {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	err = c.Delete(fmt.Sprintf("/resources/%s", name), ro, nil)

	return err
}

func (c *Client) SystemResourceGet(name string) (*structs.Resource, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Resource

	err = c.Get(fmt.Sprintf("/resources/%s", name), ro, &v)

	return v, err
}

func (c *Client) SystemResourceLink(name, app string) (*structs.Resource, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	ro.Params["app"] = app

	var v *structs.Resource

	err = c.Post(fmt.Sprintf("/resources/%s/links", name), ro, &v)

	return v, err
}

func (c *Client) SystemResourceList() (structs.Resources, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Resources

	err = c.Get("/resources", ro, &v)

	return v, err
}

func (c *Client) SystemResourceTypes() (structs.ResourceTypes, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.ResourceTypes

	err = c.Options("/resources", ro, &v)

	return v, err
}

func (c *Client) SystemResourceUnlink(name, app string) (*structs.Resource, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v *structs.Resource

	err = c.Delete(fmt.Sprintf("/resources/%s/links/%s", name, app), ro, &v)

	return v, err
}

func (c *Client) SystemResourceUpdate(name string, opts structs.ResourceUpdateOptions) (*structs.Resource, error) {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return nil, err
	}

	var v *structs.Resource

	err = c.Put(fmt.Sprintf("/resources/%s", name), ro, &v)

	return v, err
}

// skipcq
func (*Client) SystemUninstall(name string, w io.Writer, opts structs.SystemUninstallOptions) error {
	err := fmt.Errorf("not available via api")
	return err
}

func (c *Client) SystemUpdate(opts structs.SystemUpdateOptions) error {
	var err error

	ro, err := stdsdk.MarshalOptions(opts)
	if err != nil {
		return err
	}

	err = c.Put("/system", ro, nil)

	return err
}

// skipcq
func (*Client) Workers() error {
	err := fmt.Errorf("not available via api")
	return err
}
