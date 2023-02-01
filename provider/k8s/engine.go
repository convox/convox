package k8s

import (
	"time"

	"github.com/convox/convox/pkg/manifest"
)

type Engine interface {
	AppIdles(app string) (bool, error)
	AppParameters() map[string]string
	Heartbeat() (map[string]interface{}, error)
	IngressAnnotations(certDuration string) (map[string]string, error)
	IngressClass() string
	Log(app, stream string, ts time.Time, message string) error
	ManifestValidate(m *manifest.Manifest) error
	RegistryAuth(host, username, password string) (string, string, error)
	RepositoryAuth(app string) (string, string, error)
	RepositoryHost(app string) (string, bool, error)
	RepositoryPrefix() string
	ResolverHost() (string, error)
	ServiceHost(app string, s manifest.Service) string
	SystemHost() string
	SystemStatus() (string, error)
}
