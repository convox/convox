package k8s

import (
	"fmt"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	v1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ServiceHost(app string, s manifest.Service) string {
	if s.Internal {
		return fmt.Sprintf("%s.%s.%s.local", s.Name, app, p.Name)
	} else {
		return fmt.Sprintf("%s.%s.%s", s.Name, app, p.Domain)
	}
}

func (p *Provider) ServiceList(app string) (structs.Services, error) {
	lopts := am.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s,type=service", app),
	}

	a, err := p.AppGet(app)
	if err != nil {
		return nil, err
	}

	if a.Release == "" {
		return structs.Services{}, nil
	}

	m, _, err := common.ReleaseManifest(p, app, a.Release)
	if err != nil {
		return nil, err
	}

	ss := structs.Services{}

	ds, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).List(lopts)
	if err != nil {
		return nil, err
	}

	for _, d := range ds.Items {
		cs := d.Spec.Template.Spec.Containers

		if len(cs) != 1 || cs[0].Name != "main" {
			return nil, fmt.Errorf("unexpected containers for service: %s", d.ObjectMeta.Name)
		}

		ms, err := m.Service(d.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}

		s := structs.Service{
			Count:  int(common.DefaultInt32(d.Spec.Replicas, 0)),
			Domain: p.Engine.ServiceHost(app, *ms),
			Name:   d.ObjectMeta.Name,
			Ports:  serviceContainerPorts(cs[0], ms.Internal),
		}

		ss = append(ss, s)
	}

	dss, err := p.Cluster.AppsV1().DaemonSets(p.AppNamespace(app)).List(lopts)
	if err != nil {
		return nil, err
	}

	for _, d := range dss.Items {
		cs := d.Spec.Template.Spec.Containers

		if len(cs) != 1 || cs[0].Name != "main" {
			return nil, fmt.Errorf("unexpected containers for service: %s", d.ObjectMeta.Name)
		}

		ms, err := m.Service(d.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}

		s := structs.Service{
			Count:  int(d.Status.NumberReady),
			Domain: p.Engine.ServiceHost(app, *ms),
			Name:   d.ObjectMeta.Name,
			Ports:  serviceContainerPorts(cs[0], ms.Internal),
		}

		ss = append(ss, s)
	}

	return ss, nil
}

func (p *Provider) ServiceRestart(app, name string) error {
	m, _, err := common.AppManifest(p, app)
	if err != nil {
		return err
	}

	s, err := m.Service(name)
	if err != nil {
		return err
	}

	if s.Agent.Enabled {
		return p.serviceRestartDaemonset(app, name)
	}

	return p.serviceRestartDeployment(app, name)
}

func (p *Provider) serviceRestartDaemonset(app, name string) error {
	ds := p.Cluster.ExtensionsV1beta1().DaemonSets(p.AppNamespace(app))

	s, err := ds.Get(name, am.GetOptions{})
	if err != nil {
		return err
	}

	if s.Spec.Template.Annotations == nil {
		s.Spec.Template.Annotations = map[string]string{}
	}

	s.Spec.Template.Annotations["convox.com/restart"] = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	if _, err := ds.Update(s); err != nil {
		return err
	}

	return nil
}

func (p *Provider) serviceRestartDeployment(app, name string) error {
	ds := p.Cluster.ExtensionsV1beta1().Deployments(p.AppNamespace(app))

	s, err := ds.Get(name, am.GetOptions{})
	if err != nil {
		return err
	}

	if s.Spec.Template.Annotations == nil {
		s.Spec.Template.Annotations = map[string]string{}
	}

	s.Spec.Template.Annotations["convox.com/restart"] = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	if _, err := ds.Update(s); err != nil {
		return err
	}

	return nil
}

func (p *Provider) ServiceUpdate(app, name string, opts structs.ServiceUpdateOptions) error {
	d, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Get(name, am.GetOptions{})
	if err != nil {
		return err
	}

	if opts.Count != nil {
		c := int32(*opts.Count)
		d.Spec.Replicas = &c
	}

	if _, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(app)).Update(d); err != nil {
		return err
	}

	return nil
}

func (p *Provider) serviceInstall(app, release, service string) error {
	a, err := p.AppGet(app)
	if err != nil {
		return err
	}

	m, r, err := common.ReleaseManifest(p, app, release)
	if err != nil {
		return err
	}

	s, err := m.Service(service)
	if err != nil {
		return err
	}

	if s.Port.Port == 0 {
		return nil
	}

	params := map[string]interface{}{
		"Namespace": p.AppNamespace(a.Name),
		"Release":   r,
		"Service":   s,
	}

	data, err := p.RenderTemplate("app/ports", params)
	if err != nil {
		return err
	}

	if err := p.Apply(p.AppNamespace(app), fmt.Sprintf("service.%s", service), r.Id, data, fmt.Sprintf("system=convox,provider=k8s,rack=%s,app=%s,release=%s", p.Name, app, r.Id), 30); err != nil {
		return err
	}

	return nil
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
