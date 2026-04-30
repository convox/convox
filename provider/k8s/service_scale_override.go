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
	// ServiceScaleOverrideAnnotation is the annotation key written on
	// the service Deployment by ServiceScaleOverrideSet. When the value
	// is the literal string "true", releaseTemplateServices preserves
	// the runtime replica count and skips the yaml-declared
	// scale.count.min. Any other value (or absence) means the override
	// is inactive.
	ServiceScaleOverrideAnnotation = "convox.com/scale-override-active"

	// ServiceScaleOverrideValueOn is the literal annotation value that
	// activates the override. Strict equality only — variants like
	// "True", "yes", "1" are treated as inactive. See item-23 spec §6.
	ServiceScaleOverrideValueOn = "true"
)

// ServiceScaleOverrideSet toggles the per-service scale-override
// annotation on the service Deployment. When active=true, future
// release promotes preserve the runtime replica count instead of
// applying the yaml-declared scale.count. When active=false, the
// annotation is removed and yaml-declared scale resumes on next
// promote. Backed by the convox.com/scale-override-active=true
// annotation on the service Deployment metadata.
//
// The ackBy parameter carries actor identity for the audit event;
// the API layer derives it from JWT or a deprecated form-param via
// resolveAckByOverride. Mirrors the AppBudgetSet ack_by precedent.
//
// Sanitization: ackBy is normalized through sanitizeAckBy at entry —
// the provider is the canonical sanitization point per the API-layer
// doc at pkg/api/deprecation.go:43-46. Mirrors the budget_accumulator
// entry-point pattern; control chars / BiDi overrides / zero-width
// glyphs are stripped so they cannot stamp a misleading actor on the
// audit event or inject newlines into the stdout log.
func (p *Provider) ServiceScaleOverrideSet(app, service string, active bool, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)

	// Verify the service exists. NotFound here means the service is
	// not deployed yet (or was just deleted) — there's nothing to
	// toggle. Return a structured error so the API layer can map to
	// 404. This matches the existing ServiceUpdate pattern at
	// service.go:243 which also fails fast on missing Deployment.
	d, err := p.GetDeploymentFromInformer(service, p.AppNamespace(app))
	if err != nil {
		return errors.WithStack(err)
	}

	// Read existing state for the audit event payload.
	prevActive := false
	if d.Annotations != nil && d.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn {
		prevActive = true
	}

	// No-op when state already matches. Skip the patch + event so
	// retries / idempotent callers don't spam the audit stream.
	if prevActive == active {
		return nil
	}

	// Build the JSON merge-patch. When active=true: set the annotation
	// to the literal "true". When active=false: set the annotation
	// value to JSON null (k8s strategic merge-patch convention to
	// delete a key) — keeps the annotation surface minimal vs. a
	// stale "false" residue.
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

	// Audit event. Mirrors the AppBudgetSet event pattern — ack_by for
	// actor identity, prev/new pair for diff-friendly receivers.
	state := "off"
	if active {
		state = "on"
	}
	fmt.Printf("ns=k8s at=info kind=scale_override_toggled app=%s service=%s ack_by=%q prev=%t new=%t\n",
		app, service, ackBy, prevActive, active)
	// Event-name format: app:<resource>:<verb> (matches existing
	// app:budget:set, app:budget:cap, app:budget:threshold precedent).
	// Service identity carried in the payload, NOT embedded in the
	// event name — preserves grep/filter patterns keyed on the
	// 3-part colon-separated scheme (e.g. `convox events <app> | grep
	// 'app:scale-override:'` matches both -toggled and -honored).
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
