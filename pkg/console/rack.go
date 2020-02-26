package console

import (
	"encoding/json"

	"github.com/convox/stdsdk"
)

type Rack struct {
	Name         string
	Organization Organization
	Provider     string
	Status       string
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

func (c *Client) RackList() (Racks, error) {
	var rs Racks

	if err := c.Get("/racks", stdsdk.RequestOptions{}, &rs); err != nil {
		if _, ok := err.(AuthenticationError); ok {
			return nil, err
		}
	}

	return rs, nil
}
