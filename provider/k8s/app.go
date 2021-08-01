package k8s

import (
	"context"
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

	if _, err := p.Cluster.CoreV1().Namespaces().Create(context.Background(), ns, am.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	a := &structs.App{
		Name:       name,
		Parameters: p.Engine.AppParameters(),
	}

	if err := p.appUpdate(a); err != nil {
		return nil, errors.WithStack(err)
	}

	if err := p.ReleasePromote(a.Name, "", structs.ReleasePromoteOptions{Timeout: options.Int(30)}); err != nil {
		return nil, errors.WithStack(err)
	}

	a, err := p.AppGet(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.EventSend("app:create", structs.EventSendOptions{Data: map[string]string{"name": name}})

	return a, nil
}

func (p *Provider) AppDelete(name string) error {
	a, err := p.AppGet(name)
	if err != nil {
		return errors.WithStack(err)
	}

	if a.Locked {
		return errors.WithStack(fmt.Errorf("app is locked: %s", name))
	}

	if err := p.Cluster.CoreV1().Namespaces().Delete(context.Background(), p.AppNamespace(name), am.DeleteOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) AppGet(name string) (*structs.App, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.Background(), p.AppNamespace(name), am.GetOptions{})
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
	ns, err := p.Cluster.CoreV1().Namespaces().List(context.Background(), am.ListOptions{
		LabelSelector: fmt.Sprintf("system=convox,rack=%s,type=app", p.Name),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	as := structs.Apps{}

	for _, n := range ns.Items {
		a, err := p.appFromNamespace(n)
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
		return fmt.Sprintf("%s-%s", p.Name, app)
	}
}

func (p *Provider) NamespaceApp(namespace string) (string, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.Background(), namespace, am.GetOptions{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if app, ok := ns.ObjectMeta.Labels["app"]; ok && strings.TrimSpace(app) != "" {
		return app, nil
	}

	return "", errors.WithStack(fmt.Errorf("could not determine app for namespace: %s", namespace))
}

func (p *Provider) AppParameters() map[string]string {
	return map[string]string{}
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

	as, release, err := p.Atom.Status(ns.Name, "app")
	if err != nil {
		return nil, errors.WithStack(err)
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

	if _, err := p.Cluster.CoreV1().Namespaces().Get(context.Background(), p.AppNamespace(name), am.GetOptions{}); !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("app already exists: %s", name))
	}

	return nil
}

func (p *Provider) appParametersUpdate(a *structs.App, params map[string]string) error {
	defs := p.Engine.AppParameters()

	var redundantParameters []string
	for k, v := range params {
		if _, ok := defs[k]; !ok {
			redundantParameters = append(redundantParameters, k)
		} else {
			a.Parameters[k] = v
		}
	}

	fmt.Printf("Skipping redundant parameters: %s ...", strings.Join(redundantParameters, ", "))

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

	if _, err := p.Cluster.CoreV1().Namespaces().Patch(context.Background(), p.AppNamespace(a.Name), types.JSONPatchType, patch, am.PatchOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
