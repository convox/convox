package k8s

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/pkg/errors"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ServiceTriggersOverrideAnnotation    = "convox.com/triggers-override-active"
	ServiceTriggersOverrideCRDAnnotation = "convox.com/triggers-override-crd"
	ServiceTriggersOverrideValueOn       = "true"

	TriggersCRDHPA  = "hpa"
	TriggersCRDKeda = "keda"
)

func triggersCRDChoice(triggers []structs.TriggerSpec) string {
	for _, t := range triggers {
		if t.Type == structs.TriggerTypeGPUUtilization || t.Type == structs.TriggerTypeQueueDepth {
			return TriggersCRDKeda
		}
	}
	return TriggersCRDHPA
}

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
		return fmt.Errorf("GPU and inference queue triggers require KEDA. Run: convox rack params set keda_enable=true")
	}

	if crdChoice == TriggersCRDKeda {
		promURL := strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
		if promURL == "" {
			for _, t := range opts.Triggers {
				if t.Type == structs.TriggerTypeGPUUtilization || t.Type == structs.TriggerTypeQueueDepth {
					return fmt.Errorf("GPU and inference queue triggers require GPU Telemetry. Enable it on the rack settings page or set prometheus_url directly: convox rack params set prometheus_url=<url>")
				}
			}
		}
	}

	if crdChoice == TriggersCRDHPA && opts.Min < 1 {
		return fmt.Errorf("CPU and memory triggers do not support min=0. To scale to zero, add a GPU utilization or inference queue depth trigger, or set min to 1 or higher")
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
		// Delete any manifest-materialized CRD of the opposite type.
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
		// Rollback: delete orphan CRD if annotation patch failed.
		switch crdChoice {
		case TriggersCRDHPA:
			_ = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		case TriggersCRDKeda:
			_ = p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Delete(context.TODO(), service, am.DeleteOptions{})
		}
		return errors.WithStack(err)
	}

	minPatch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, opts.Min)
	if _, err := p.Cluster.AppsV1().Deployments(ns).Patch(
		context.TODO(), service, types.StrategicMergePatchType,
		[]byte(minPatch), am.PatchOptions{}); err != nil {
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

func (p *Provider) applyTriggersHPA(ns, service string, opts structs.ServiceTriggersOptions) error {
	min := int32(opts.Min) //nolint:gosec
	max := int32(opts.Max) //nolint:gosec

	metrics := make([]autoscalingv2.MetricSpec, 0, len(opts.Triggers))
	for _, t := range opts.Triggers {
		if t.Type != structs.TriggerTypeCPU && t.Type != structs.TriggerTypeMemory {
			continue
		}
		threshold := int32(t.Threshold) //nolint:gosec
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

	// Strip atom label so deploy-time prune won't delete Console-owned HPA.
	existing.Spec.ScaleTargetRef = scaleTargetRef
	existing.Spec.MinReplicas = &min
	existing.Spec.MaxReplicas = max
	existing.Spec.Metrics = metrics
	delete(existing.Labels, "atom")
	_, err = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), existing, am.UpdateOptions{})
	return errors.WithStack(err)
}

func (p *Provider) applyTriggersKEDA(app, ns, service string, opts structs.ServiceTriggersOptions) error {
	promURL := strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))

	if promURL == "" {
		for _, t := range opts.Triggers {
			if t.Type == structs.TriggerTypeGPUUtilization || t.Type == structs.TriggerTypeQueueDepth {
				return fmt.Errorf("GPU and inference queue triggers require GPU Telemetry; enable it on the rack settings page or set prometheus_url directly")
			}
		}
	}

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

	// Strip atom label so deploy-time prune won't delete Console-owned SO.
	_ = unstructured.SetNestedMap(existing.Object, map[string]interface{}{"name": service}, "spec", "scaleTargetRef")
	_ = unstructured.SetNestedField(existing.Object, int64(opts.Min), "spec", "minReplicaCount")
	_ = unstructured.SetNestedField(existing.Object, int64(opts.Max), "spec", "maxReplicaCount")
	_ = unstructured.SetNestedSlice(existing.Object, triggers, "spec", "triggers")
	labels, _, _ := unstructured.NestedStringMap(existing.Object, "metadata", "labels")
	if _, ok := labels["atom"]; ok {
		delete(labels, "atom")
		_ = unstructured.SetNestedStringMap(existing.Object, labels, "metadata", "labels")
	}
	_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), existing, am.UpdateOptions{})
	return errors.WithStack(err)
}

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

func (p *Provider) patchHPABounds(ns, service string, opts structs.ServiceUpdateOptions) error {
	hpa, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) || meta.IsNoMatchError(err) {
			return fmt.Errorf("no HPA found for service %s; override state is corrupted (try Disable and re-enable)", service)
		}
		return errors.WithStack(err)
	}
	if opts.Min != nil {
		v := int32(*opts.Min) //nolint:gosec
		hpa.Spec.MinReplicas = &v
	}
	if opts.Max != nil {
		hpa.Spec.MaxReplicas = int32(*opts.Max) //nolint:gosec
	}
	delete(hpa.Labels, "atom")
	if _, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), hpa, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (p *Provider) overlayLiveCRDBounds(app, service string, s *structs.Service) {
	if s == nil {
		return
	}
	ns := p.AppNamespace(app)

	hpa, hpaErr := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
	if hpaErr == nil && hpa != nil {
		if hpa.Spec.MinReplicas != nil {
			v := int(*hpa.Spec.MinReplicas)
			s.Min = &v
		}
		v := int(hpa.Spec.MaxReplicas)
		s.Max = &v
	}

	if p.DynamicClient == nil {
		return
	}
	so, soErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
	if soErr != nil || so == nil {
		return
	}
	if minVal, found, _ := unstructured.NestedInt64(so.Object, "spec", "minReplicaCount"); found {
		v := int(minVal)
		s.Min = &v
	}
	if maxVal, found, _ := unstructured.NestedInt64(so.Object, "spec", "maxReplicaCount"); found {
		v := int(maxVal)
		s.Max = &v
	}
}

// Half the threshold, floor of 1 — mirrors manifest-driven derivation.
func prometheusActivationThreshold(threshold float64) float64 {
	activation := threshold / 2
	if activation < 1 {
		activation = 1
	}
	return activation
}

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

// Best-effort: next deploy creates the authoritative autoscaler regardless.
func (p *Provider) reinstateManifestAutoscaler(ns, app, service string, ms *manifest.Service) {
	if ms.Scale.IsKedaEnabled() {
		p.reinstateKedaScaledObject(ns, app, service, ms)
		return
	}

	if ms.Scale.Autoscale != nil && len(ms.Scale.Autoscale.Custom) > 0 {
		p.reinstateKedaScaledObject(ns, app, service, ms)
		return
	}

	opts, ok := manifestToTriggersOptions(ms)
	if !ok {
		return
	}
	crd := triggersCRDChoice(opts.Triggers)
	var err error
	switch crd {
	case TriggersCRDHPA:
		err = p.applyTriggersHPA(ns, service, opts)
	case TriggersCRDKeda:
		err = p.applyTriggersKEDA(app, ns, service, opts)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: could not reinstate manifest autoscaler for %s/%s: %v\n", app, service, err)
	}
}

func (p *Provider) reinstateKedaScaledObject(ns, app, service string, ms *manifest.Service) {
	if p.DynamicClient == nil {
		return
	}
	min := int32(ms.Scale.Count.Min) //nolint:gosec
	max := int32(ms.Scale.Count.Max) //nolint:gosec

	var triggers []kedav1alpha1.ScaleTriggers
	if ms.Scale.Autoscale != nil && ms.Scale.Autoscale.IsEnabled() {
		promURL := strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
		triggers = append(triggers, ms.Scale.Autoscale.BuildTriggers(app, service, promURL)...)
	}
	if ms.Scale.Keda != nil {
		triggers = append(triggers, ms.Scale.Keda.Triggers...)
	}

	so := ms.KedaScaledObject(manifest.KedaScaledObjectParameters{
		ServiceName: service,
		Namespace:   ns,
		MinCount:    min,
		MaxCount:    max,
		Triggers:    triggers,
	})
	if so == nil {
		return
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(so)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: could not convert ScaledObject for %s/%s: %v\n", app, service, err)
		return
	}
	u := &unstructured.Unstructured{Object: obj}

	existing, getErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
	if getErr != nil {
		_, createErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Create(context.TODO(), u, am.CreateOptions{})
		if createErr != nil && !meta.IsNoMatchError(createErr) {
			fmt.Fprintf(os.Stderr, "NOTICE: could not reinstate KEDA ScaledObject for %s/%s: %v\n", app, service, createErr)
		}
		return
	}
	u.SetResourceVersion(existing.GetResourceVersion())
	_, updateErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), u, am.UpdateOptions{})
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: could not reinstate KEDA ScaledObject for %s/%s: %v\n", app, service, updateErr)
	}
}

func manifestToTriggersOptions(ms *manifest.Service) (structs.ServiceTriggersOptions, bool) {
	min := ms.Scale.Count.Min
	max := ms.Scale.Count.Max
	if min == max {
		return structs.ServiceTriggersOptions{}, false
	}

	var triggers []structs.TriggerSpec

	if ms.Scale.Autoscale != nil {
		a := ms.Scale.Autoscale
		if a.Cpu != nil && a.Cpu.Threshold > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeCPU, Threshold: a.Cpu.Threshold})
		}
		if a.Memory != nil && a.Memory.Threshold > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeMemory, Threshold: a.Memory.Threshold})
		}
		if a.GpuUtilization != nil && a.GpuUtilization.Threshold > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeGPUUtilization, Threshold: a.GpuUtilization.Threshold})
		}
		if a.QueueDepth != nil && a.QueueDepth.Threshold > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeQueueDepth, Threshold: a.QueueDepth.Threshold})
		}
	} else {
		if ms.Scale.Targets.Cpu > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeCPU, Threshold: float64(ms.Scale.Targets.Cpu)})
		}
		if ms.Scale.Targets.Memory > 0 {
			triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeMemory, Threshold: float64(ms.Scale.Targets.Memory)})
		}
	}

	if len(triggers) == 0 {
		// Range-only services default to CPU=80% HPA in deploy template.
		triggers = append(triggers, structs.TriggerSpec{Type: structs.TriggerTypeCPU, Threshold: 80})
	}

	opts := structs.ServiceTriggersOptions{
		Min:      min,
		Max:      max,
		Triggers: triggers,
	}

	// HPA rejects minReplicas=0; only KEDA supports scale-to-zero.
	if triggersCRDChoice(opts.Triggers) == TriggersCRDHPA && opts.Min < 1 {
		opts.Min = 1
	}

	return opts, true
}

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

	var replicaTarget int32 = 1
	if ms, msErr := p.serviceManifestService(app, service); msErr == nil && ms != nil {
		if ms.Scale.Count.Min > 0 {
			replicaTarget = int32(ms.Scale.Count.Min) //nolint:gosec
		}
		p.reinstateManifestAutoscaler(ns, app, service, ms)
	}
	scalePatch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicaTarget)
	_, _ = p.Cluster.AppsV1().Deployments(ns).Patch(
		context.TODO(), service, types.StrategicMergePatchType,
		[]byte(scalePatch), am.PatchOptions{})

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

func (p *Provider) ServiceTriggersThresholdSet(app, service, triggerType string, threshold float64, ackBy string) error {
	ackBy = sanitizeAckBy(ackBy)

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
			v := int32(threshold) //nolint:gosec
			hpa.Spec.Metrics[i].Resource.Target.AverageUtilization = &v
			updated = true
		}
	}
	if !updated {
		v := int32(threshold) //nolint:gosec
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceName(triggerType),
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &v,
				},
			},
		})
	}
	delete(hpa.Labels, "atom")
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
	soLabels, _, _ := unstructured.NestedStringMap(obj.Object, "metadata", "labels")
	if _, ok := soLabels["atom"]; ok {
		delete(soLabels, "atom")
		_ = unstructured.SetNestedStringMap(obj.Object, soLabels, "metadata", "labels")
	}
	if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), obj, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Strip atom label so deploy-time prune won't delete Console-owned CRD.
func (p *Provider) stripAtomLabelFromOverrideCRD(app, service, crd string) {
	ns := p.AppNamespace(app)
	switch crd {
	case TriggersCRDHPA:
		hpa, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(context.TODO(), service, am.GetOptions{})
		if err != nil {
			return
		}
		if _, ok := hpa.Labels["atom"]; !ok {
			return
		}
		delete(hpa.Labels, "atom")
		if _, err := p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(context.TODO(), hpa, am.UpdateOptions{}); err != nil {
			fmt.Printf("ns=service at=strip-atom-label-hpa app=%s service=%s err=%v\n", app, service, err)
		}
	case TriggersCRDKeda:
		if p.DynamicClient == nil {
			return
		}
		so, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), service, am.GetOptions{})
		if err != nil {
			return
		}
		labels, _, _ := unstructured.NestedStringMap(so.Object, "metadata", "labels")
		if _, ok := labels["atom"]; !ok {
			return
		}
		delete(labels, "atom")
		_ = unstructured.SetNestedStringMap(so.Object, labels, "metadata", "labels")
		if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Update(context.TODO(), so, am.UpdateOptions{}); err != nil {
			fmt.Printf("ns=service at=strip-atom-label-so app=%s service=%s err=%v\n", app, service, err)
		}
	}
}
