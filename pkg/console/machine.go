package console

import (
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
)

func (c *Client) Machines() (structs.Machines, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Machines

	err = c.Get("/machines", ro, &v)

	return v, err
}
