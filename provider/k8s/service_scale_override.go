package k8s

import (
	"context"
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ServiceScaleOverrideAnnotation = "convox.com/scale-override-active"
	ServiceScaleOverrideValueOn    = "true"
)

func (p *Provider) ServiceScaleOverrideSet(app, service string, active bool, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)
	ns := p.AppNamespace(app)

	prevActive := false
	stateful := false
	s, err := p.Cluster.AppsV1().StatefulSets(ns).Get(context.TODO(), service, am.GetOptions{})
	switch {
	case err == nil:
		stateful = true
		prevActive = s.Annotations != nil && s.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn
	case !kerr.IsNotFound(err):
		return errors.WithStack(err)
	default:
		d, err := p.GetDeploymentFromInformer(service, ns)
		if err != nil {
			return errors.WithStack(err)
		}
		prevActive = d.Annotations != nil && d.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn
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

	if stateful {
		if _, err := p.Cluster.AppsV1().StatefulSets(ns).Patch(context.TODO(), service, types.MergePatchType, patch, am.PatchOptions{}); err != nil {
			return errors.WithStack(err)
		}
	} else {
		if _, err := p.Cluster.AppsV1().Deployments(ns).Patch(context.TODO(), service, types.MergePatchType, patch, am.PatchOptions{}); err != nil {
			return errors.WithStack(err)
		}
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
