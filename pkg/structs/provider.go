package structs

import (
	context "context"
	"io"

	"github.com/convox/stdapi"
)

type Provider interface {
	Initialize(opts ProviderOptions) error
	Start() error

	AppCancel(name string) error
	AppCreate(name string, opts AppCreateOptions) (*App, error)
	AppGet(name string) (*App, error)
	AppDelete(name string) error
	AppList() (Apps, error)
	AppLogs(name string, opts LogsOptions) (io.ReadCloser, error)
	AppMetrics(name string, opts MetricsOptions) (Metrics, error)
	AppUpdate(name string, opts AppUpdateOptions) error

	BalancerList(app string) (Balancers, error)

	BuildCreate(app, url string, opts BuildCreateOptions) (*Build, error)
	BuildExport(app, id string, w io.Writer) error
	BuildGet(app, id string) (*Build, error)
	BuildImport(app string, r io.Reader) (*Build, error)
	BuildLogs(app, id string, opts LogsOptions) (io.ReadCloser, error)
	BuildList(app string, opts BuildListOptions) (Builds, error)
	BuildUpdate(app, id string, opts BuildUpdateOptions) (*Build, error)

	CapacityGet() (*Capacity, error)

	CertificateApply(app, service string, port int, id string) error
	CertificateCreate(pub, key string, opts CertificateCreateOptions) (*Certificate, error)
	CertificateDelete(id string) error
	CertificateRenew(id string) error
	CertificateGenerate(domains []string, opts CertificateGenerateOptions) (*Certificate, error)
	CertificateList(opts CertificateListOptions) (Certificates, error)

	LetsEncryptConfigGet() (*LetsEncryptConfig, error)
	LetsEncryptConfigApply(config LetsEncryptConfig) error

	EventSend(action string, opts EventSendOptions) error

	FilesDelete(app, pid string, files []string) error
	FilesDownload(app, pid string, file string) (io.Reader, error)
	FilesUpload(app, pid string, r io.Reader, opts FileTransterOptions) error

	InstanceKeyroll() (*KeyPair, error)
	InstanceList() (Instances, error)
	InstanceShell(id string, rw io.ReadWriter, opts InstanceShellOptions) (int, error)
	InstanceTerminate(id string) error

	ObjectDelete(app, key string) error
	ObjectExists(app, key string) (bool, error)
	ObjectFetch(app, key string) (io.ReadCloser, error)
	ObjectList(app, prefix string) ([]string, error)
	ObjectStore(app, key string, r io.Reader, opts ObjectStoreOptions) (*Object, error)

	ProcessExec(app, pid, command string, rw io.ReadWriter, opts ProcessExecOptions) (int, error)
	ProcessGet(app, pid string) (*Process, error)
	ProcessList(app string, opts ProcessListOptions) (Processes, error)
	ProcessLogs(app, pid string, opts LogsOptions) (io.ReadCloser, error)
	ProcessRun(app, service string, opts ProcessRunOptions) (*Process, error)
	ProcessStop(app, pid string) error

	Proxy(host string, port int, rw io.ReadWriter, opts ProxyOptions) error

	RegistryAdd(server, username, password string) (*Registry, error)
	RegistryList() (Registries, error)
	RegistryProxy(ctx *stdapi.Context) error
	RegistryRemove(server string) error

	ReleaseCreate(app string, opts ReleaseCreateOptions) (*Release, error)
	ReleaseGet(app, id string) (*Release, error)
	ReleaseList(app string, opts ReleaseListOptions) (Releases, error)
	ReleasePromote(app, id string, opts ReleasePromoteOptions) error

	ResourceConsole(app, name string, rw io.ReadWriter, opts ResourceConsoleOptions) error
	ResourceExport(app, name string) (io.ReadCloser, error)
	ResourceGet(app, name string) (*Resource, error)
	ResourceImport(app, name string, r io.Reader) error
	ResourceList(app string) (Resources, error)

	ServiceList(app string) (Services, error)
	ServiceRestart(app, name string) error
	ServiceUpdate(app, name string, opts ServiceUpdateOptions) error
	ServiceLogs(app, name string, opts LogsOptions) (io.ReadCloser, error)

	SystemGet() (*System, error)
	SystemInstall(w io.Writer, opts SystemInstallOptions) (string, error)
	SystemJwtSignKey() (string, error)
	SystemJwtSignKeyRotate() (string, error)
	SystemLogs(opts LogsOptions) (io.ReadCloser, error)
	SystemMetrics(opts MetricsOptions) (Metrics, error)
	SystemProcesses(opts SystemProcessesOptions) (Processes, error)
	SystemReleases() (Releases, error)
	SystemUninstall(name string, w io.Writer, opts SystemUninstallOptions) error
	SystemUpdate(opts SystemUpdateOptions) error

	SystemResourceCreate(kind string, opts ResourceCreateOptions) (*Resource, error)
	SystemResourceDelete(name string) error
	SystemResourceGet(name string) (*Resource, error)
	SystemResourceLink(name, app string) (*Resource, error)
	SystemResourceList() (Resources, error)
	SystemResourceTypes() (ResourceTypes, error)
	SystemResourceUnlink(name, app string) (*Resource, error)
	SystemResourceUpdate(name string, opts ResourceUpdateOptions) (*Resource, error)

	WithContext(ctx context.Context) Provider

	Workers() error
}

type ProviderOptions struct {
	Logs                io.Writer
	IgnorePriorityClass bool
}
