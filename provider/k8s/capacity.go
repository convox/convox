package k8s

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) CapacityGet() (*structs.Capacity, error) {
	return nil, fmt.Errorf("unimplemented")
}
