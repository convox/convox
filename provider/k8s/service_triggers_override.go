package k8s

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// ServiceTriggersOverrideAnnotation marks a service Deployment as
	// having a Console-driven autoscale configuration that the deploy
	// controller must NOT overwrite from the manifest. Mirrors the
	// ServiceScaleOverrideAnnotation pattern for replica counts.
	ServiceTriggersOverrideAnnotation = "convox.com/triggers-override-active"

	// ServiceTriggersOverrideCRDAnnotation records which CRD owns the
	// Console-driven autoscaler so the Disable path knows which resource
	// to tear down without re-probing both surfaces.
	ServiceTriggersOverrideCRDAnnotation = "convox.com/triggers-override-crd"

	// ServiceTriggersOverrideValueOn is the literal annotation value
	// that activates the override. Strict equality only.
	ServiceTriggersOverrideValueOn = "true"

	TriggersCRDHPA  = "hpa"
	TriggersCRDKeda = "keda"
)

// triggersCRDChoice returns "keda" when any requested trigger requires
// KEDA (gpuUtilization or queueDepth); "hpa" otherwise. Empty input
// returns "hpa" — the caller's validation rejects empty trigger sets
// before reaching this function.
func triggersCRDChoice(triggers []structs.TriggerSpec) string {
	for _, t := range triggers {
		if t.Type == structs.TriggerTypeGPUUtilization || t.Type == structs.TriggerTypeQueueDepth {
			return TriggersCRDKeda
		}
	}
	return TriggersCRDHPA
}

// ServiceTriggersEnable materializes a Console-driven autoscaler for the
// service: validates inputs, runs preflight (GPU manifest reservation +
// KEDA availability), dispatches to the HPA or KEDA branch based on the
// requested trigger mix, then sets the override annotations on the
// service Deployment. The annotation patch is the last write so that on
// any failure the rack is left in a consistent state — best-effort
// rollback of the just-created CRD prevents an "orphan CRD with no
// annotation" surface invisible to the Disable path.
//
// CRD dispatch:
//   - All triggers in {cpu, memory} → native autoscaling/v2 HPA. KEDA
//     is not required.
//   - Any trigger in {gpuUtilization, queueDepth} → KEDA ScaledObject.
//     Rejected at preflight when keda_enable=false on the rack.
func (p *Provider) ServiceTriggersEnable(app, service string, opts structs.ServiceTriggersOptions, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)

	if err := opts.Validate(); err != nil {
		return err
	}

	for _, t := range opts.Triggers {
		if t.Type != structs.TriggerTypeGPUUtilization {
			continue
		}
		gpuCount, err := p.serviceGPUCount(app, service)
		if err != nil {
			return err
		}
		if gpuCount < 1 {
			return fmt.Errorf("GPU autoscale requires the service to declare scale.gpu.count >= 1 in convox.yml")
		}
	}

	crdChoice := triggersCRDChoice(opts.Triggers)

	if crdChoice == TriggersCRDKeda && !p.IsKedaEnabled {
		return fmt.Errorf("GPU and queue-depth triggers require KEDA. Run convox rack params set keda_enable=true on the rack and re-deploy, then try again")
	}

	// HPA-path scale-to-zero requires the K8s HPAScaleToZero feature
	// gate (alpha; not enabled on EKS or most managed K8s). Reject
	// Min=0 on the HPA path with a friendly error pointing users at a
	// KEDA-eligible trigger so the K8s API server doesn't surface a
	// bare HorizontalPodAutoscaler validation error in the toast.
	if crdChoice == TriggersCRDHPA && opts.Min < 1 {
		return fmt.Errorf("CPU and memory triggers require min >= 1 on this rack. Scale-to-zero (min=0) needs the Kubernetes HPAScaleToZero feature gate, which is alpha and is not enabled on EKS or most managed clusters. To get scale-to-zero, either add a KEDA-eligible trigger (gpuUtilization or queueDepth — these use the KEDA ScaledObject path which supports min=0 natively) or set min >= 1 here and rely on natural CPU/memory scaling")
	}

	ns := p.AppNamespace(app)
	d, err := p.Cluster.AppsV1().Deployments(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return errors.WithStack(fmt.Errorf("service not found: %s/%s", app, service))
		}
		return errors.WithStack(err)
	}

	prevCRD := ""
	if d.Annotations != nil {
		prevCRD = d.Annotations[ServiceTriggersOverrideCRDAnnotation]
	}
	switch {
	case prevCRD != "" && prevCRD != crdChoice:
		// Cross-CRD-type override switch. Delete the previous CRD
		// before creating the new one so the new SO/HPA is the sole
		// autoscaler owning the Deployment.
		switch prevCRD {
		case TriggersCRDHPA:
			delErr := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
			if delErr != nil && !kerr.IsNotFound(delErr) && !meta.IsNoMatchError(delErr) {
				return errors.WithStack(delErr)
			}
		case TriggersCRDKeda:
			delErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
			if delErr != nil && !kerr.IsNotFound(delErr) && !meta.IsNoMatchError(delErr) {
				return errors.WithStack(delErr)
			}
		}
	case prevCRD == "":
		// No prior override, but a manifest-materialized CRD of the
		// opposite type may exist (e.g. service has scale.autoscale.cpu
		// on a KEDA rack → SO exists; user enables HPA-path override).
		// Delete the opposite-type CRD so the new override is the sole
		// owner. Best-effort: NotFound / NoMatch are expected and
		// silently absorbed.
		if crdChoice == TriggersCRDHPA {
			_ = p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		} else {
			_ = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		}
	}

	switch crdChoice {
	case TriggersCRDHPA:
		if err := p.applyTriggersHPA(ns, service, opts); err != nil {
			return errors.WithStack(err)
		}
	case TriggersCRDKeda:
		if err := p.applyTriggersKEDA(app, ns, service, opts); err != nil {
			return errors.WithStack(err)
		}
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:%q,%q:%q}}}`,
		ServiceTriggersOverrideAnnotation, ServiceTriggersOverrideValueOn,
		ServiceTriggersOverrideCRDAnnotation, crdChoice)
	if _, err := p.Cluster.AppsV1().Deployments(ns).Patch(
		context.TODO(), service, types.StrategicMergePatchType,
		[]byte(patch), am.PatchOptions{}); err != nil {
		// Best-effort rollback: delete the just-created CRD so the
		// rack does not leak an orphan that the Disable path cannot
		// see (annotation missing → idempotent no-op skips deletion).
		switch crdChoice {
		case TriggersCRDHPA:
			_ = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		case TriggersCRDKeda:
			_ = p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		}
		return errors.WithStack(err)
	}

	fmt.Printf("ns=service at=triggers-override-enable app=%s service=%s crd=%s min=%d max=%d ack_by=%q\n",
		app, service, crdChoice, opts.Min, opts.Max, ackBy)

	_ = p.EventSend("app:triggers-override:toggled", structs.EventSendOptions{
		Data: map[string]string{
			"app":     app,
			"service": service,
			"state":   "on",
			"crd":     crdChoice,
			"actor":   ackBy,
			"ack_by":  ackBy,
		},
	})

	return nil
}

// applyTriggersHPA creates or in-place updates a native K8s HPA owning
// the named service Deployment with one metrics entry per requested
// CPU/memory trigger. Non-CPU/memory triggers are silently skipped (the
// KEDA branch catches GPU/queue triggers earlier in the dispatch).
//
// Update preserves labels, annotations, and Spec.Behavior on the existing
// HPA — only the override-owned fields (ScaleTargetRef, MinReplicas,
// MaxReplicas, Metrics) are replaced. This avoids wiping manifest-set
// metadata when the Console takes ownership of an existing HPA.
func (p *Provider) applyTriggersHPA(ns, service string, opts structs.ServiceTriggersOptions) error {
	min := int32(opts.Min)
	max := int32(opts.Max)

	metrics := make([]autoscalingv2.MetricSpec, 0, len(opts.Triggers))
	for _, t := range opts.Triggers {
		if t.Type != structs.TriggerTypeCPU && t.Type != structs.TriggerTypeMemory {
			continue
		}
		threshold := int32(t.Threshold)
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceName(t.Type),
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &threshold,
				},
			},
		})
	}

	scaleTargetRef := autoscalingv2.CrossVersionObjectReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       service,
	}

	existing, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil && !kerr.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return errors.WithStack(err)
	}
	if err != nil {
		// No existing HPA — create fresh.
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{
				Name:      service,
				Namespace: ns,
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: scaleTargetRef,
				MinReplicas:    &min,
				MaxReplicas:    max,
				Metrics:        metrics,
			},
		}
		_, createErr := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Create(context.TODO(), hpa, am.CreateOptions{})
		return errors.WithStack(createErr)
	}

	// In-place update: mutate the existing object so Spec.Behavior,
	// labels, annotations, and OwnerReferences are preserved across
	// the Update call.
	existing.Spec.ScaleTargetRef = scaleTargetRef
	existing.Spec.MinReplicas = &min
	existing.Spec.MaxReplicas = max
	existing.Spec.Metrics = metrics
	_, err = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), existing, am.UpdateOptions{})
	return errors.WithStack(err)
}

// applyTriggersKEDA creates or in-place patches a KEDA ScaledObject for
// the named service. CPU and memory triggers use KEDA's built-in
// resource types; GPU utilization and queue-depth triggers use the
// Prometheus trigger template mirroring manifest.ServiceAutoscale.BuildTriggers
// (pkg/manifest/service.go:428) so manifest-driven and Console-driven
// triggers materialize identically. Idempotent: if an SO already exists,
// it is updated in place.
func (p *Provider) applyTriggersKEDA(app, ns, service string, opts structs.ServiceTriggersOptions) error {
	promURL := os.Getenv("PROMETHEUS_URL")

	triggers := []interface{}{}
	for _, t := range opts.Triggers {
		switch t.Type {
		case structs.TriggerTypeCPU:
			triggers = append(triggers, map[string]interface{}{
				"type":       "cpu",
				"name":       "convox-cpu",
				"metricType": "Utilization",
				"metadata":   map[string]interface{}{"value": fmt.Sprintf("%g", t.Threshold)},
			})
		case structs.TriggerTypeMemory:
			triggers = append(triggers, map[string]interface{}{
				"type":       "memory",
				"name":       "convox-memory",
				"metricType": "Utilization",
				"metadata":   map[string]interface{}{"value": fmt.Sprintf("%g", t.Threshold)},
			})
		case structs.TriggerTypeGPUUtilization:
			triggers = append(triggers, map[string]interface{}{
				"type": "prometheus",
				"name": "convox-gpu-utilization",
				"metadata": map[string]interface{}{
					"serverAddress":       promURL,
					"metricName":          "DCGM_FI_DEV_GPU_UTIL",
					"threshold":           fmt.Sprintf("%g", t.Threshold),
					"activationThreshold": fmt.Sprintf("%g", prometheusActivationThreshold(t.Threshold)),
					"query":               fmt.Sprintf("max(DCGM_FI_DEV_GPU_UTIL{app=%q,service=%q})", app, service),
				},
			})
		case structs.TriggerTypeQueueDepth:
			triggers = append(triggers, map[string]interface{}{
				"type": "prometheus",
				"name": "convox-queue-depth",
				"metadata": map[string]interface{}{
					"serverAddress":       promURL,
					"metricName":          "vllm:num_requests_waiting",
					"threshold":           fmt.Sprintf("%g", t.Threshold),
					"activationThreshold": fmt.Sprintf("%g", prometheusActivationThreshold(t.Threshold)),
					"query":               fmt.Sprintf("max(vllm:num_requests_waiting{app=%q,service=%q})", app, service),
				},
			})
		}
	}

	existing, getErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
	if getErr != nil && !kerr.IsNotFound(getErr) && !meta.IsNoMatchError(getErr) {
		return errors.WithStack(getErr)
	}
	if getErr != nil {
		so := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "keda.sh/v1alpha1",
				"kind":       "ScaledObject",
				"metadata": map[string]interface{}{
					"name":      service,
					"namespace": ns,
				},
				"spec": map[string]interface{}{
					"scaleTargetRef":  map[string]interface{}{"name": service},
					"minReplicaCount": int64(opts.Min),
					"maxReplicaCount": int64(opts.Max),
					"triggers":        triggers,
				},
			},
		}
		_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Create(context.TODO(), so, am.CreateOptions{})
		return errors.WithStack(err)
	}

	// In-place update: mutate existing.Object so non-override fields
	// (cooldownPeriod, pollingInterval, advanced behaviors, labels,
	// annotations) survive the Update call. Only the override-owned
	// fields (scaleTargetRef, min/max, triggers) are replaced.
	_ = unstructured.SetNestedMap(existing.Object, map[string]interface{}{"name": service}, "spec", "scaleTargetRef")
	_ = unstructured.SetNestedField(existing.Object, int64(opts.Min), "spec", "minReplicaCount")
	_ = unstructured.SetNestedField(existing.Object, int64(opts.Max), "spec", "maxReplicaCount")
	_ = unstructured.SetNestedSlice(existing.Object, triggers, "spec", "triggers")
	_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), existing, am.UpdateOptions{})
	return errors.WithStack(err)
}

// populateLiveCRDThresholds reads the active autoscaler CRD (HPA and/or
// KEDA ScaledObject) and overlays its threshold values onto the supplied
// ServiceAutoscaleState. Source of truth for threshold fields surfaces
// the live CRD when one exists; the manifest values built by
// buildServiceAutoscaleState remain the fallback when no CRD is present.
//
// When both an HPA and a ScaledObject exist for the same service
// (operator-introduced corruption — manifest emits exactly one), the SO
// reads win because it carries the larger configuration surface; a
// structured log line surfaces the corruption to rack-side telemetry.
//
// state may be nil — the function returns without effect, so callers do
// not need to gate.
func (p *Provider) populateLiveCRDThresholds(app, service string, state *structs.ServiceAutoscaleState) {
	if state == nil {
		return
	}
	ns := p.AppNamespace(app)

	hpa, hpaErr := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	hasHPA := hpaErr == nil
	if hasHPA && hpa != nil {
		for _, m := range hpa.Spec.Metrics {
			if m.Type != autoscalingv2.ResourceMetricSourceType || m.Resource == nil || m.Resource.Target.AverageUtilization == nil {
				continue
			}
			val := int(*m.Resource.Target.AverageUtilization)
			switch string(m.Resource.Name) {
			case "cpu":
				state.CpuThreshold = &val
			case "memory":
				state.MemThreshold = &val
			}
		}
	}

	// DynamicClient is nil on racks with KEDA never installed and on
	// test fixtures that don't seed it; the SO read is purely
	// additive (only contributes when an SO exists), so guard rather
	// than fail the projection.
	var (
		so    *unstructured.Unstructured
		soErr error
	)
	if p.DynamicClient != nil {
		so, soErr = p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
	}
	hasSO := soErr == nil && so != nil
	if hasSO && so != nil {
		triggers, _, _ := unstructured.NestedSlice(so.Object, "spec", "triggers")
		for _, tIface := range triggers {
			tr, ok := tIface.(map[string]interface{})
			if !ok {
				continue
			}
			md, _ := tr["metadata"].(map[string]interface{})
			if md == nil {
				continue
			}
			rawValue, _ := md["value"].(string)
			if rawValue == "" {
				rawValue, _ = md["threshold"].(string)
			}
			if rawValue == "" {
				continue
			}
			num, err := strconv.ParseFloat(rawValue, 64)
			if err != nil {
				continue
			}
			ival := int(num)
			switch tr["name"] {
			case "convox-cpu":
				state.CpuThreshold = &ival
			case "convox-memory":
				state.MemThreshold = &ival
			case "convox-gpu-utilization":
				state.GpuThreshold = &ival
			case "convox-queue-depth":
				state.QueueThreshold = &ival
			}
		}
	}

	if hasHPA && hasSO {
		fmt.Printf("ns=service at=dual-crd-detected app=%q service=%q\n", app, service)
	}
}

// patchHPABounds applies min/max from ServiceUpdateOptions to the
// service's HPA. Used by serviceUpdateScaledObject when the
// triggers-override annotation declares the HPA owns the autoscaler;
// the Range Apply path on the Console reaches here.
func (p *Provider) patchHPABounds(ns, service string, opts structs.ServiceUpdateOptions) error {
	hpa, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) || meta.IsNoMatchError(err) {
			return fmt.Errorf("no HPA found for service %s; override state is corrupted (try Disable and re-enable)", service)
		}
		return errors.WithStack(err)
	}
	if opts.Min != nil {
		v := int32(*opts.Min)
		hpa.Spec.MinReplicas = &v
	}
	if opts.Max != nil {
		hpa.Spec.MaxReplicas = int32(*opts.Max)
	}
	if _, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), hpa, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// prometheusActivationThreshold mirrors the manifest-driven derivation
// (pkg/manifest/service.go:470-472, 500-502): half the trigger
// threshold, with a floor of 1, so KEDA only activates when the
// observed metric is materially non-zero.
func prometheusActivationThreshold(threshold float64) float64 {
	activation := threshold / 2
	if activation < 1 {
		activation = 1
	}
	return activation
}

// serviceGPUCount returns the manifest-declared scale.gpu.count for the
// named service from the rack's current release. Used by the GPU
// preflight in ServiceTriggersEnable to reject gpuUtilization triggers
// on services that don't have a GPU reservation (the Prometheus query
// behind the KEDA trigger would return nothing forever and autoscale
// would silently no-op).
//
// Tests may set Provider.TriggersOverrideManifestServiceHook to bypass
// the AppGet → ReleaseGet → manifest.Load chain and inject a manifest
// service deterministically.
func (p *Provider) serviceGPUCount(app, service string) (int, error) {
	s, err := p.serviceManifestService(app, service)
	if err != nil {
		return 0, err
	}
	return s.Scale.Gpu.Count, nil
}

func (p *Provider) serviceManifestService(app, service string) (*manifest.Service, error) {
	if p.TriggersOverrideManifestServiceHook != nil {
		return p.TriggersOverrideManifestServiceHook(app, service)
	}
	a, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	r, err := p.ReleaseGet(app, a.Release)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	m, err := manifest.Load([]byte(r.Manifest), map[string]string{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s, err := m.Service(service)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s, nil
}

// ServiceTriggersDisable removes the Console-driven autoscale configuration
// from a service: deletes the CRD recorded by the triggers-override-crd
// annotation, then clears both override annotations on the Deployment.
// Idempotent on a service that was never overridden; tolerant of
// already-deleted CRDs (kerr.IsNotFound) and of KEDA-not-installed racks
// (meta.IsNoMatchError on the dynamic-client lookup).
func (p *Provider) ServiceTriggersDisable(app, service, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)
	ns := p.AppNamespace(app)

	d, err := p.Cluster.AppsV1().Deployments(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return errors.WithStack(fmt.Errorf("service not found: %s/%s", app, service))
		}
		return errors.WithStack(err)
	}

	if d.Annotations == nil || d.Annotations[ServiceTriggersOverrideAnnotation] != ServiceTriggersOverrideValueOn {
		return nil
	}

	crd := d.Annotations[ServiceTriggersOverrideCRDAnnotation]

	switch crd {
	case TriggersCRDHPA:
		err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		if err != nil && !kerr.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return errors.WithStack(err)
		}
	case TriggersCRDKeda:
		err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		if err != nil && !kerr.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return errors.WithStack(err)
		}
	default:
		// Corruption recovery: active annotation present but the CRD
		// type was lost (e.g. partial write, hand-edited Deployment).
		// Try both deletes; whichever resource exists is removed.
		_ = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		_ = p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:null,%q:null}}}`,
		ServiceTriggersOverrideAnnotation, ServiceTriggersOverrideCRDAnnotation)
	if _, err := p.Cluster.AppsV1().Deployments(ns).Patch(
		context.TODO(), service, types.StrategicMergePatchType,
		[]byte(patch), am.PatchOptions{}); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("ns=service at=triggers-override-disable app=%s service=%s crd=%s ack_by=%q\n",
		app, service, crd, ackBy)

	_ = p.EventSend("app:triggers-override:toggled", structs.EventSendOptions{
		Data: map[string]string{
			"app":     app,
			"service": service,
			"state":   "off",
			"crd":     crd,
			"actor":   ackBy,
			"ack_by":  ackBy,
		},
	})

	return nil
}

// ServiceTriggersThresholdSet updates a single trigger's threshold value
// on the CRD owned by an active Console-driven override. Reads the
// triggers-override-crd annotation to dispatch to the HPA or KEDA branch;
// rejects when no override is active or when the requested trigger is
// not present on the active CRD.
func (p *Provider) ServiceTriggersThresholdSet(app, service, triggerType string, threshold float64, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)

	// Reuse the canonical TriggerSpec.Validate so the same threshold
	// rules apply on both Enable (full opts) and ThresholdSet (single
	// trigger): positive value, <=100 for percent types, queueDepth
	// uncapped. Mirrors structs.TriggerSpec.Validate at pkg/structs/service.go.
	if err := (structs.TriggerSpec{Type: triggerType, Threshold: threshold}).Validate(); err != nil {
		return err
	}
	ns := p.AppNamespace(app)

	d, err := p.Cluster.AppsV1().Deployments(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return errors.WithStack(fmt.Errorf("service not found: %s/%s", app, service))
		}
		return errors.WithStack(err)
	}
	if d.Annotations[ServiceTriggersOverrideAnnotation] != ServiceTriggersOverrideValueOn {
		return fmt.Errorf("threshold edits require an active triggers override. Click Enable triggers or Override triggers first")
	}

	crd := d.Annotations[ServiceTriggersOverrideCRDAnnotation]
	switch crd {
	case TriggersCRDHPA:
		if err := p.patchHPAThreshold(ns, service, triggerType, threshold); err != nil {
			return err
		}
	case TriggersCRDKeda:
		if err := p.patchKEDAThreshold(app, ns, service, triggerType, threshold); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown override CRD %q", crd)
	}

	fmt.Printf("ns=service at=triggers-override-threshold-set app=%s service=%s type=%s threshold=%g ack_by=%q\n",
		app, service, triggerType, threshold, ackBy)

	_ = p.EventSend("app:triggers-override:threshold-set", structs.EventSendOptions{
		Data: map[string]string{
			"app":       app,
			"service":   service,
			"type":      triggerType,
			"threshold": fmt.Sprintf("%g", threshold),
			"actor":     ackBy,
			"ack_by":    ackBy,
		},
	})

	return nil
}

func (p *Provider) patchHPAThreshold(ns, service, triggerType string, threshold float64) error {
	if triggerType != structs.TriggerTypeCPU && triggerType != structs.TriggerTypeMemory {
		return fmt.Errorf("HPA-backed override supports only cpu and memory triggers")
	}
	hpa, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) || meta.IsNoMatchError(err) {
			return fmt.Errorf("no HPA found for service %s; override state is corrupted (try Disable and re-enable)", service)
		}
		return errors.WithStack(err)
	}
	updated := false
	for i, m := range hpa.Spec.Metrics {
		if m.Type != autoscalingv2.ResourceMetricSourceType || m.Resource == nil {
			continue
		}
		if string(m.Resource.Name) == triggerType {
			v := int32(threshold)
			hpa.Spec.Metrics[i].Resource.Target.AverageUtilization = &v
			updated = true
		}
	}
	if !updated {
		return fmt.Errorf("trigger %q not present on this HPA; enable it first via service triggers enable", triggerType)
	}
	if _, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), hpa, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (p *Provider) patchKEDAThreshold(app, ns, service, triggerType string, threshold float64) error {
	targetName := map[string]string{
		structs.TriggerTypeCPU:            "convox-cpu",
		structs.TriggerTypeMemory:         "convox-memory",
		structs.TriggerTypeGPUUtilization: "convox-gpu-utilization",
		structs.TriggerTypeQueueDepth:     "convox-queue-depth",
	}[triggerType]
	if targetName == "" {
		return fmt.Errorf("unknown trigger type %q", triggerType)
	}

	obj, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) || meta.IsNoMatchError(err) {
			return fmt.Errorf("no ScaledObject found for service %s; override state is corrupted (try Disable and re-enable)", service)
		}
		return errors.WithStack(err)
	}
	triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers")
	updated := false
	for i, tIface := range triggers {
		tm, ok := tIface.(map[string]interface{})
		if !ok {
			continue
		}
		if tm["name"] != targetName {
			continue
		}
		md, _ := tm["metadata"].(map[string]interface{})
		if md == nil {
			md = map[string]interface{}{}
		}
		// CPU/memory built-in triggers use metadata.value; Prometheus
		// triggers store the actionable scaling threshold under
		// metadata.threshold + an activation gate at metadata.activationThreshold.
		if tm["type"] == "prometheus" {
			md["threshold"] = fmt.Sprintf("%g", threshold)
			md["activationThreshold"] = fmt.Sprintf("%g", prometheusActivationThreshold(threshold))
		} else {
			md["value"] = fmt.Sprintf("%g", threshold)
		}
		tm["metadata"] = md
		triggers[i] = tm
		updated = true
	}
	if !updated {
		return fmt.Errorf("trigger %q not present on this ScaledObject; enable it first via service triggers enable", triggerType)
	}
	if err := unstructured.SetNestedSlice(obj.Object, triggers, "spec", "triggers"); err != nil {
		return errors.WithStack(err)
	}
	if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), obj, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
