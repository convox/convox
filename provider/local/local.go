package local

import (
	"context"
	"os"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
)

type Provider struct {
	*k8s.Provider

	Registry string
	Secret   string
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider: k,
		Registry: os.Getenv("REGISTRY"),
		Secret:   os.Getenv("SECRET"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	opts.IgnorePriorityClass = true
	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	pc, err := NewPodController(p)
	if err != nil {
		return err
	}

	go pc.Run()

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}
