package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (p *Provider) AppCancel(name string) error {
	if _, err := p.AppGet(name); err != nil {
		return errors.WithStack(err)
	}

	if err := p.Atom.Cancel(p.AppNamespace(name), "app"); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) AppCreate(name string, opts structs.AppCreateOptions) (*structs.App, error) {
	if err := p.appNameValidate(name); err != nil {
		return nil, errors.WithStack(err)
	}

	ns := &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: p.AppNamespace(name),
			Annotations: map[string]string{
				"convox.com/lock":   "false",
				"convox.com/params": "{}",
			},
			Labels: map[string]string{
				"name": name,
				"type": "app",
			},
		},
	}

	if p.ContextTID() != "" {
		ns.ObjectMeta.Labels["tid"] = p.ContextTID()
	}

	if _, err := p.Cluster.CoreV1().Namespaces().Create(context.TODO(), ns, am.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	a := &structs.App{
		Name:       name,
		Parameters: p.Engine.AppParameters(),
	}

	if err := p.appUpdate(a); err != nil {
		return nil, errors.WithStack(err)
	}

	timeout := common.DefaultInt(opts.Timeout, 300)
	if err := p.ReleasePromote(a.Name, "", structs.ReleasePromoteOptions{Timeout: &timeout}); err != nil {
		return nil, errors.WithStack(err)
	}

	a, err := p.AppGet(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.EventSend("app:create", structs.EventSendOptions{Data: map[string]string{"name": name}})

	return a, nil
}

func (p *Provider) AppConfigGet(app, name string) (*structs.AppConfig, error) {
	cfg, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).Get(context.TODO(), p.GenConfigName(name), am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if ae.IsNotFound(err) {
		return nil, errors.WithStack(fmt.Errorf("app config not found: %s", name))
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c := &structs.AppConfig{
		Name: name,
	}
	if cfg.Data != nil {
		c.Value = string(cfg.Data[APP_CONFIG_KEY])
	}

	return c, nil
}

func (p *Provider) AppConfigList(app string) ([]structs.AppConfig, error) {
	resp, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).List(context.TODO(), am.ListOptions{
		LabelSelector: "system=convox,type=config",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cfgs := []structs.AppConfig{}
	for i := range resp.Items {
		cfg := &resp.Items[i]
		c := structs.AppConfig{
			Name: strings.TrimPrefix(cfg.Name, "cfg-"),
		}
		if cfg.Data != nil {
			c.Value = string(cfg.Data[APP_CONFIG_KEY])
		}

		cfgs = append(cfgs, c)
	}

	return cfgs, nil
}

func (p *Provider) AppConfigSet(app, name, valueBase64 string) error {
	data, err := base64.StdEncoding.DecodeString(valueBase64)
	if err != nil {
		return fmt.Errorf("failed to parse base 64 vaule: %s", err)
	}

	_, err = p.CreateOrPatchSecret(p.ctx, am.ObjectMeta{
		Name:      p.GenConfigName(name),
		Namespace: p.AppNamespace(app),
	}, func(s *ac.Secret) *ac.Secret {
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		s.Labels["app"] = app
		s.Labels["system"] = "convox"
		s.Labels["type"] = "config"

		s.Data = map[string][]byte{
			APP_CONFIG_KEY: data,
		}

		return s
	}, am.PatchOptions{FieldManager: "convox"})
	if err != nil {
		return fmt.Errorf("failed to set config: %s", err)
	}
	return err
}

func (p *Provider) AppDelete(name string) error {
	a, err := p.AppGet(name)
	if err != nil {
		return errors.WithStack(err)
	}

	if a.Locked {
		return errors.WithStack(fmt.Errorf("app is locked: %s", name))
	}

	if err := p.Cluster.CoreV1().Namespaces().Delete(context.TODO(), p.AppNamespace(name), am.DeleteOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) AppGet(name string) (*structs.App, error) {
	ns, err := p.GetNamespaceFromInformer(p.AppNamespace(name))
	if ae.IsNotFound(err) {
		return nil, errors.WithStack(fmt.Errorf("app not found: %s", name))
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}

	a, err := p.appFromNamespace(*ns)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return a, nil
}

func (p *Provider) AppIdles(name string) (bool, error) {
	return false, nil
}

func (p *Provider) AppList() (structs.Apps, error) {
	selector := fmt.Sprintf("system=convox,rack=%s,type=app", p.Name)
	if p.ContextTID() != "" {
		selector = fmt.Sprintf("%s,tid=%s", selector, p.ContextTID())
	}
	ns, err := p.ListNamespacesFromInformer(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	as := structs.Apps{}

	for _, n := range ns.Items {
		a, err := p.appFromNamespaceOnly(n)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		as = append(as, *a)
	}

	return as, nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) AppMetrics(name string, opts structs.MetricsOptions) (structs.Metrics, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) AppNamespace(app string) string {
	switch app {
	case "system":
		return p.Namespace
	default:
		if p.ContextTID() != "" {
			return fmt.Sprintf("%s-%s-%s", p.Name, p.ContextTID(), app)
		}
		return fmt.Sprintf("%s-%s", p.Name, app)
	}
}

func (p *Provider) NamespaceApp(namespace string) (string, error) {
	ns, err := p.GetNamespaceFromInformer(namespace)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if app, ok := ns.ObjectMeta.Labels["app"]; ok && strings.TrimSpace(app) != "" {
		return app, nil
	}

	return "", errors.WithStack(fmt.Errorf("could not determine app for namespace: %s", namespace))
}

func (p *Provider) AppParameters() map[string]string {
	return map[string]string{
		structs.AppParamBuildCpu:    "",
		structs.AppParamBuildMem:    "",
		structs.AppParamBuildLabels: "",
	}
}

func (p *Provider) AppUpdate(name string, opts structs.AppUpdateOptions) error {
	a, err := p.AppGet(name)
	if err != nil {
		return errors.WithStack(err)
	}

	if opts.Lock != nil {
		a.Locked = *opts.Lock
	}

	if opts.Parameters != nil {
		if err := p.appParametersUpdate(a, opts.Parameters); err != nil {
			return errors.WithStack(err)
		}
	}

	if err := p.appUpdate(a); err != nil {
		return errors.WithStack(err)
	}

	if err := p.ReleasePromote(a.Name, a.Release, structs.ReleasePromoteOptions{Timeout: options.Int(30)}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) appFromNamespace(ns ac.Namespace) (*structs.App, error) {
	name := common.CoalesceString(ns.Labels["app"], ns.Labels["name"])
	as, release := ns.Annotations["convox.com/app-status"], ns.Annotations["convox.com/app-release"]

	if as == "" || release == "" {
		var err error
		as, release, err = p.Atom.Status(ns.Name, "app")
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	status := common.AtomStatus(as)

	if ns.Annotations == nil {
		ns.Annotations = map[string]string{}
	}

	a := &structs.App{
		Generation: "3",
		Locked:     ns.Annotations["convox.com/lock"] == "true",
		Name:       name,
		Release:    release,
		Router:     p.Router,
		Status:     status,
	}

	var params map[string]string

	if data, ok := ns.Annotations["convox.com/params"]; ok && data > "" {
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if params == nil {
		params = map[string]string{}
	}

	defparams := p.Engine.AppParameters()

	// set parameter default values
	for k, v := range defparams {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}

	// filter out invalid parameters
	for k, v := range params {
		if _, ok := defparams[k]; !ok || (ok && v == "") {
			delete(params, k)
		}
	}

	a.Parameters = params

	switch ns.Status.Phase {
	case "Terminating":
		a.Status = "deleting"
	}

	return a, nil
}

func (p *Provider) appFromNamespaceOnly(ns ac.Namespace) (*structs.App, error) {
	name := common.CoalesceString(ns.Labels["app"], ns.Labels["name"])

	status := common.AtomStatus(ns.Annotations["convox.com/app-status"])

	if ns.Annotations == nil {
		ns.Annotations = map[string]string{}
	}

	a := &structs.App{
		Generation: "3",
		Locked:     ns.Annotations["convox.com/lock"] == "true",
		Name:       name,
		Release:    ns.Annotations["convox.com/app-release"],
		Router:     p.Router,
		Status:     status,
	}

	var params map[string]string

	if data, ok := ns.Annotations["convox.com/params"]; ok && data > "" {
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if params == nil {
		params = map[string]string{}
	}

	defparams := p.Engine.AppParameters()

	// set parameter default values
	for k, v := range defparams {
		if _, ok := params[k]; !ok && v != "" {
			params[k] = v
		}
	}

	// filter out invalid parameters
	for k := range params {
		if _, ok := defparams[k]; !ok {
			delete(params, k)
		}
	}

	a.Parameters = params

	switch ns.Status.Phase {
	case "Terminating":
		a.Status = "deleting"
	}

	return a, nil
}

func (p *Provider) appNameValidate(name string) error {
	switch name {
	case "rack", "system":
		return errors.WithStack(fmt.Errorf("app name is reserved"))
	}

	if _, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.AppNamespace(name), am.GetOptions{}); !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("app already exists: %s", name))
	}

	return nil
}

func (p *Provider) appParametersUpdate(a *structs.App, params map[string]string) error {
	defs := p.Engine.AppParameters()

	var invalidParameters []string
	for k, v := range params {
		if _, ok := defs[k]; !ok {
			invalidParameters = append(invalidParameters, k)
		} else {
			a.Parameters[k] = v
		}
	}

	fmt.Printf("Skipping unsupported parameters: %s ...", strings.Join(invalidParameters, ", "))

	return nil
}

func (p *Provider) appUpdate(a *structs.App) error {
	params, err := json.Marshal(a.Parameters)
	if err != nil {
		return errors.WithStack(err)
	}

	patches := []Patch{
		{Op: "add", Path: "/metadata/annotations/convox.com~1lock", Value: fmt.Sprintf("%t", a.Locked)},
		{Op: "add", Path: "/metadata/annotations/convox.com~1params", Value: string(params)},
	}

	patch, err := json.Marshal(patches)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := p.Cluster.CoreV1().Namespaces().Patch(
		context.TODO(), p.AppNamespace(a.Name), types.JSONPatchType, patch, am.PatchOptions{},
	); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) GenConfigName(id string) string {
	return fmt.Sprintf("cfg-%s", id)
}
