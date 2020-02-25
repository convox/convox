package console

import (
	"encoding/json"
	"fmt"

	"github.com/convox/stdsdk"
)

type Rack struct {
	Deletable    bool
	Name         string
	Organization Organization
	Parameters   map[string]string
	Provider     string
	Status       string
	State        []byte
}

type Racks []Rack

func (c *Client) RackCreate(name, provider string, state []byte, params map[string]string) (*Rack, error) {
	pdata, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	opts := stdsdk.RequestOptions{
		Params: stdsdk.Params{
			"name":     name,
			"params":   string(pdata),
			"provider": provider,
			"state":    string(state),
		},
	}

	var r Rack

	if err := c.Post("/racks", opts, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

func (c *Client) RackDelete(name string) error {
	if err := c.Delete(fmt.Sprintf("/racks/%s", name), stdsdk.RequestOptions{}, nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) RackGet(name string) (*Rack, error) {
	var r Rack

	if err := c.Get(fmt.Sprintf("/racks/%s", name), stdsdk.RequestOptions{}, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

func (c *Client) RackList() (Racks, error) {
	var rs Racks

	if err := c.Get("/racks", stdsdk.RequestOptions{}, &rs); err != nil {
		return nil, err
	}

	return rs, nil
}
