package k8s

import (
	"bytes"
	"context"
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
	"github.com/convox/convox/provider/aws/provisioner"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	"github.com/pkg/errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	v1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ReleaseCreate(app string, opts structs.ReleaseCreateOptions) (*structs.Release, error) {
	r, err := p.releaseFork(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if opts.Build != nil {
		r.Build = *opts.Build
	}

	if r.Build != "" {
		b, err := p.BuildGet(app, r.Build)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		r.Description = b.Description
		r.Manifest = b.Manifest
	}

	if opts.Env != nil {
		desc, err := common.EnvDiff(r.Env, *opts.Env)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if strings.TrimSpace(desc) != "" {
			r.Description = fmt.Sprintf("env %s", desc)
		}
		r.Env = *opts.Env
	}

	if opts.Description != nil {
		r.Description = *opts.Description
	}

	ro, err := p.releaseCreate(r)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.EventSend("release:create", structs.EventSendOptions{Data: map[string]string{"app": ro.App, "id": ro.Id}})

	return ro, nil
}

func (p *Provider) ReleaseGet(app, id string) (*structs.Release, error) {
	r, err := p.releaseGet(app, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return r, nil
}

func (p *Provider) ReleaseList(app string, opts structs.ReleaseListOptions) (structs.Releases, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, errors.WithStack(err)
	}

	rs, err := p.releaseList(app)
	if err != nil {
		return nil, errors.WithStack(err)
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
		return errors.WithStack(err)
	}

	items := [][]byte{}
	dependencies := []string{}

	// app
	data, err := p.releaseTemplateApp(a, opts)
	if err != nil {
		return errors.WithStack(err)
	}

	items = append(items, data)

	// ca
	if ca, err := p.Cluster.CoreV1().Secrets("convox-system").Get(context.TODO(), "ca", am.GetOptions{}); err == nil {
		data, err := p.releaseTemplateCA(a, ca)
		if err != nil {
			return errors.WithStack(err)
		}

		items = append(items, data)
	}

	if id != "" {
		m, r, err := common.ReleaseManifest(p, app, id)
		if err != nil {
			return errors.WithStack(err)
		}

		e, err := structs.NewEnvironment([]byte(r.Env))
		if err != nil {
			return errors.WithStack(err)
		}

		// balancers
		for _, b := range m.Balancers {
			data, err := p.releaseTemplateBalancer(a, r, b, m.Labels)
			if err != nil {
				return errors.WithStack(err)
			}

			items = append(items, data)
		}

		// ingress
		if rss := m.Services.Routable().External(); len(rss) > 0 {
			data, err := p.releaseTemplateIngress(a, rss, opts)
			if err != nil {
				return errors.WithStack(err)
			}

			items = append(items, data)
		}

		// ingress internal
		if rss := m.Services.Routable().InternalRouter(); len(rss) > 0 {
			if p.DomainInternal == "" {
				return errors.New("please enable the rack's internal router first: convox rack params set internal_router=true")
			}
			data, err := p.releaseTemplateIngressInternal(a, rss, opts)
			if err != nil {
				return errors.WithStack(err)
			}

			items = append(items, data)
		}

		// resources
		for _, r := range m.Resources {
			if !r.IsCustomManagedResource() {
				data, err := p.releaseTemplateResource(a, e, r)
				if err != nil {
					return errors.WithStack(err)
				}

				items = append(items, data)
			}
		}

		// services
		data, err := p.releaseTemplateServices(a, e, r, m.Services, opts)
		if err != nil {
			return errors.WithStack(err)
		}

		items = append(items, data)

		// timers
		for _, t := range m.Timers {
			s, err := m.Service(t.Service)
			if err != nil {
				return errors.WithStack(err)
			}

			data, err := p.releaseTemplateTimer(a, e, r, s, t)
			if err != nil {
				return errors.WithStack(err)
			}

			items = append(items, data)
		}

		items = append(items, data)

		// rds resources
		rdsItems, rdsDeps, err := p.releaseRdsResources(a, e, m)
		if err != nil {
			return err
		}

		items = append(items, rdsItems...)
		if len(rdsDeps) > 0 {
			dependencies = append(dependencies, rdsDeps...)
		}

		// elasticache resources
		elasticacheItems, elastiDeps, err := p.releaseElasticacheResources(a, e, m)
		if err != nil {
			return err
		}

		items = append(items, elasticacheItems...)
		if len(elastiDeps) > 0 {
			dependencies = append(dependencies, elastiDeps...)
		}
	}

	tdata := bytes.Join(items, []byte("---\n"))

	timeout := int32(common.DefaultInt(opts.Timeout, 3000))

	if err := p.Apply(p.AppNamespace(app), "app", PromoteApplyConfig{
		Version:      id,
		Data:         tdata,
		Labels:       fmt.Sprintf("system=convox,provider=k8s,rack=%s,app=%s,release=%s", p.Name, app, id),
		Timeout:      timeout,
		Dependencies: dependencies,
	}); err != nil {
		return errors.WithStack(err)
	}

	p.EventSend("release:promote", structs.EventSendOptions{Data: map[string]string{"app": app, "id": id}, Status: options.String("start")})

	p.FlushStateLog(app)

	return nil
}

func (p *Provider) releaseCreate(r *structs.Release) (*structs.Release, error) {
	kr, err := p.Convox.ConvoxV1().Releases(p.AppNamespace(r.App)).Create(p.releaseMarshal(r))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.releaseUnmarshal(kr)
}

func (p *Provider) releaseGet(app, id string) (*structs.Release, error) {
	kr, err := p.Convox.ConvoxV1().Releases(p.AppNamespace(app)).Get(strings.ToLower(id), am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.releaseUnmarshal(kr)
}

func (p *Provider) releaseFork(app string) (*structs.Release, error) {
	r := structs.NewRelease(app)

	rs, err := p.ReleaseList(app, structs.ReleaseListOptions{Limit: options.Int(1)})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(rs) > 0 {
		r.Build = rs[0].Build
		r.Env = rs[0].Env
	}

	return r, nil
}

func (p *Provider) releaseList(app string) (structs.Releases, error) {
	krs, err := p.Convox.ConvoxV1().Releases(p.AppNamespace(app)).List(am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rs := structs.Releases{}

	for _, kr := range krs.Items {
		r, err := p.releaseUnmarshal(&kr)
		if err != nil {
			return nil, errors.WithStack(err)
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
			Build:       r.Build,
			Created:     r.Created.Format(common.SortableTime),
			Description: r.Description,
			Env:         r.Env,
			Manifest:    r.Manifest,
		},
	}
}

func (p *Provider) releaseTemplateApp(a *structs.App, opts structs.ReleasePromoteOptions) ([]byte, error) {
	owner, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.Namespace, am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateBalancer(a *structs.App, r *structs.Release, b manifest.Balancer, lbs manifest.Labels) ([]byte, error) {
	params := map[string]interface{}{
		"Annotations": b.AnnotationsMap(),
		"Balancer":    b,
		"Namespace":   p.AppNamespace(a.Name),
		"Release":     r,
		"Labels":      lbs,
	}

	data, err := p.RenderTemplate("app/balancer", params)
	if err != nil {
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateIngress(a *structs.App, ss manifest.Services, opts structs.ReleasePromoteOptions) ([]byte, error) {
	idles, err := p.Engine.AppIdles(a.Name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	items := [][]byte{}

	for i := range ss {
		s := ss[i]
		ans, err := p.Engine.IngressAnnotations(s.Certificate.Duration)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if s.Certificate.Id != "" {
			keys := []string{}
			for k := range ans {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if strings.HasPrefix(k, "cert-manager.io") {
					delete(ans, k)
				}
			}

			data, err := p.releaseTemplateCertSecret(a, s)
			if err != nil {
				return nil, err
			}
			items = append(items, data)
		}

		customAns := s.IngressAnnotationsMap()
		reservedAns := p.reservedNginxAnnotations()

		for k, v := range customAns {
			if !reservedAns[k] {
				ans[k] = v
			}
		}

		params := map[string]interface{}{
			"Annotations":                ans,
			"App":                        a.Name,
			"Class":                      p.Engine.IngressClass(),
			"ConvoxDomainTLSCertDisable": !p.ConvoxDomainTLSCertDisable,
			"Host":                       p.Engine.ServiceHost(a.Name, s),
			"Idles":                      common.DefaultBool(opts.Idle, idles),
			"Namespace":                  p.AppNamespace(a.Name),
			"Rack":                       p.Name,
			"Service":                    s,
		}

		data, err := p.RenderTemplate("app/ingress", params)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		items = append(items, data)
	}

	return bytes.Join(items, []byte("---\n")), nil
}

func (p *Provider) releaseTemplateIngressInternal(a *structs.App, ss manifest.Services, opts structs.ReleasePromoteOptions) ([]byte, error) {
	idles, err := p.Engine.AppIdles(a.Name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	items := [][]byte{}

	for i := range ss {
		s := ss[i]

		ans, err := p.Engine.IngressAnnotations(s.Certificate.Duration)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if s.Certificate.Id != "" {
			keys := []string{}
			for k := range ans {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if strings.HasPrefix(k, "cert-manager.io") {
					delete(ans, k)
				}
			}

			data, err := p.releaseTemplateCertSecret(a, s)
			if err != nil {
				return nil, err
			}
			items = append(items, data)
		}

		customAns := s.IngressAnnotationsMap()
		reservedAns := p.reservedNginxAnnotations()

		for k, v := range customAns {
			if !reservedAns[k] {
				ans[k] = v
			}
		}

		params := map[string]interface{}{
			"Annotations":                ans,
			"App":                        a.Name,
			"Class":                      p.Engine.IngressInternalClass(),
			"ConvoxDomainTLSCertDisable": !p.ConvoxDomainTLSCertDisable,
			"Host":                       p.Engine.ServiceHost(a.Name, s),
			"Idles":                      common.DefaultBool(opts.Idle, idles),
			"Namespace":                  p.AppNamespace(a.Name),
			"Rack":                       p.Name,
			"Service":                    s,
		}

		data, err := p.RenderTemplate("app/ingress-internal", params)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		items = append(items, data)
	}

	return bytes.Join(items, []byte("---\n")), nil
}

func (p *Provider) releaseTemplateCertSecret(a *structs.App, s manifest.Service) ([]byte, error) {
	certSecret, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(p.ctx, s.Certificate.Id, am.GetOptions{})
	if err != nil {
		return nil, err
	}

	hash, err := secretDataHash(certSecret)
	if err != nil {
		return nil, err
	}

	secretObj := &v1.Secret{
		TypeMeta: am.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: am.ObjectMeta{
			Name:      s.Certificate.Id,
			Namespace: p.AppNamespace(a.Name),
			Annotations: map[string]string{
				AnnotationSecretDataHash: hash,
			},
		},
		Data: certSecret.Data,
	}

	return SerializeK8sObjToYaml(secretObj)
}

func (p *Provider) releaseTemplateResource(a *structs.App, e structs.Environment, r manifest.Resource) ([]byte, error) {
	if url := strings.TrimSpace(e[r.DefaultEnv()]); url != "" {
		return p.releaseTemplateResourceMasked(a, r, url)
	}

	params := map[string]interface{}{
		"App":        a.Name,
		"Namespace":  p.AppNamespace(a.Name),
		"Name":       r.Name,
		"Parameters": r.Options,
		"Password":   fmt.Sprintf("%x", sha256.Sum256([]byte(p.Name)))[0:30],
		"Rack":       p.Name,
		"Image":      r.Image,
	}

	data, err := p.RenderTemplate(fmt.Sprintf("resource/%s", r.Type), params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateCustomResource(a *structs.App, e structs.Environment, r manifest.Resource, conn *provisioner.ConnectionInfo) ([]byte, error) {
	params := map[string]interface{}{
		"App":        a.Name,
		"Namespace":  p.AppNamespace(a.Name),
		"Name":       r.Name,
		"Parameters": r.Options,
		"Rack":       p.Name,
		"Host":       conn.Host,
		"Port":       conn.Port,
		"User":       conn.UserName,
		"Password":   conn.Password,
		"Database":   conn.Database,
	}

	data, err := p.RenderTemplate(fmt.Sprintf("resource/%s", r.Type), params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateResourceMasked(a *structs.App, r manifest.Resource, url string) ([]byte, error) {
	params := map[string]interface{}{
		"App":       a.Name,
		"Namespace": p.AppNamespace(a.Name),
		"Name":      r.Name,
		"Rack":      p.Name,
		"Url":       url,
	}

	data, err := p.RenderTemplate("resource/masked", params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateServices(a *structs.App, e structs.Environment, r *structs.Release, ss manifest.Services, opts structs.ReleasePromoteOptions) ([]byte, error) {
	items := [][]byte{}

	pss, err := p.ServiceList(a.Name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sc := map[string]int{}

	for _, s := range pss {
		sc[s.Name] = s.Count
	}

	for i := range ss {
		// efs
		vdata, err := p.releaseTemplateEfs(a, ss[i])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if vdata != nil {
			items = append(items, vdata)
		}

		s := ss[i]
		min := s.Deployment.Minimum
		max := s.Deployment.Maximum

		if opts.Min != nil {
			min = *opts.Min
		}

		if opts.Max != nil {
			max = *opts.Max
		}

		replicas := common.CoalesceInt(sc[s.Name], s.Scale.Count.Min)

		env, err := p.environment(a, r, s, e)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		params := map[string]interface{}{
			"Annotations":    s.AnnotationsMap(),
			"App":            a,
			"Environment":    env,
			"MaxSurge":       max - 100,
			"MaxUnavailable": 100 - min,
			"Namespace":      p.AppNamespace(a.Name),
			"Password":       p.Password,
			"Rack":           p.Name,
			"Release":        r,
			"Replicas":       replicas,
			"Resources":      s.ResourceMap(),
			"Service":        s,
		}

		if ip, err := p.Engine.ResolverHost(); err == nil {
			params["Resolver"] = ip
		}

		data, err := p.RenderTemplate("app/service", params)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		items = append(items, data)
	}

	return bytes.Join(items, []byte("---\n")), nil
}

func (p *Provider) releaseTemplateTimer(a *structs.App, e structs.Environment, r *structs.Release, s *manifest.Service, t manifest.Timer) ([]byte, error) {
	if t.Concurrency != "" {
		caser := cases.Title(language.Und, cases.NoLower)
		t.Concurrency = caser.String(t.Concurrency)
	}

	params := map[string]interface{}{
		"Annotations": t.AnnotationsMap(),
		"App":         a,
		"Namespace":   p.AppNamespace(a.Name),
		"Rack":        p.Name,
		"Release":     r,
		"Resources":   s.ResourceMap(),
		"Service":     s,
		"Timer":       t,
	}

	if ip, err := p.Engine.ResolverHost(); err == nil {
		params["Resolver"] = ip
	}

	data, err := p.RenderTemplate("app/timer", params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseTemplateEfs(a *structs.App, s manifest.Service) ([]byte, error) {
	for i := range s.VolumeOptions {
		if s.VolumeOptions[i].AwsEfs != nil && len(p.EfsFileSystemId) <= 2 {
			return nil, fmt.Errorf("efs csi driver is not enabled but efs volume is specified")
		}
		if err := s.VolumeOptions[i].Validate(); err != nil {
			return nil, err
		}
	}

	params := map[string]interface{}{
		"App":       a,
		"Namespace": p.AppNamespace(a.Name),
		"Rack":      p.Name,
		"Service":   s,
	}

	data, err := p.RenderTemplate("app/efs", params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

func (p *Provider) releaseUnmarshal(kr *ca.Release) (*structs.Release, error) {
	created, err := time.Parse(common.SortableTime, kr.Spec.Created)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	r := &structs.Release{
		App:         kr.ObjectMeta.Labels["app"],
		Build:       kr.Spec.Build,
		Created:     created,
		Description: kr.Spec.Description,
		Env:         kr.Spec.Env,
		Id:          strings.ToUpper(kr.ObjectMeta.Name),
		Manifest:    kr.Spec.Manifest,
	}

	if len(r.Env) == 0 {
		if s, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(r.App)).Get(
			context.TODO(), fmt.Sprintf("release-%s", kr.ObjectMeta.Name), am.GetOptions{},
		); err == nil {
			e := structs.Environment{}

			for k, v := range s.Data {
				e[k] = string(v)
			}

			r.Env = e.String()
		}
	}

	return r, nil
}

func (p *Provider) releaseElasticacheResources(app *structs.App, envs structs.Environment, m *manifest.Manifest) ([][]byte, []string, error) {
	items := [][]byte{}
	dependencies := []string{}
	stateMap := map[string]struct{}{}
	for _, r := range m.Resources {
		if r.IsElastiCache() {
			if err := r.ElastiCacheNameValidate(); err != nil {
				return nil, nil, err
			}

			id := p.CreateAwsResourceStateId(app.Name, r.Name)

			stateMap[id] = struct{}{}

			err := p.ElasticacheProvisioner.Provision(id, p.MapToElasticacheParameter(r.Type, app.Name, r.Options))
			if err != nil {
				return nil, nil, err
			}

			isAvailable, err := p.ElasticacheProvisioner.IsElastiCacheAvailable(id)
			if err != nil {
				return nil, nil, err
			}
			if isAvailable {
				conn, err := p.ElasticacheProvisioner.GetConnectionInfo(id)
				if err != nil {
					return nil, nil, err
				}

				data, err := p.releaseTemplateCustomResource(app, envs, r, conn)
				if err != nil {
					return nil, nil, errors.WithStack(err)
				}

				items = append(items, data)
			} else {
				substitutionId := resourceSubstitutionId(app.Name, r.Type, r.Name)
				dependencies = append(dependencies, substitutionId)
				items = append(items, []byte(substitutionId))
			}
		}
	}

	existingStateIds, err := p.ListElasticacheStateForApp(app.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get elasticache state list: %s", err)
	}

	for _, eId := range existingStateIds {
		if _, has := stateMap[eId]; !has {
			// delete the state, elasticache clean up job will delete the elasticache
			err = p.Cluster.CoreV1().Secrets(p.AppNamespace(app.Name)).Delete(p.ctx, eId, am.DeleteOptions{})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to delete elasticache state: %s", err)
			}
			p.SendStateLog(eId, "elasticache deletion is triggered")
		}
	}
	return items, dependencies, nil
}

func (p *Provider) releaseRdsResources(app *structs.App, envs structs.Environment, m *manifest.Manifest) ([][]byte, []string, error) {
	items := [][]byte{}
	dependencies := []string{}
	rdsStateMap := map[string]struct{}{}
	for _, r := range m.Resources {
		if r.IsRds() {
			if err := r.RdsNameValidate(); err != nil {
				return nil, nil, err
			}

			id := p.CreateAwsResourceStateId(app.Name, r.Name)

			rdsStateMap[id] = struct{}{}

			err := p.RdsProvisioner.Provision(id, p.MapToRdsParameter(r.Type, app.Name, r.Options))
			if err != nil {
				return nil, nil, err
			}

			isAvailable, err := p.RdsProvisioner.IsDbAvailable(id)
			if err != nil {
				return nil, nil, err
			}

			if isAvailable {
				conn, err := p.RdsProvisioner.GetConnectionInfo(id)
				if err != nil {
					return nil, nil, err
				}

				data, err := p.releaseTemplateCustomResource(app, envs, r, conn)
				if err != nil {
					return nil, nil, errors.WithStack(err)
				}

				items = append(items, data)
			} else {
				substitutionId := resourceSubstitutionId(app.Name, r.Type, r.Name)
				dependencies = append(dependencies, substitutionId)
				items = append(items, []byte(substitutionId))
			}
		}
	}

	existingStateIds, err := p.ListRdsStateForApp(app.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get rds state list: %s", err)
	}

	for _, rdsId := range existingStateIds {
		if _, has := rdsStateMap[rdsId]; !has {
			// delete the state, rds clean up job will delete the rds
			err = p.Cluster.CoreV1().Secrets(p.AppNamespace(app.Name)).Delete(p.ctx, rdsId, am.DeleteOptions{})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to delete rds state: %s", err)
			}
			p.SendStateLog(rdsId, "db instance deletion is triggered")
		}
	}
	return items, dependencies, nil
}

func (p *Provider) reservedNginxAnnotations() map[string]bool {
	return map[string]bool{
		"alb.ingress.kubernetes.io/scheme":                   true,
		"cert-manager.io/cluster-issuer":                     true,
		"cert-manager.io/duration":                           true,
		"nginx.ingress.kubernetes.io/backend-protocol":       true,
		"nginx.ingress.kubernetes.io/proxy-connect-timeout":  true,
		"nginx.ingress.kubernetes.io/proxy-read-timeout":     true,
		"nginx.ingress.kubernetes.io/proxy-send-timeout":     true,
		"nginx.ingress.kubernetes.io/server-snippet":         true,
		"nginx.ingress.kubernetes.io/affinity":               true,
		"nginx.ingress.kubernetes.io/session-cookie-name":    true,
		"nginx.ingress.kubernetes.io/ssl-redirect":           true,
		"nginx.ingress.kubernetes.io/whitelist-source-range": true,
	}
}
