package rack

import (
	"fmt"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/version"
)

type Direct struct {
	client   sdk.Interface
	name     string
	provider string
	status   string
}

func LoadDirect(client sdk.Interface) (*Direct, error) {
	dr := &Direct{
		client: client,
	}

	s, err := client.SystemGet()
	if err != nil {
		return nil, err
	}

	dr.name = s.Name
	dr.provider = s.Provider
	dr.status = s.Status

	return dr, nil
}

func (d Direct) Client() (sdk.Interface, error) {
	return d.client, nil
}

func (d Direct) Delete() error {
	return fmt.Errorf("can not delete a rack with RACK_URL")
}

func (d Direct) Metadata() (*Metadata, error) {
	return nil, fmt.Errorf("metadata not available with RACK_URL")
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

func (d Direct) UpdateParams(params map[string]string) error {
	cc, err := d.Client()
	if err != nil {
		return err
	}

	opts := structs.SystemUpdateOptions{
		Parameters: params,
	}

	if err := cc.SystemUpdate(opts); err != nil {
		return err
	}

	return nil
}

func (d Direct) UpdateVersion(version string) error {
	if version == "" {
		v, err := d.latest()
		if err != nil {
			return err
		}
		version = v
	}

	cc, err := d.Client()
	if err != nil {
		return err
	}

	opts := structs.SystemUpdateOptions{
		Version: options.String(version),
	}

	if err := cc.SystemUpdate(opts); err != nil {
		return err
	}

	return nil
}

func (d Direct) latest() (string, error) {
	s, err := d.client.SystemGet()
	if err != nil {
		return "", err
	}

	v, err := version.Next(s.Version)
	if err != nil {
		return "", err
	}

	return v, nil
}
