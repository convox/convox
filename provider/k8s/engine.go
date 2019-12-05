package k8s

import (
	"time"

	"github.com/convox/convox/pkg/manifest"
)

type Engine interface {
	AppIdles(app string) (bool, error)
	AppParameters() map[string]string
	AppStatus(app string) (string, error)
	Heartbeat() (map[string]interface{}, error)
	IngressAnnotations(app string) (map[string]string, error)
	IngressSecrets(app string) ([]string, error)
	Log(app, stream string, ts time.Time, message string) error
	ManifestValidate(m *manifest.Manifest) error
	RegistryAuth(host, username, password string) (string, string, error)
	RepositoryAuth(app string) (string, string, error)
	RepositoryHost(app string) (string, bool, error)
	ResolverHost() (string, error)
	ServiceHost(app string, s manifest.Service) string
	SystemHost() string
	SystemStatus() (string, error)
}
