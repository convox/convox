package k8s

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws/provisioner"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/pkg/errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

const (
	APP_CONFIG_KEY = "app.json"
)

func (p *Provider) ReleaseCreate(app string, opts structs.ReleaseCreateOptions) (*structs.Release, error) {
	r, err := p.releaseFork(app, opts.ParentRelease)
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

	limit := common.DefaultInt(opts.Limit, 10)

	rs, err := p.releaseList(app, limit)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sort.Slice(rs, func(i, j int) bool { return rs[j].Created.Before(rs[i].Created) })

	if len(rs) > limit {
		rs = rs[0:limit]
	}

	return rs, nil
}

func (p *Provider) ReleasePromote(app, id string, opts structs.ReleasePromoteOptions) error {
	if err := p.budgetCircuitBreakerTripped(app); err != nil {
		return errors.WithStack(err)
	}

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

		// Reject manifest-tier budget enforcement when the rack-level
		// cost accumulator is disabled. Mirrors the AppBudgetSet gate
		// (provider/k8s/budget_accumulator.go) so a developer who adds
		// a budget: block to convox.yml gets a loud, actionable error
		// rather than a deploy that silently persists unenforced config.
		//
		// Note on persistence: the manifest budget block validates here
		// but does NOT persist runtime state via AppBudgetSet. Per
		// docs/reference/primitives/app/budget.md the AppBudget primitive
		// is rack-managed and lives outside the per-app convox.yml on
		// purpose — coupling cap values to deploy lifecycles would let
		// every promote silently overwrite operator-set values, escalate
		// non-admin write tokens into admin-tier cap changes, and emit
		// noise webhook events on every redeploy. Set the budget via
		// `convox budget set` or the Console budget tab; manifest
		// runtime fields (NeverAutoShutdown, ShutdownOrder, RecoveryMode,
		// ShutdownGracePeriod, NotifyBeforeMinutes) are read fresh from
		// the manifest at simulate/tick time per
		// provider/k8s/budget_shutdown.go, so they take effect without
		// persistence.
		if err := p.requireCostTrackingForManifestBudget(m); err != nil {
			return errors.WithStack(err)
		}

		e, err := structs.NewEnvironment([]byte(r.Env))
		if err != nil {
			return errors.WithStack(err)
		}

		// docker hub auth secret (once per promote, before resource/service/timer loops)
		if p.hasDockerHubAuth() {
			if err := p.ensureDockerHubSecret(p.AppNamespace(app)); err != nil {
				return errors.WithStack(err)
			}
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
				return structs.ErrBadRequest("please enable the rack's internal router first: convox rack params set internal_router=true")
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

		if err := p.releaseAppConfigs(a, m); err != nil {
			return err
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

	// Capture context BEFORE the goroutine launch — the watcher outlives
	// the request-scoped p.ctx, so it must NOT reference p.ContextActor()
	// at later points. Mirrors build.go:600-608.
	capturedActor := p.ContextActor()

	// Read the release-id mirror that p.Apply just wrote, so the watcher
	// can detect supersession by a NEWER promote. p.Atom.Status() returns
	// (status, release-id, err); we use the release-id as the watcher's
	// supersession discriminator and compare to `convox.com/app-release`
	// (mirrored by the AtomController) on each tick. Fallback to the
	// inbound `id` parameter if Atom.Status() fails — same value Apply
	// just wrote, so the version-mismatch check still works correctly.
	_, atomVer, _ := p.Atom.Status(p.AppNamespace(app), "app")
	if atomVer == "" {
		atomVer = id
	}

	// Persist watch state BEFORE emitting :start. If a fast-fail occurs
	// between annotation-write and goroutine-launch, the cold-start GC
	// scan at next api-pod startup recovers it (timeout path).
	// Annotation-write failures are logged but do NOT block promote
	// success — the rollout itself proceeded; the user just doesn't
	// get a second event.
	state := structs.ReleasePromoteWatchState{
		SchemaVersion: 1,
		ReleaseID:     id,
		AtomVersion:   atomVer,
		StartedAt:     time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(time.Duration(timeout) * time.Second),
		Actor:         capturedActor,
	}
	if err := p.writeReleasePromoteWatchAnnotation(p.ctx, app, &state); err != nil {
		fmt.Printf("ns=release_watcher at=warn kind=annotation_write app=%s err=%q\n", app, err)
	}

	// :start emit uses the captured actor (audit-trail consistency with
	// the future app:promote:completed / app:promote:errored /
	// app:promote:cancelled events the watcher will emit). The action
	// name `release:promote` is preserved verbatim — existing prior art
	// that webhook consumers / audit-log scrapers depend on. New event
	// types use the canonical app:<resource>:<verb> form.
	_ = p.EventSend("release:promote", structs.EventSendOptions{
		Data:   map[string]string{"app": app, "id": id, "actor": capturedActor},
		Status: options.String("start"),
	})

	p.FlushStateLog(app)

	// Per-(app, release-id) lock — sync.Map.LoadOrStore is the atomic
	// check-and-set primitive. If a watcher is already in-flight for
	// this exact pair, the second promote skips the goroutine launch
	// (the existing watcher continues; it will see the supersession
	// via the release annotation mismatch on its next tick).
	if acquired, release := tryAcquireWatchSlot(app, id); acquired {
		s := state // own a heap copy so the goroutine doesn't alias
		go p.runReleasePromoteWatcher(p.ctx, app, &s, release)
	}

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
	kr, err := p.GetReleaseFromInformer(strings.ToLower(id), p.AppNamespace(app))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.releaseUnmarshal(kr)
}

func (p *Provider) releaseFork(app string, parentRelease *string) (*structs.Release, error) {
	r := structs.NewRelease(app)

	if parentRelease != nil && *parentRelease != "" {
		pr, err := p.releaseGet(app, *parentRelease)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		r.Build = pr.Build
		r.Env = pr.Env

		return r, nil
	}

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

func (p *Provider) releaseList(app string, limit int) (structs.Releases, error) {
	krs, err := p.ListReleasesFromInformer(p.AppNamespace(app), "", limit)
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
	owner, err := p.GetNamespaceFromInformer(p.Namespace)
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
	if p.FeatureGates[options.FeatureGateBalancerDisable] {
		return nil, structs.ErrBadRequest("balancer resource is disabled")
	}
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
			"Host":                       p.ServiceHost(a.Name, s),
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
			"Host":                       p.ServiceHost(a.Name, s),
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
		"App":           a.Name,
		"Namespace":     p.AppNamespace(a.Name),
		"Name":          r.Name,
		"Parameters":    r.Options,
		"Password":      fmt.Sprintf("%x", sha256.Sum256([]byte(p.Name)))[0:30],
		"Rack":          p.Name,
		"Image":         r.Image,
		"DockerHubAuth": p.hasDockerHubAuth(),
	}

	if r.Image == "" && p.EcrDockerHubCachePrefix != "" {
		if img := ecrCachedResourceImage(p.EcrDockerHubCachePrefix, r.Type, r.Options); img != "" {
			params["Image"] = img
		}
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

	// Populate per-service scale-override map from Service.ScaleOverrideActive
	// already populated by ServiceList above. When the annotation is strict-
	// literal "true", releaseTemplateServices preserves the runtime replica
	// count and skips the yaml-declared scale.count.min on this promote.
	// ServiceList's own populate path tolerates informer error (continue-safe);
	// services missing from pss inherit overrideActive[name]=false (the safe
	// default). The override path must NEVER cause a promote to fail outright.
	overrideActive := map[string]bool{}
	for i := range pss {
		if pss[i].ScaleOverrideActive != nil && *pss[i].ScaleOverrideActive {
			overrideActive[pss[i].Name] = true
		}
	}

	// Per-service Console-driven triggers-override state. When set, the
	// release loop must skip materialization of the manifest-declared
	// autoscaler so the user's Console-set HPA/SO survives the deploy.
	// Read directly off Deployment annotations: ServiceList projection
	// adds a TriggersOverrideActive field, but we read here as well so
	// the deploy controller behaves correctly on pre-projection paths
	// (e.g. when the rack was rolled without a fresh ServiceList sync).
	triggersOverride := map[string]bool{}
	for i := range pss {
		dep, err := p.Cluster.AppsV1().Deployments(p.AppNamespace(a.Name)).Get(context.TODO(), pss[i].Name, am.GetOptions{})
		if err != nil {
			continue
		}
		if dep.Annotations[ServiceTriggersOverrideAnnotation] == ServiceTriggersOverrideValueOn {
			triggersOverride[pss[i].Name] = true
		}
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

		// azure files
		afdata, err := p.releaseTemplateAzureFiles(a, &ss[i])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if afdata != nil {
			items = append(items, afdata)
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

		var replicas int
		if overrideActive[s.Name] {
			// Annotation honored: preserve the runtime replica count
			// regardless of yaml scale.count.min. If
			// sc[s.Name] is 0 (rare: user scaled to 0 with
			// override active), preserve 0 — explicit user choice,
			// not yaml fallback. Distinct from the CoalesceInt default
			// where 0 falls through to yaml min.
			replicas = sc[s.Name]
			// Audit-trail emit so users see in their event stream
			// that yaml scale was deliberately skipped on this promote.
			// Event-name format app:<resource>:<verb> — matches
			// app:budget:set / :cap / :threshold precedent. Service
			// identity carried in data.service, NOT embedded in the
			// event name (preserves grep/filter patterns keyed on the
			// 3-part colon scheme). actor="system" matches the
			// release:autoscale-disabled and release:manifest-advisory
			// system-emit convention. Cardinality bounded by service
			// count which is operator-controlled.
			_ = p.EventSend("app:scale-override:honored", structs.EventSendOptions{
				Data: map[string]string{
					"actor":           "system",
					"app":             a.Name,
					"service":         s.Name,
					"release":         r.Id,
					"preserved_count": strconv.Itoa(sc[s.Name]),
					"yaml_count_min":  strconv.Itoa(s.Scale.Count.Min),
				},
			})
		} else {
			// Existing default — runtime count wins when non-zero,
			// else yaml min.
			replicas = common.CoalesceInt(sc[s.Name], s.Scale.Count.Min)
		}

		env, err := p.environment(a, r, s, e)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		wantsAutoscale := s.Scale.Autoscale.IsEnabled() || s.Scale.IsKedaEnabled()
		// Triggers-override services keep their Console-driven autoscaler
		// across deploys. Skip manifest-driven autoscale materialization
		// for them so the rack does not fight a user's Console setup.
		// The template-side gate further down (TriggersOverrideActive
		// passed into params) ensures the HPA template branch is also
		// suppressed.
		if triggersOverride[s.Name] {
			wantsAutoscale = false
		}
		// Agent services render as DaemonSets; KEDA ScaledObject only targets
		// Deployments, so skip the autoscale path entirely. The
		// pkg/manifest validator emits a stderr WARNING for this combo
		// rather than a hard-fail; emit a matching release-time audit
		// event so the user's webhook stream reflects the runtime
		// ignore decision (parallel to release:autoscale-disabled and
		// release:prometheus-skipped). actor=system because no caller
		// identity is in scope at the release path.
		if s.Agent.Enabled && wantsAutoscale {
			wantsAutoscale = false
			_ = p.EventSend("release:agent-autoscale-ignored", structs.EventSendOptions{
				Data: map[string]string{
					"actor":   "system",
					"app":     a.Name,
					"service": s.Name,
					"release": r.Id,
				},
			})
		}
		if !p.IsKedaEnabled && wantsAutoscale {
			_ = p.EventSend("release:autoscale-disabled", structs.EventSendOptions{
				Data: map[string]string{
					"actor":   "system",
					"app":     a.Name,
					"service": s.Name,
					"reason":  "rack has keda_enable=false; autoscale ignored, using Count.Min static replicas",
				},
			})
			wantsAutoscale = false
		}

		if !s.Agent.Enabled && s.Scale.Min != nil && *s.Scale.Min == 0 && !s.Scale.Autoscale.IsEnabled() && !s.Scale.IsKedaEnabled() {
			_ = p.EventSend("release:manifest-advisory", structs.EventSendOptions{
				Data: map[string]string{
					"actor":   "system",
					"app":     a.Name,
					"service": s.Name,
					"reason":  "scale.min=0 without autoscale fields will keep the service at zero replicas permanently; set scale.autoscale.cpu.threshold or equivalent to enable scale-up",
				},
			})
		}

		ipsBlocks, ipsNames, err := renderImagePullSecrets(a.Name, p.AppNamespace(a.Name), &s, func(k string) (string, bool) {
			v, ok := env[k]
			return v, ok
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		items = append(items, ipsBlocks...)

		params := map[string]interface{}{
			"Annotations":            s.AnnotationsMap(),
			"App":                    a,
			"Environment":            env,
			"MaxSurge":               max - 100,
			"MaxUnavailable":         100 - min,
			"Namespace":              p.AppNamespace(a.Name),
			"Password":               p.Password,
			"Rack":                   p.Name,
			"Release":                r,
			"Replicas":               replicas,
			"Resources":              s.ResourceMap(),
			"Service":                s,
			"KedaIsEnabled":          s.Scale.IsKedaEnabled() && !s.Agent.Enabled,
			"AutoscaleIsEnabled":     s.Scale.Autoscale.IsEnabled() && !s.Agent.Enabled,
			"TriggersOverrideActive": triggersOverride[s.Name],
			"DockerHubAuth":          p.hasDockerHubAuth(),
			"ImagePullSecretNames":   ipsNames,
		}

		if ip, err := p.Engine.ResolverHost(); err == nil {
			params["Resolver"] = ip
		}

		if options.GetFeatureGates()[options.FeatureGateExternalDnsResolver] {
			params["DisableDnsSearches"] = true
		}

		data, err := p.RenderTemplate("app/service", params)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		items = append(items, data)

		if wantsAutoscale && p.IsKedaEnabled && s.Scale.Count.Max > s.Scale.Count.Min {
			promURL := os.Getenv("PROMETHEUS_URL")
			if promURL == "" && s.Scale.Autoscale.NeedsPrometheus() {
				// Service requires Prometheus (gpu-utilization or queue-depth
				// autoscale without an explicit per-trigger prometheusUrl) but
				// PROMETHEUS_URL is empty. Skip the ScaledObject build for this
				// service and emit an audit-trail event. Non-Prometheus triggers
				// (cpu, memory, manual scale.keda.triggers) take the else branch
				// below and render normally. VPA rendering further down also
				// continues unconditionally for this service — the skip is scoped
				// to the ScaledObject only, not the entire iteration.
				skippedStatus := "skipped"
				_ = p.EventSend("release:prometheus-skipped", structs.EventSendOptions{
					Data: map[string]string{
						"actor":   "system",
						"app":     a.Name,
						"service": s.Name,
						"reason":  "PROMETHEUS_URL not set; skipping KEDA prometheus trigger creation",
					},
					Status: &skippedStatus,
				})
			} else {
				var triggers []kedav1alpha1.ScaleTriggers
				if s.Scale.Autoscale.IsEnabled() {
					triggers = append(triggers, s.Scale.Autoscale.BuildTriggers(a.Name, s.Name, promURL)...)
				}
				if s.Scale.IsKedaEnabled() {
					triggers = append(triggers, s.Scale.Keda.Triggers...)
				}

				// MinCount / MaxCount are int in the manifest type but int32 in
				// KEDA's ScaledObject spec (mirrors upstream `*int32`). Clamp
				// rather than silently truncating: replica counts above
				// MaxInt32 are operationally nonsensical (Kubernetes nodes
				// would be exhausted long before), and silent truncation via
				// two's-complement could flip large values to small or
				// negative. Surface intent by clamping to the int32 range.
				clampReplica := func(n int) int32 {
					if n < 0 {
						return 0
					}
					if n > math.MaxInt32 {
						return math.MaxInt32
					}
					return int32(n)
				}
				scaledObj := s.KedaScaledObject(manifest.KedaScaledObjectParameters{
					ServiceName: s.Name,
					Namespace:   p.AppNamespace(a.Name),
					MinCount:    clampReplica(s.Scale.Count.Min),
					MaxCount:    clampReplica(s.Scale.Count.Max),
					Triggers:    triggers,
				})
				if scaledObj != nil {
					if p.applyAnnotationsToHPA(a.Name, s.Name, map[string]string{
						"validations.keda.sh/hpa-ownership": "false",
					}) != nil {
						return nil, fmt.Errorf("failed to apply annotations to HPA for service %s", s.Name)
					}

					scaledObj.Labels = map[string]string{
						"system":  "convox",
						"rack":    p.Name,
						"app":     a.Name,
						"service": s.Name,
					}
					soData, err := SerializeK8sObjToYaml(scaledObj)
					if err != nil {
						return nil, errors.WithStack(err)
					}

					if defaultAuth := s.DefaultTriggerAuthentionIfAws(p.AppNamespace(a.Name)); defaultAuth != nil {
						authData, err := SerializeK8sObjToYaml(defaultAuth)
						if err != nil {
							return nil, errors.WithStack(err)
						}
						items = append(items, authData)
					}

					items = append(items, soData)
				}
			}
		}

		if s.Scale.IsVpaEnabled() {
			if !p.IsVpaEnabled {
				return nil, structs.ErrBadRequest("vpa is not enabled on the rack")
			}
			vpaObj, err := s.Scale.VPA.VpaObject(s.Name, p.AppNamespace(a.Name), map[string]string{
				"system":  "convox",
				"rack":    p.Name,
				"app":     a.Name,
				"service": s.Name,
			})
			if err != nil {
				return nil, errors.WithStack(err)
			}
			vpaData, err := SerializeK8sObjToYaml(vpaObj)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			items = append(items, vpaData)
		}
	}

	return bytes.Join(items, []byte("---\n")), nil
}

func (p *Provider) releaseTemplateTimer(a *structs.App, e structs.Environment, r *structs.Release, s *manifest.Service, t manifest.Timer) ([]byte, error) {
	if t.Concurrency != "" {
		caser := cases.Title(language.Und, cases.NoLower)
		t.Concurrency = caser.String(t.Concurrency)
	}

	params := map[string]interface{}{
		"Annotations":          t.AnnotationsMap(),
		"App":                  a,
		"Namespace":            p.AppNamespace(a.Name),
		"Rack":                 p.Name,
		"Release":              r,
		"Resources":            s.ResourceMap(),
		"Service":              s,
		"Timer":                t,
		"DockerHubAuth":        p.hasDockerHubAuth(),
		"ImagePullSecretNames": imagePullSecretNames(a.Name, s.Name, s.ImagePullSecrets),
	}

	if ip, err := p.Engine.ResolverHost(); err == nil {
		params["Resolver"] = ip
	}

	if options.GetFeatureGates()[options.FeatureGateExternalDnsResolver] {
		params["DisableDnsSearches"] = true
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
			return nil, structs.ErrBadRequest("efs csi driver is not enabled but efs volume is specified")
		}
		if s.VolumeOptions[i].AwsEfs != nil && s.VolumeOptions[i].AwsEfs.VolumeHandle != "" {
			s.VolumeOptions[i].AwsEfs.ProcessTemplate(p.EfsFileSystemId, a.Name, s.Name)
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

func (p *Provider) releaseTemplateAzureFiles(a *structs.App, s *manifest.Service) ([]byte, error) {
	hasAzureFiles := false
	for i := range s.VolumeOptions {
		if s.VolumeOptions[i].AzureFiles != nil {
			hasAzureFiles = true
			if p.AzureFilesEnabled != "true" {
				return nil, structs.ErrBadRequest("azure files is not enabled but azureFiles volume is specified")
			}
		}
		if err := s.VolumeOptions[i].Validate(); err != nil {
			return nil, err
		}
	}

	if !hasAzureFiles {
		return nil, nil
	}

	params := map[string]interface{}{
		"App":       a,
		"Namespace": p.AppNamespace(a.Name),
		"Rack":      p.Name,
		"Service":   s,
	}

	data, err := p.RenderTemplate("app/azurefiles", params)
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
			if p.FeatureGates[options.FeatureGateElasticacheDisable] {
				return nil, nil, structs.ErrBadRequest("elasticache resource is disabled")
			}
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
			if p.FeatureGates[options.FeatureGateRdsDisable] {
				return nil, nil, structs.ErrBadRequest("rds resource is disabled")
			}
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

func (p *Provider) releaseAppConfigs(app *structs.App, m *manifest.Manifest) error {
	if len(m.Configs) == 0 {
		return nil
	}

	for i := range m.Configs {
		name := common.CoalesceString(m.Configs[i].Name, p.GenConfigName(m.Configs[i].Id))
		_, err := p.CreateOrPatchSecret(p.ctx, am.ObjectMeta{
			Name:      name,
			Namespace: p.AppNamespace(app.Name),
			Labels: map[string]string{
				"system": "convox",
				"type":   "config",
				"app":    app.Name,
			},
		}, func(s *v1.Secret) *v1.Secret {
			if m.Configs[i].Value != nil {
				s.Data = map[string][]byte{
					APP_CONFIG_KEY: []byte(*m.Configs[i].Value),
				}
			} else if _, has := s.Data[APP_CONFIG_KEY]; !has {
				s.Data = map[string][]byte{
					APP_CONFIG_KEY: []byte(""),
				}
			}

			if s.Labels == nil {
				s.Labels = map[string]string{}
			}

			s.Labels["app"] = app.Name
			s.Labels["system"] = "convox"
			s.Labels["type"] = "config"
			return s
		}, am.PatchOptions{
			FieldManager: "convox",
		})
		if err != nil {
			return fmt.Errorf("failed to create/update config: %s", err)
		}
	}

	return nil
}

func (p *Provider) applyAnnotationsToHPA(app string, service string, annotations map[string]string) error {
	// Prepare patch payload for metadata.annotations
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = p.Cluster.AutoscalingV2().HorizontalPodAutoscalers(p.AppNamespace(app)).Patch(
		p.ctx,
		service,
		ktypes.MergePatchType,
		patchBytes,
		am.PatchOptions{},
	)
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	return nil
}

// resourceDefaultImages maps resource types to their default Docker Hub image references.
// Library images use just the name (e.g. "redis"), non-library images include the org (e.g. "postgis/postgis").
var resourceDefaultImages = map[string]struct {
	image          string
	defaultVersion string
	isLibrary      bool
}{
	"redis":     {image: "redis", defaultVersion: "4.0.10", isLibrary: true},
	"postgres":  {image: "postgres", defaultVersion: "10.5", isLibrary: true},
	"mysql":     {image: "mysql", defaultVersion: "5.7.23", isLibrary: true},
	"mariadb":   {image: "mariadb", defaultVersion: "10.6.0", isLibrary: true},
	"memcached": {image: "memcached", defaultVersion: "1.4.34", isLibrary: true},
	"postgis":   {image: "postgis/postgis", defaultVersion: "10-3.2", isLibrary: false},
}

// ecrCachedResourceImage returns the ECR pull-through cache URL for a resource's Docker Hub image.
// Returns empty string if the resource type is not recognized.
func ecrCachedResourceImage(prefix, resourceType string, options map[string]string) string {
	info, ok := resourceDefaultImages[resourceType]
	if !ok {
		return ""
	}

	version := info.defaultVersion
	if v, ok := options["version"]; ok && v != "" {
		version = v
	}

	imagePath := info.image
	if info.isLibrary {
		imagePath = "library/" + imagePath
	}

	return fmt.Sprintf("%s/%s:%s", strings.TrimRight(prefix, "/"), imagePath, version)
}

// requireCostTrackingForManifestBudget rejects a manifest budget block whose
// enforcement-bearing fields (MonthlyCapUsd, AlertThresholdPercent,
// AtCapAction) are set when the rack-level cost accumulator is disabled.
// Mirrors AppBudgetSet's gate. PricingAdjustment alone does not trigger —
// it modifies the pricing-model output but does not depend on the
// accumulator running.
func (p *Provider) requireCostTrackingForManifestBudget(m *manifest.Manifest) error {
	if m == nil {
		return nil
	}
	enforcement := m.Budget.MonthlyCapUsd > 0 ||
		m.Budget.AlertThresholdPercent > 0 ||
		m.Budget.AtCapAction != ""
	if !enforcement {
		return nil
	}
	if !p.costTrackingEnabled() {
		return structs.ErrUnprocessable(
			"convox.yml budget: block requires cost_tracking_enable=true on the rack.\n" +
				"  Set on the rack first:\n" +
				"    convox rack params set cost_tracking_enable=true\n" +
				"  Wait ~3 min for the apply to complete, then redeploy.",
		)
	}
	return nil
}
