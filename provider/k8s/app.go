package k8s

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) AppCancel(name string) error {
	if _, err := p.AppGet(name); err != nil {
		return err
	}

	if err := p.Atom.Cancel(p.AppNamespace(name), "app"); err != nil {
		return err
	}

	return nil
}

func (p *Provider) AppCreate(name string, opts structs.AppCreateOptions) (*structs.App, error) {
	if err := p.appNameValidate(name); err != nil {
		return nil, err
	}

	a := &structs.App{
		Name:       name,
		Parameters: p.Engine.AppParameters(),
	}

	if err := p.appApply(a); err != nil {
		return nil, err
	}

	a, err := p.AppGet(name)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (p *Provider) AppDelete(name string) error {
	if _, err := p.AppGet(name); err != nil {
		return err
	}

	if err := p.Cluster.CoreV1().Namespaces().Delete(p.AppNamespace(name), nil); err != nil {
		return err
	}

	return nil
}

func (p *Provider) AppGet(name string) (*structs.App, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(p.AppNamespace(name), am.GetOptions{})
	if ae.IsNotFound(err) {
		return nil, fmt.Errorf("app not found: %s", name)
	}
	if err != nil {
		return nil, err
	}

	a, err := p.appFromNamespace(*ns)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (p *Provider) AppList() (structs.Apps, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().List(am.ListOptions{
		LabelSelector: fmt.Sprintf("system=convox,rack=%s,type=app", p.Name),
	})
	if err != nil {
		return nil, err
	}

	as := structs.Apps{}

	for _, n := range ns.Items {
		a, err := p.appFromNamespace(n)
		if err != nil {
			return nil, err
		}

		as = append(as, *a)
	}

	return as, nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) AppMetrics(name string, opts structs.MetricsOptions) (structs.Metrics, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) AppNamespace(app string) string {
	switch app {
	case "system":
		return p.Namespace
	default:
		return fmt.Sprintf("%s-%s", p.Name, app)
	}
}

func (p *Provider) AppUpdate(name string, opts structs.AppUpdateOptions) error {
	a, err := p.AppGet(name)
	if err != nil {
		return err
	}

	if opts.Lock != nil {
		a.Locked = *opts.Lock
	}

	dps := p.Engine.AppParameters()

	if opts.Parameters != nil {
		for k, v := range opts.Parameters {
			if _, ok := dps[k]; !ok {
				return fmt.Errorf("invalid parameter: %s", k)
			}

			a.Parameters[k] = v
		}
	}

	if err := p.appApply(a); err != nil {
		return err
	}

	if a.Release != "" {
		if err := p.ReleasePromote(a.Name, a.Release, structs.ReleasePromoteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (p *Provider) appApply(a *structs.App) error {
	params := map[string]interface{}{
		"Name":      a.Name,
		"Namespace": p.AppNamespace(a.Name),
		"Params":    a.Parameters,
		"Rack":      p.Name,
	}

	data, err := p.RenderTemplate("app/app", params)
	if err != nil {
		return err
	}

	if err := p.ApplyWait(p.AppNamespace(a.Name), "app", "", data, fmt.Sprintf("system=convox,provider=k8s,rack=%s,app=%s", p.Name, a.Name), 30); err != nil {
		return err
	}

	return nil
}

func (p *Provider) appFromNamespace(ns ac.Namespace) (*structs.App, error) {
	name := common.CoalesceString(ns.Labels["app"], ns.Labels["name"])

	as, release, err := p.Atom.Status(ns.Name, "app")
	if err != nil {
		return nil, err
	}

	status := common.AtomStatus(as)

	a := &structs.App{
		Generation: "2",
		Locked:     ns.Annotations["convox.com/lock"] == "true",
		Name:       name,
		Release:    release,
		Router:     p.Router,
		Status:     status,
	}

	var params map[string]string

	if data, ok := ns.Annotations["convox.com/params"]; ok && data > "" {
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return nil, err
		}
	}

	if params == nil {
		params = map[string]string{}
	}

	defparams := p.Engine.AppParameters()

	for k, v := range defparams {
		if _, ok := params[k]; !ok {
			params[k] = v
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
		return fmt.Errorf("app name is reserved")
	}

	if _, err := p.Cluster.CoreV1().Namespaces().Get(p.AppNamespace(name), am.GetOptions{}); !ae.IsNotFound(err) {
		return fmt.Errorf("app already exists: %s", name)
	}

	return nil
}
