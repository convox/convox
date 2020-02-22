package console

import "github.com/convox/stdsdk"

type Rack struct {
	Name         string
	Organization Organization
	Provider     string
	Status       string
}

type Racks []Rack

func (c *Client) RackList() (Racks, error) {
	var rs Racks

	if err := c.Get("/racks", stdsdk.RequestOptions{}, &rs); err != nil {
		if _, ok := err.(AuthenticationError); ok {
			return nil, err
		}
	}

	return rs, nil
}
