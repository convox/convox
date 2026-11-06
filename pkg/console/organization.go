package console

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
)

type Organization struct {
	Name string
}

func (c *Client) OrganizationRuntimes(org string) (structs.Runtimes, error) {
	var err error

	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}

	var v structs.Runtimes

	err = c.Get(fmt.Sprintf("/organizations/%s/runtimes", org), ro, &v)

	return v, err
}
