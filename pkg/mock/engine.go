package mock

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/pkg/errors"
)

type TestEngine struct {
}

func (*TestEngine) AppIdles(_ string) (bool, error) {
	return false, nil
}

func (*TestEngine) AppParameters() map[string]string {
	return map[string]string{"Test": "foo"}
}

func (*TestEngine) Heartbeat() (map[string]interface{}, error) {
	return map[string]interface{}{"foo": "bar"}, nil
}

func (*TestEngine) IngressAnnotations(_ string) (map[string]string, error) {
	return map[string]string{"ann1": "val1"}, nil
}

func (*TestEngine) IngressClass() string {
	return ""
}

func (*TestEngine) IngressInternalClass() string {
	return ""
}

func (*TestEngine) Log(_, _ string, _ time.Time, _ string) error {
	return nil
}

func (*TestEngine) ManifestValidate(_ *manifest.Manifest) error {
	return nil
}

func (*TestEngine) RegistryAuth(_, username, password string) (string, string, error) {
	return username, password, nil
}

func (*TestEngine) RepositoryAuth(_ string) (string, string, error) {
	return "un1", "pw1", nil
}

func (*TestEngine) RepositoryHost(_ string) (string, bool, error) {
	return "repo1", true, nil
}

func (*TestEngine) RepositoryPrefix() string {
	return ""
}

func (*TestEngine) ResolverHost() (string, error) {
	return "", errors.WithStack(fmt.Errorf("no resolver"))
}

func (*TestEngine) ServiceHost(_ string, _ manifest.Service) string {
	return "service.host"
}

func (*TestEngine) SystemHost() string {
	return "system.host"
}

func (*TestEngine) SystemStatus() (string, error) {
	return "amazing", nil
}
