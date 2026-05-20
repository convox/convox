package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	resource "k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
)

var scaledObjectGVR = schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"}

func (p *Provider) ServiceHost(app string, s manifest.Service) string {
	if s.Internal {
		return fmt.Sprintf("%s.%s.%s.local", s.Name, app, p.Name)
	} else if s.InternalRouter {
		if p.ContextTID() != "" {
			return fmt.Sprintf("%s.%s.%s.%s", s.Name, app, p.ContextTID(), p.DomainInternal)
		}
		return fmt.Sprintf("%s.%s.%s", s.Name, app, p.DomainInternal)
	} else {
		if p.ContextTID() != "" {
			return fmt.Sprintf("%s.%s.%s.%s", s.Name, app, p.ContextTID(), p.Domain)
		}
		return fmt.Sprintf("%s.%s.%s", s.Name, app, p.Domain)
	}
}

func (p *Provider) ServiceList(app string) (structs.Services, error) {
	lopts := am.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s,type=service", app),
	}

	a, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if a.Release == "" {
		return structs.Services{}, nil
	}

	m, _, err := common.ReleaseManifest(p, app, a.Release)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ss := structs.Services{}

	ds, err := p.ListDeploymentsFromInformer(p.AppNamespace(app), lopts.LabelSelector)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, d := range ds.Items {
		c, err := primaryContainer(d.Spec.Template.Spec.Containers, app)
		if err != nil {
			return nil, err
		}

		ms, err := m.Service(d.ObjectMeta.Name)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		s := structs.Service{
			Count:  int(common.DefaultInt32(d.Spec.Replicas, 0)),
			Domain: p.ServiceHost(app, *ms),
			Name:   d.ObjectMeta.Name,
			Ports:  serviceContainerPorts(*c, ms.Internal),
		}

		if v := c.Resources.Requests.Cpu(); v != nil {
			s.Cpu = int(v.MilliValue())
		}

		if v := c.Resources.Requests.Memory(); v != nil {
			s.Memory = int(v.Value() / (1024 * 1024)) // Mi
		}

		for key, vendor := range gpuKeyToVendor {
			if q, ok := c.Resources.Requests[v1.ResourceName(key)]; ok {
				s.Gpu = int(q.Value())
				s.GpuVendor = vendor
				break
			}
		}

		min := ms.Scale.Count.Min
		max := ms.Scale.Count.Max
		s.Min = &min
		s.Max = &max
		if min == 0 && s.Count == 0 {
			cold := true
			s.ColdStart = &cold
		}

		triggersOverride := d.Annotations != nil && d.Annotations[ServiceTriggersOverrideAnnotation] == ServiceTriggersOverrideValueOn
		classicHPA := ms.Scale.Count.Max > ms.Scale.Count.Min &&
			!ms.Scale.Autoscale.IsEnabled() &&
			!ms.Scale.IsKedaEnabled()

		if ms.Scale.Autoscale.IsEnabled() {
			s.Autoscale = buildServiceAutoscaleState(ms.Scale.Autoscale)
		} else if classicHPA || triggersOverride {
			// Surface classic HPA / triggers-override as autoscale-enabled for Console.
			s.Autoscale = &structs.ServiceAutoscaleState{Enabled: true}
			if ms.Scale.Targets.Cpu > 0 {
				cpu := ms.Scale.Targets.Cpu
				s.Autoscale.CpuThreshold = &cpu
			}
			if ms.Scale.Targets.Memory > 0 {
				mem := ms.Scale.Targets.Memory
				s.Autoscale.MemThreshold = &mem
			}
		}

		// Overlay live HPA/SO bounds so users see what they actually set.
		p.overlayLiveCRDBounds(app, d.ObjectMeta.Name, &s)

		p.populateLiveCRDThresholds(app, d.ObjectMeta.Name, s.Autoscale)

		// 3.24.6+: always set pointer; nil = pre-3.24.6 rack (Console compat).
		if d.Annotations != nil && d.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn {
			s.ScaleOverrideActive = options.Bool(true)
		} else {
			s.ScaleOverrideActive = options.Bool(false)
		}

		if triggersOverride {
			s.TriggersOverrideActive = options.Bool(true)
		} else {
			s.TriggersOverrideActive = options.Bool(false)
		}

		ss = append(ss, s)
	}

	hasAgent := false
	for _, s := range m.Services {
		if s.Agent.Enabled {
			hasAgent = true
			break
		}
	}

	if hasAgent {
		ss, err = p.serviceListAppendDaemonsets(app, ss, &lopts, m)
		if err != nil {
			return nil, err
		}
	}

	p.enrichGpuTelemetry(context.Background(), app, ss)

	return ss, nil
}

func (p *Provider) serviceListAppendDaemonsets(app string, ss structs.Services, lopts *am.ListOptions, m *manifest.Manifest) (structs.Services, error) {
	dss, err := p.Cluster.AppsV1().DaemonSets(p.AppNamespace(app)).List(context.TODO(), *lopts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, d := range dss.Items {
		c, err := primaryContainer(d.Spec.Template.Spec.Containers, app)
		if err != nil {
			return nil, err
		}

		ms, err := m.Service(d.ObjectMeta.Name)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		s := structs.Service{
			Count:  int(d.Status.NumberReady),
			Domain: p.ServiceHost(app, *ms),
			Name:   d.ObjectMeta.Name,
			Ports:  serviceContainerPorts(*c, ms.Internal),
		}

		if v := c.Resources.Requests.Cpu(); v != nil {
			s.Cpu = int(v.MilliValue())
		}

		if v := c.Resources.Requests.Memory(); v != nil {
			s.Memory = int(v.Value() / (1024 * 1024)) // Mi
		}

		for key, vendor := range gpuKeyToVendor {
			if q, ok := c.Resources.Requests[v1.ResourceName(key)]; ok {
				s.Gpu = int(q.Value())
				s.GpuVendor = vendor
				break
			}
		}

		min := ms.Scale.Count.Min
		max := ms.Scale.Count.Max
		s.Min = &min
		s.Max = &max

		if d.Annotations != nil && d.Annotations[ServiceScaleOverrideAnnotation] == ServiceScaleOverrideValueOn {
			s.ScaleOverrideActive = options.Bool(true)
		} else {
			s.ScaleOverrideActive = options.Bool(false)
		}

		s.TriggersOverrideActive = options.Bool(false)

		s.Agent = true

		ss = append(ss, s)
	}

	return ss, nil
}

// enrichGpuTelemetry averages per-pod GPU metrics by service label onto ServiceList entries.
func (p *Provider) enrichGpuTelemetry(ctx context.Context, app string, ss structs.Services) {
	if p.PromClient == nil {
		return
	}

	gpuServices := []string{}
	for i := range ss {
		if ss[i].Gpu > 0 {
			gpuServices = append(gpuServices, ss[i].Name)
		}
	}
	if len(gpuServices) == 0 {
		return
	}

	gpuByPod, err := p.PromClient.QueryGPUMetrics(ctx, app, gpuServices)
	if err != nil {
		_ = p.logger.Errorf("failed to fetch gpu metrics: %s", err)
		return
	}

	// Per-metric independent counters so missing samples don't skew other metrics.
	type accum struct {
		util, memUsed, memTotal                float64
		utilCount, memUsedCount, memTotalCount int
		tensorActive, smActive, dramActive     float64
		tensorCount, smCount, dramCount        int
		fp16, fp32, powerW                     float64
		fp16Count, fp32Count, powerWCount      int
	}
	byService := map[string]*accum{}
	for _, gm := range gpuByPod {
		if gm.Service == "" {
			continue // pod was scraped but has no `service` label — skip
		}
		a := byService[gm.Service]
		if a == nil {
			a = &accum{}
			byService[gm.Service] = a
		}
		if gm.Util != nil {
			a.util += *gm.Util
			a.utilCount++
		}
		if gm.MemUsed != nil {
			a.memUsed += float64(*gm.MemUsed)
			a.memUsedCount++
		}
		if gm.MemTotal != nil {
			a.memTotal += float64(*gm.MemTotal)
			a.memTotalCount++
		}
		if gm.TensorActive != nil {
			a.tensorActive += *gm.TensorActive
			a.tensorCount++
		}
		if gm.SmActive != nil {
			a.smActive += *gm.SmActive
			a.smCount++
		}
		if gm.DramActive != nil {
			a.dramActive += *gm.DramActive
			a.dramCount++
		}
		if gm.Fp16Active != nil {
			a.fp16 += *gm.Fp16Active
			a.fp16Count++
		}
		if gm.Fp32Active != nil {
			a.fp32 += *gm.Fp32Active
			a.fp32Count++
		}
		if gm.PowerW != nil {
			a.powerW += *gm.PowerW
			a.powerWCount++
		}
	}
	for i := range ss {
		if ss[i].Gpu == 0 {
			continue
		}
		a, has := byService[ss[i].Name]
		if !has {
			continue
		}
		if a.utilCount > 0 {
			v := a.util / float64(a.utilCount)
			ss[i].GpuUtil = &v
		}
		if a.memUsedCount > 0 {
			v := int64(a.memUsed / float64(a.memUsedCount))
			ss[i].GpuMemUsed = &v
		}
		if a.memTotalCount > 0 {
			v := int64(a.memTotal / float64(a.memTotalCount))
			ss[i].GpuMemTotal = &v
		}
		if a.tensorCount > 0 {
			v := a.tensorActive / float64(a.tensorCount)
			ss[i].GpuTensorActive = &v
		}
		if a.smCount > 0 {
			v := a.smActive / float64(a.smCount)
			ss[i].GpuSmActive = &v
		}
		if a.dramCount > 0 {
			v := a.dramActive / float64(a.dramCount)
			ss[i].GpuDramActive = &v
		}
		if a.fp16Count > 0 {
			v := a.fp16 / float64(a.fp16Count)
			ss[i].GpuFp16Active = &v
		}
		if a.fp32Count > 0 {
			v := a.fp32 / float64(a.fp32Count)
			ss[i].GpuFp32Active = &v
		}
		if a.powerWCount > 0 {
			v := a.powerW / float64(a.powerWCount)
			ss[i].GpuPowerW = &v
		}
	}
}

// ServiceMetrics returns time-range GPU metrics for a single service (V3: GPU only).
func (p *Provider) ServiceMetrics(app, service string, opts structs.MetricsOptions) (structs.Metrics, error) {
	if p.PromClient == nil {
		return structs.Metrics{}, nil
	}

	ctx := p.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	byMetric, err := p.PromClient.QueryGPURange(ctx, app, []string{service}, opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	names := GpuRangeWireNames()
	metrics := make(structs.Metrics, 0, len(names))
	for _, wire := range names {
		byService := byMetric[wire]
		values := byService[service]
		if values == nil {
			values = structs.MetricValues{}
		}
		metrics = append(metrics, structs.Metric{
			Name:   wire,
			Values: structs.MetricValues(values),
		})
	}
	return metrics, nil
}

func (p *Provider) ServiceRestart(app, name string) error {
	m, _, err := common.AppManifest(p, app)
	if err != nil {
		return errors.WithStack(err)
	}

	s, err := m.Service(name)
	if err != nil {
		return errors.WithStack(err)
	}

	if s.Agent.Enabled {
		return p.serviceRestartDaemonset(app, name)
	}

	return p.serviceRestartDeployment(app, name)
}

func (p *Provider) serviceRestartDaemonset(app, name string) error {
	ds := p.Cluster.AppsV1().DaemonSets(p.AppNamespace(app))

	s, err := ds.Get(context.TODO(), name, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	if s.Spec.Template.Annotations == nil {
		s.Spec.Template.Annotations = map[string]string{}
	}

	s.Spec.Template.Annotations["convox.com/restart"] = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	if _, err := ds.Update(context.TODO(), s, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) serviceRestartDeployment(app, name string) error {
	ds := p.Cluster.AppsV1().Deployments(p.AppNamespace(app))

	s, err := ds.Get(context.TODO(), name, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	if s.Spec.Template.Annotations == nil {
		s.Spec.Template.Annotations = map[string]string{}
	}

	s.Spec.Template.Annotations["convox.com/restart"] = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	if _, err := ds.Update(context.TODO(), s, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) ServiceUpdate(app, name string, opts structs.ServiceUpdateOptions) error {
	if err := p.budgetCircuitBreakerTripped(app); err != nil {
		return errors.WithStack(err)
	}

	d, err := p.GetDeploymentFromInformer(name, p.AppNamespace(app))
	if err != nil {
		return errors.WithStack(err)
	}

	if opts.Min != nil || opts.Max != nil {
		handled, err := p.serviceUpdateScaledObject(app, name, opts)
		if err != nil {
			return err
		}
		if !handled {
			if opts.Max != nil {
				if !p.IsKedaEnabled {
					return fmt.Errorf("range scaling (min/max) requires KEDA on this rack; run `convox rack params set keda_enable=true` and re-deploy, click Enable triggers in the Console to configure one through the UI (CPU/Memory work without KEDA), or use --count for a fixed replica count")
				}
				return fmt.Errorf("range scaling (min/max) requires an autoscale block in convox.yml; set scale.autoscale and re-deploy, click Enable triggers in the Console to configure one through the UI, or use --count for a fixed replica count")
			}
			// --min-only: patch replicas directly (backward compat with `convox scale --min N`).
			if opts.Min != nil {
				c := int32(*opts.Min) //nolint:gosec // replica counts are user-validated and bounded
				d.Spec.Replicas = &c
			}
		}
	}

	countHandledByScaledObject := false
	if opts.Count != nil {
		countHandled, err := p.serviceUpdateCount(app, name, *opts.Count)
		if err != nil {
			return err
		}
		countHandledByScaledObject = countHandled
		if !countHandled {
			c := int32(*opts.Count) //nolint:gosec // replica counts are user-validated and bounded
			d.Spec.Replicas = &c
		}
	}

	if opts.Cpu != nil {
		cpuSize := resource.MustParse(fmt.Sprintf("%dm", *opts.Cpu))

		d.Spec.Template.Spec.Containers[0].Resources.
			Requests[v1.ResourceCPU] = cpuSize
	}

	if opts.Memory != nil {
		memorySize := resource.MustParse(fmt.Sprintf("%dMi", *opts.Memory))

		d.Spec.Template.Spec.Containers[0].Resources.
			Limits[v1.ResourceMemory] = memorySize

		d.Spec.Template.Spec.Containers[0].Resources.
			Requests[v1.ResourceMemory] = memorySize
	}

	if opts.Gpu != nil {
		vendor := ""
		if opts.GpuVendor != nil {
			vendor = *opts.GpuVendor
		}
		key := v1.ResourceName(gpuResourceKey(vendor))
		qty := resource.MustParse(fmt.Sprintf("%d", *opts.Gpu))

		if d.Spec.Template.Spec.Containers[0].Resources.Requests == nil {
			d.Spec.Template.Spec.Containers[0].Resources.Requests = v1.ResourceList{}
		}
		d.Spec.Template.Spec.Containers[0].Resources.Requests[key] = qty

		if d.Spec.Template.Spec.Containers[0].Resources.Limits == nil {
			d.Spec.Template.Spec.Containers[0].Resources.Limits = v1.ResourceList{}
		}
		d.Spec.Template.Spec.Containers[0].Resources.Limits[key] = qty

		if *opts.Gpu > 0 {
			hasGpuToleration := false
			for _, t := range d.Spec.Template.Spec.Tolerations {
				if t.Key == string(key) && t.Effect == v1.TaintEffectNoSchedule && t.Operator == v1.TolerationOpExists {
					hasGpuToleration = true
					break
				}
			}
			if !hasGpuToleration {
				d.Spec.Template.Spec.Tolerations = append(d.Spec.Template.Spec.Tolerations, v1.Toleration{
					Key:      string(key),
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoSchedule,
				})
			}
		}
	}

	// Skip Deployment write when KEDA owns replicas (avoids KEDA reconciler race).
	countOnly := opts.Count != nil && opts.Cpu == nil && opts.Memory == nil && opts.Gpu == nil && opts.Min == nil && opts.Max == nil
	if countOnly && countHandledByScaledObject {
		return nil
	}

	if _, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Update(context.TODO(), d, am.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) serviceDaemonset(app, name string) (*appsv1.DaemonSet, error) {
	ds := p.Cluster.AppsV1().DaemonSets(p.AppNamespace(app))
	return ds.Get(context.TODO(), name, am.GetOptions{})
}

func (p *Provider) serviceDeployment(app, name string) (*appsv1.Deployment, error) {
	ds := p.Cluster.AppsV1().Deployments(p.AppNamespace(app))
	return ds.Get(context.TODO(), name, am.GetOptions{})
}

func serviceContainerPorts(c v1.Container, internal bool) []structs.ServicePort {
	ps := []structs.ServicePort{}

	for _, cp := range c.Ports {
		if cp.Name == "main" && !internal {
			ps = append(ps, structs.ServicePort{Balancer: 443, Container: int(cp.ContainerPort)})
		} else {
			ps = append(ps, structs.ServicePort{Container: int(cp.ContainerPort)})
		}
	}

	return ps
}

func buildServiceAutoscaleState(a *manifest.ServiceAutoscale) *structs.ServiceAutoscaleState {
	if !a.IsEnabled() {
		return nil
	}
	st := &structs.ServiceAutoscaleState{Enabled: true}
	if a.Cpu != nil {
		v := int(a.Cpu.Threshold)
		st.CpuThreshold = &v
	}
	if a.Memory != nil {
		v := int(a.Memory.Threshold)
		st.MemThreshold = &v
	}
	if a.GpuUtilization != nil {
		v := int(a.GpuUtilization.Threshold)
		st.GpuThreshold = &v
		if a.GpuUtilization.MetricName != "" {
			st.MetricName = a.GpuUtilization.MetricName
		}
	}
	if a.QueueDepth != nil {
		v := int(a.QueueDepth.Threshold)
		st.QueueThreshold = &v
		if a.QueueDepth.MetricName != "" {
			st.MetricName = a.QueueDepth.MetricName
		}
	}
	st.CustomTriggers = len(a.Custom)
	return st
}

// serviceUpdateScaledObject patches the KEDA ScaledObject min/max; returns handled=true when SO exists.
func (p *Provider) serviceUpdateScaledObject(app, name string, opts structs.ServiceUpdateOptions) (handled bool, err error) {
	ns := p.AppNamespace(app)

	// Triggers-override HPA path takes priority; KEDA path falls through below.
	if d, derr := p.Cluster.AppsV1().Deployments(ns).Get(context.TODO(), name, am.GetOptions{}); derr == nil &&
		d.Annotations[ServiceTriggersOverrideAnnotation] == ServiceTriggersOverrideValueOn &&
		d.Annotations[ServiceTriggersOverrideCRDAnnotation] == TriggersCRDHPA {
		if err := p.patchHPABounds(ns, name, opts); err != nil {
			return false, err
		}
		return true, nil
	}

	_, getErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), name, am.GetOptions{})
	hasScaledObject := getErr == nil
	// NoMatchError = KEDA CRD absent; treat same as not-found.
	if getErr != nil && !kerr.IsNotFound(getErr) && !meta.IsNoMatchError(getErr) {
		return false, errors.WithStack(getErr)
	}

	if opts.Min != nil && *opts.Min == 0 {
		if err := p.ensureWakeMechanism(app, name, hasScaledObject); err != nil {
			return false, err
		}
	}

	if !hasScaledObject {
		return false, nil
	}

	spec := map[string]interface{}{}
	if opts.Min != nil {
		spec["minReplicaCount"] = *opts.Min
	}
	if opts.Max != nil {
		spec["maxReplicaCount"] = *opts.Max
	}
	patch := map[string]interface{}{"spec": spec}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Patch(
		context.TODO(), name, ktypes.MergePatchType, patchBytes, am.PatchOptions{},
	); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

// serviceUpdateCount patches the ScaledObject when SO owns replicas; returns handled=true.
func (p *Provider) serviceUpdateCount(app, name string, count int) (handled bool, err error) {
	ns := p.AppNamespace(app)

	_, getErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), name, am.GetOptions{})
	if kerr.IsNotFound(getErr) {
		return false, nil
	}
	// NoMatchError = KEDA CRD absent; treat same as not-found.
	if meta.IsNoMatchError(getErr) {
		return false, nil
	}
	if getErr != nil {
		return false, errors.WithStack(getErr)
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"minReplicaCount": count,
			"maxReplicaCount": count,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Patch(
		context.TODO(), name, ktypes.MergePatchType, patchBytes, am.PatchOptions{},
	); err != nil {
		return false, errors.WithStack(err)
	}

	_ = p.EventSend("release:imperative-patch-note", structs.EventSendOptions{
		Data: map[string]string{
			"actor":   "system",
			"app":     app,
			"service": name,
			"reason":  "KEDA ScaledObject owns replicas; patched scaledobject spec.minReplicaCount / spec.maxReplicaCount instead of deployment replicas",
		},
	})

	return true, nil
}

func (p *Provider) ensureWakeMechanism(app, name string, hasScaledObject bool) error {
	if hasScaledObject {
		return nil
	}

	a, err := p.AppGet(app)
	if err != nil {
		return errors.WithStack(err)
	}

	if a.Release != "" {
		m, _, err := common.ReleaseManifest(p, app, a.Release)
		if err != nil {
			return errors.WithStack(fmt.Errorf("could not verify wake mechanism for service %s: %w", name, err))
		}
		ms, mErr := m.Service(name)
		if mErr != nil {
			return structs.ErrBadRequest("service %s not found in current release manifest", name)
		}
		if ms.Scale.Autoscale.IsEnabled() || ms.Scale.IsKedaEnabled() {
			return nil
		}
	}

	return structs.ErrBadRequest(
		"cannot set --min 0 on service %s: no autoscale mechanism is configured to wake pods back up. Set scale.autoscale.* in convox.yml and promote a release first, or use --min 1 (or higher)",
		name,
	)
}
