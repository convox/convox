package rack

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/convox/version"
)

type Direct struct {
	ctx      *stdcli.Context
	endpoint string
	name     string
	provider string
	status   string
}

func LoadDirect(c *stdcli.Context, endpoint string) (*Direct, error) {
	dr := &Direct{
		ctx:      c,
		endpoint: endpoint,
	}

	cc, err := dr.Client()
	if err != nil {
		return nil, err
	}

	s, err := cc.SystemGet()
	if err != nil {
		return nil, err
	}

	dr.name = s.Name
	dr.provider = s.Provider
	dr.status = s.Status

	return dr, nil
}

func (d Direct) Client() (sdk.Interface, error) {
	return sdk.New(d.endpoint)
}

func (d Direct) Name() string {
	return d.name
}

func (d Direct) Parameters() (map[string]string, error) {
	cc, err := d.Client()
	if err != nil {
		return nil, err
	}

	s, err := cc.SystemGet()
	if err != nil {
		return nil, err
	}

	return s.Parameters, nil
}

func (d Direct) Provider() string {
	return d.provider
}

func (d Direct) Remote() bool {
	return true
}

func (d Direct) Status() string {
	return d.status
}

func (d Direct) Uninstall() error {
	return fmt.Errorf("uninstall not supported with RACK_URL")
}

func (d Direct) Update(opts map[string]string) error {
	uopts := structs.SystemUpdateOptions{}

	if v, ok := opts["release"]; ok {
		if strings.TrimSpace(v) == "" {
			latest, err := version.Latest()
			if err != nil {
				return err
			}

			v = latest
		}

		uopts.Version = options.String(v)
	}

	delete(opts, "release")

	if len(opts) > 0 {
		uopts.Parameters = opts
	}

	cc, err := d.Client()
	if err != nil {
		return err
	}

	if err := cc.SystemUpdate(uopts); err != nil {
		return err
	}

	return nil
}
