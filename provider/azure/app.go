package azure

import (
	"github.com/convox/convox/pkg/structs"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) AppGet(name string) (*structs.App, error) {
	a, err := p.Provider.AppGet(name)
	if err != nil {
		return nil, err
	}

	switch a.Parameters["Router"] {
	case "dedicated":
		ing, err := p.Cluster.ExtensionsV1beta1().Ingresses(p.AppNamespace(a.Name)).Get(a.Name, am.GetOptions{})
		if err != nil {
			return nil, err
		}

		if len(ing.Status.LoadBalancer.Ingress) > 0 {
			a.Router = ing.Status.LoadBalancer.Ingress[0].IP
		}
	}

	return a, nil
}

func (p *Provider) AppIdles(name string) (bool, error) {
	return false, nil
}

func (p *Provider) AppParameters() map[string]string {
	return map[string]string{
		"Router": "shared",
	}
}

func (p *Provider) AppStatus(name string) (string, error) {
	return "running", nil
}
