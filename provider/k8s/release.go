package k8s

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	v1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ReleaseCreate(app string, opts structs.ReleaseCreateOptions) (*structs.Release, error) {
	r, err := p.releaseFork(app)
	if err != nil {
		return nil, err
	}

	if opts.Build != nil {
		r.Build = *opts.Build
	}

	if opts.Env != nil {
		r.Env = *opts.Env
	}

	if r.Build != "" {
		b, err := p.BuildGet(app, r.Build)
		if err != nil {
			return nil, err
		}

		r.Manifest = b.Manifest
	}

	ro, err := p.releaseCreate(r)
	if err != nil {
		return nil, err
	}

	return ro, nil
}

func (p *Provider) ReleaseGet(app, id string) (*structs.Release, error) {
	r, err := p.releaseGet(app, id)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (p *Provider) ReleaseList(app string, opts structs.ReleaseListOptions) (structs.Releases, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, err
	}

	rs, err := p.releaseList(app)
	if err != nil {
		return nil, err
	}

	sort.Slice(rs, func(i, j int) bool { return rs[j].Created.Before(rs[i].Created) })

	if limit := common.DefaultInt(opts.Limit, 10); len(rs) > limit {
		rs = rs[0:limit]
	}

	return rs, nil
}

func (p *Provider) ReleasePromote(app, id string, opts structs.ReleasePromoteOptions) error {
	a, err := p.AppGet(app)
	if err != nil {
		return err
	}

	items := [][]byte{}

	// app
	data, err := p.releaseTemplateApp(a, opts)
	if err != nil {
		return err
	}

	items = append(items, data)

	// ca
	if ca, err := p.Cluster.CoreV1().Secrets("convox-system").Get("ca", am.GetOptions{}); err == nil {
		data, err := p.releaseTemplateCA(a, ca)
		if err != nil {
			return err
		}

		items = append(items, data)
	}

	if id != "" {
		m, r, err := common.ReleaseManifest(p, app, id)
		if err != nil {
			return err
		}

		e, err := structs.NewEnvironment([]byte(r.Env))
		if err != nil {
			return err
		}

		// balancers
		for _, b := range m.Balancers {
			data, err := p.releaseTemplateBalancer(a, r, b)
			if err != nil {
				return err
			}

			items = append(items, data)
		}

		// ingress
		if rss := m.Services.Routable().External(); len(rss) > 0 {
			data, err := p.releaseTemplateIngress(a, rss, opts)
			if err != nil {
				return err
			}

			items = append(items, data)
		}

		// resources
		for _, r := range m.Resources {
			data, err := p.releaseTemplateResource(a, r)
			if err != nil {
				return err
			}

			items = append(items, data)
		}

		// services
		data, err := p.releaseTemplateServices(a, e, r, m.Services, opts)
		if err != nil {
			return err
		}

		items = append(items, data)

		// timers
		for _, t := range m.Timers {
			s, err := m.Service(t.Service)
			if err != nil {
				return err
			}

			data, err := p.releaseTemplateTimer(a, r, s, t)
			if err != nil {
				return err
			}

			items = append(items, data)
		}

		// volumes
		data, err = p.releaseTemplateVolumes(a, m.Services)
		if err != nil {
			return err
		}

		items = append(items, data)
	}

	tdata := bytes.Join(items, []byte("---\n"))

	timeout := int32(common.DefaultInt(opts.Timeout, 1800))

	if err := p.Apply(p.AppNamespace(app), "app", id, tdata, fmt.Sprintf("system=convox,provider=k8s,rack=%s,app=%s,release=%s", p.Name, app, id), timeout); err != nil {
		return err
	}

	return nil
}

func (p *Provider) releaseCreate(r *structs.Release) (*structs.Release, error) {
	c, err := p.convoxClient()
	if err != nil {
		return nil, err
	}

	kr, err := c.ConvoxV1().Releases(p.AppNamespace(r.App)).Create(p.releaseMarshal(r))
	if err != nil {
		return nil, err
	}

	return p.releaseUnmarshal(kr)
}

func (p *Provider) releaseGet(app, id string) (*structs.Release, error) {
	c, err := p.convoxClient()
	if err != nil {
		return nil, err
	}

	kr, err := c.ConvoxV1().Releases(p.AppNamespace(app)).Get(strings.ToLower(id), am.GetOptions{})
	if err != nil {
		return nil, err
	}

	return p.releaseUnmarshal(kr)
}

func (p *Provider) releaseFork(app string) (*structs.Release, error) {
	r := &structs.Release{
		Id:      common.Id("R", 10),
		App:     app,
		Created: time.Now().UTC(),
	}

	rs, err := p.ReleaseList(app, structs.ReleaseListOptions{Limit: options.Int(1)})
	if err != nil {
		return nil, err
	}

	if len(rs) > 0 {
		r.Build = rs[0].Build
		r.Env = rs[0].Env
	}

	return r, nil
}

func (p *Provider) releaseList(app string) (structs.Releases, error) {
	c, err := p.convoxClient()
	if err != nil {
		return nil, err
	}

	krs, err := c.ConvoxV1().Releases(p.AppNamespace(app)).List(am.ListOptions{})
	if err != nil {
		return nil, err
	}

	rs := structs.Releases{}

	for _, kr := range krs.Items {
		r, err := p.releaseUnmarshal(&kr)
		if err != nil {
			return nil, err
		}

		rs = append(rs, *r)
	}

	return rs, nil
}

func (p *Provider) releaseMarshal(r *structs.Release) *ca.Release {
	return &ca.Release{
		ObjectMeta: am.ObjectMeta{
			Namespace: p.AppNamespace(r.App),
			Name:      strings.ToLower(r.Id),
			Labels: map[string]string{
				"system": "convox",
				"rack":   p.Name,
				"app":    r.App,
			},
		},
		Spec: ca.ReleaseSpec{
			Build:    r.Build,
			Created:  r.Created.Format(common.SortableTime),
			Env:      r.Env,
			Manifest: r.Manifest,
		},
	}
}

func (p *Provider) releaseTemplateApp(a *structs.App, opts structs.ReleasePromoteOptions) ([]byte, error) {
	owner, err := p.Cluster.CoreV1().Namespaces().Get(p.Namespace, am.GetOptions{})
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"Locked":     a.Locked,
		"Name":       a.Name,
		"Namespace":  p.AppNamespace(a.Name),
		"Owner":      owner,
		"Parameters": a.Parameters,
	}

	data, err := p.RenderTemplate("app/app", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateBalancer(a *structs.App, r *structs.Release, b manifest.Balancer) ([]byte, error) {
	params := map[string]interface{}{
		"Balancer":  b,
		"Namespace": p.AppNamespace(a.Name),
		"Release":   r,
	}

	data, err := p.RenderTemplate("app/balancer", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateCA(a *structs.App, ca *v1.Secret) ([]byte, error) {
	params := map[string]interface{}{
		"CA":        base64.StdEncoding.EncodeToString(ca.Data["tls.crt"]),
		"Namespace": p.AppNamespace(a.Name),
	}

	data, err := p.RenderTemplate("app/ca", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateIngress(a *structs.App, ss manifest.Services, opts structs.ReleasePromoteOptions) ([]byte, error) {
	ans, err := p.Engine.IngressAnnotations(a.Name)
	if err != nil {
		return nil, err
	}

	iss, err := p.ingressSecrets(a, ss)
	if err != nil {
		return nil, err
	}

	idles, err := p.Engine.AppIdles(a.Name)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"Annotations": ans,
		"App":         a.Name,
		"Idles":       common.DefaultBool(opts.Idle, idles),
		"Namespace":   p.AppNamespace(a.Name),
		"Secrets":     iss,
		"Services":    ss,
	}

	data, err := p.RenderTemplate("app/ingress", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateResource(a *structs.App, r manifest.Resource) ([]byte, error) {
	params := map[string]interface{}{
		"App":        a.Name,
		"Namespace":  p.AppNamespace(a.Name),
		"Name":       r.Name,
		"Parameters": r.Options,
		"Password":   fmt.Sprintf("%x", sha256.Sum256([]byte(p.Name)))[0:30],
		"Rack":       p.Name,
	}

	data, err := p.RenderTemplate(fmt.Sprintf("resource/%s", r.Type), params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateServices(a *structs.App, e structs.Environment, r *structs.Release, ss manifest.Services, opts structs.ReleasePromoteOptions) ([]byte, error) {
	items := [][]byte{}

	pss, err := p.ServiceList(a.Name)
	if err != nil {
		return nil, err
	}

	sc := map[string]int{}

	for _, s := range pss {
		sc[s.Name] = s.Count
	}

	for _, s := range ss {
		min := 50
		max := 200

		if s.Agent.Enabled || s.Singleton {
			min = 0
			max = 100
		}

		if opts.Min != nil {
			min = *opts.Min
		}

		if opts.Max != nil {
			max = *opts.Max
		}

		replicas := common.CoalesceInt(sc[s.Name], s.Scale.Count.Min)

		env, err := p.environment(a, r, s, e)
		if err != nil {
			return nil, err
		}

		params := map[string]interface{}{
			"App":            a,
			"Environment":    env,
			"MaxSurge":       max,
			"MaxUnavailable": 100 - min,
			"Namespace":      p.AppNamespace(a.Name),
			"Password":       p.Password,
			"Rack":           p.Name,
			"Release":        r,
			"Replicas":       replicas,
			"Service":        s,
		}

		if ip, err := p.Engine.ResolverHost(); err == nil {
			params["Resolver"] = ip
		}

		data, err := p.RenderTemplate("app/service", params)
		if err != nil {
			return nil, err
		}

		items = append(items, data)
	}

	return bytes.Join(items, []byte("---\n")), nil
}

func (p *Provider) releaseTemplateTimer(a *structs.App, r *structs.Release, s *manifest.Service, t manifest.Timer) ([]byte, error) {
	params := map[string]interface{}{
		"App":       a,
		"Namespace": p.AppNamespace(a.Name),
		"Rack":      p.Name,
		"Release":   r,
		"Service":   s,
		"Timer":     t,
	}

	if ip, err := p.Engine.ResolverHost(); err == nil {
		params["Resolver"] = ip
	}

	data, err := p.RenderTemplate("app/timer", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseTemplateVolumes(a *structs.App, ss manifest.Services) ([]byte, error) {
	vsh := map[string]bool{}

	for _, s := range ss {
		for _, v := range p.volumeSources(a.Name, s.Name, s.Volumes) {
			if !systemVolume(v) {
				vsh[v] = true
			}
		}
	}

	vs := []string{}

	for s := range vsh {
		vs = append(vs, s)
	}

	params := map[string]interface{}{
		"App":       a.Name,
		"Namespace": p.AppNamespace(a.Name),
		"Rack":      p.Name,
		"Volumes":   vs,
	}

	data, err := p.RenderTemplate("app/volumes", params)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) releaseUnmarshal(kr *ca.Release) (*structs.Release, error) {
	created, err := time.Parse(common.SortableTime, kr.Spec.Created)
	if err != nil {
		return nil, err
	}

	r := &structs.Release{
		App:      kr.ObjectMeta.Labels["app"],
		Build:    kr.Spec.Build,
		Created:  created,
		Env:      kr.Spec.Env,
		Id:       strings.ToUpper(kr.ObjectMeta.Name),
		Manifest: kr.Spec.Manifest,
	}

	if len(r.Env) == 0 {
		if s, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(r.App)).Get(fmt.Sprintf("release-%s", kr.ObjectMeta.Name), am.GetOptions{}); err == nil {
			e := structs.Environment{}

			for k, v := range s.Data {
				e[k] = string(v)
			}

			r.Env = e.String()
		}
	}

	return r, nil
}
