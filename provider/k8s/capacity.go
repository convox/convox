package k8s

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

func (p *Provider) CapacityGet() (*structs.Capacity, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}
