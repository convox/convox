package k8s

import (
	"context"
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ServiceScaleOverrideAnnotation = "convox.com/scale-override-active"
	ServiceScaleOverrideValueOn    = "true"
)

func (p *Provider) ServiceScaleOverrideSet(app, service string, active bool, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)

	d, err := p.GetDeploymentFromInformer(service, p.AppNamespace(app))
	if err != nil {
		return errors.WithStack(err)
	}

	prevActive := false
	if d.Annotations != nil && d.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn {
		prevActive = true
	}

	if prevActive == active {
		return nil
	}

	var patch []byte
	if active {
		b, perr := patchBytes(map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					ServiceScaleOverrideAnnotation: ServiceScaleOverrideValueOn,
				},
			},
		})
		if perr != nil {
			return errors.WithStack(perr)
		}
		patch = b
	} else {
		patch = []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:null}}}`, ServiceScaleOverrideAnnotation))
	}

	if _, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Patch(
		context.TODO(),
		service,
		types.MergePatchType,
		patch,
		am.PatchOptions{},
	); err != nil {
		return errors.WithStack(err)
	}

	state := "off"
	if active {
		state = "on"
	}
	fmt.Printf("ns=k8s at=info kind=scale_override_toggled app=%s service=%s ack_by=%q prev=%t new=%t\n",
		app, service, ackBy, prevActive, active)
	_ = p.EventSend("app:scale-override:toggled", structs.EventSendOptions{
		Data: map[string]string{
			"actor":   ackBy,
			"ack_by":  ackBy,
			"app":     app,
			"service": service,
			"state":   state,
		},
	})

	return nil
}
