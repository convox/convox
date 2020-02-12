package rack

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

var (
	TestClient sdk.Interface
)

type Test struct {
	name     string
	provider string
	status   string
}

func LoadTest(c *stdcli.Context, name string) (*Test, error) {
	return &Test{name: name}, nil
}

func (t Test) Client() (sdk.Interface, error) {
	return TestClient, nil
}

func (t Test) Name() string {
	return t.name
}

func (t Test) Provider() string {
	return "provider1"
}

func (t Test) Remote() bool {
	return false
}

func (t Test) Status() string {
	return "running"
}

func (t Test) Uninstall() error {
	return nil
}

func (t Test) Update(options map[string]string) error {
	return nil
}
