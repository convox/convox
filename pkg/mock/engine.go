package mock

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/pkg/errors"
)

type TestEngine struct {
}

func (tr *TestEngine) AppIdles(app string) (bool, error) {
	return false, nil
}

func (te *TestEngine) AppParameters() map[string]string {
	return map[string]string{"Test": "foo"}
}

func (te *TestEngine) Heartbeat() (map[string]interface{}, error) {
	return map[string]interface{}{"foo": "bar"}, nil
}

func (te *TestEngine) IngressAnnotations(app string) (map[string]string, error) {
	return map[string]string{"ann1": "val1"}, nil
}

func (te *TestEngine) IngressClass() string {
	return ""
}

func (te *TestEngine) Log(app, stream string, ts time.Time, message string) error {
	return nil
}

func (te *TestEngine) ManifestValidate(m *manifest.Manifest) error {
	return nil
}

func (te *TestEngine) RegistryAuth(host, username, password string) (string, string, error) {
	return username, password, nil
}

func (te *TestEngine) RepositoryAuth(app string) (string, string, error) {
	return "un1", "pw1", nil
}

func (te *TestEngine) RepositoryHost(app string) (string, bool, error) {
	return "repo1", true, nil
}

func (te *TestEngine) RepositoryPrefix() string {
	return ""
}

func (te *TestEngine) ResolverHost() (string, error) {
	return "", errors.WithStack(fmt.Errorf("no resolver"))
}

func (te *TestEngine) ServiceHost(app string, s manifest.Service) string {
	return "service.host"
}

func (te *TestEngine) SystemHost() string {
	return "system.host"
}

func (te *TestEngine) SystemStatus() (string, error) {
	return "amazing", nil
}
