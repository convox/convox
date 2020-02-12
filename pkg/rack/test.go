package rack

import (
	"strings"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
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

func (t Test) Parameters() (map[string]string, error) {
	cc, err := t.Client()
	if err != nil {
		return nil, err
	}

	if cc == nil {
		return map[string]string{}, nil
	}

	s, err := cc.SystemGet()
	if err != nil {
		return nil, err
	}

	return s.Parameters, nil
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

func (t Test) Update(opts map[string]string) error {
	if TestClient == nil {
		return nil
	}

	uopts := structs.SystemUpdateOptions{}

	if v, ok := opts["release"]; ok {
		if strings.TrimSpace(v) == "" {
			v = "latest"
		}

		uopts.Version = options.String(v)
	}

	delete(opts, "release")

	if len(opts) > 0 {
		uopts.Parameters = opts
	}

	cc, err := t.Client()
	if err != nil {
		return err
	}

	if err := cc.SystemUpdate(uopts); err != nil {
		return err
	}

	return nil
}
