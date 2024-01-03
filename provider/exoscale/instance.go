package exoscale

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) InstanceKeyroll() (*structs.KeyPair, error) {
	return nil, fmt.Errorf("not supported")
}
