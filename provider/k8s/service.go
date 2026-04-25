package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
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
		if ms.Scale.Autoscale.IsEnabled() {
			s.Autoscale = buildServiceAutoscaleState(ms.Scale.Autoscale)
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

	if !hasAgent {
		return ss, nil
	}

	dss, err := p.Cluster.AppsV1().DaemonSets(p.AppNamespace(app)).List(context.TODO(), lopts)
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

		ss = append(ss, s)
	}

	return ss, nil
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
	d, err := p.GetDeploymentFromInformer(name, p.AppNamespace(app))
	if err != nil {
		return errors.WithStack(err)
	}

	if opts.Min != nil || opts.Max != nil {
		if err := p.serviceUpdateScaledObject(app, name, opts); err != nil {
			return err
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

	// When only --count was supplied AND the ScaledObject claimed ownership,
	// skip the Deployment Update. The informer-cached d.Spec.Replicas is
	// likely stale, and writing it back would briefly fight KEDA's reconciler
	// before KEDA converges on the patched minReplicaCount/maxReplicaCount.
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

func (p *Provider) serviceUpdateScaledObject(app, name string, opts structs.ServiceUpdateOptions) error {
	ns := p.AppNamespace(app)

	_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), name, am.GetOptions{})
	hasScaledObject := err == nil
	if err != nil && !kerr.IsNotFound(err) {
		return errors.WithStack(err)
	}

	if opts.Min != nil && *opts.Min == 0 {
		if err := p.ensureWakeMechanism(app, name, hasScaledObject); err != nil {
			return err
		}
	}

	if !hasScaledObject {
		return nil
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
		return errors.WithStack(err)
	}

	if _, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Patch(
		context.TODO(), name, ktypes.MergePatchType, patchBytes, am.PatchOptions{},
	); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// serviceUpdateCount patches the ScaledObject CRD when one owns the deployment;
// returns handled=true in that case so the caller knows to skip the subsequent
// deployment.Spec.Replicas write (which would race with KEDA's reconciler).
// Returns handled=false when no ScaledObject exists, letting the caller fall
// back to the normal Deployment patch path.
func (p *Provider) serviceUpdateCount(app, name string, count int) (handled bool, err error) {
	ns := p.AppNamespace(app)

	_, getErr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(context.TODO(), name, am.GetOptions{})
	if kerr.IsNotFound(getErr) {
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
