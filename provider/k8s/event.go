package k8s

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

func (p *Provider) EventSend(action string, opts structs.EventSendOptions) error {
	return errors.WithStack(fmt.Errorf("unimplemented"))
}
