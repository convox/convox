package metal

import (
	"context"
	"os"

	"github.com/convox/convox/pkg/loki"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
)

type Provider struct {
	*k8s.Provider

	Registry       string
	Secret         string
	SpacesAccess   string
	SpacesEndpoint string
	SpacesSecret   string

	loki *loki.Client
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
	if err := p.initializeMetalServices(); err != nil {
		return err
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}

func (p *Provider) initializeMetalServices() error {
	lc, err := loki.New(os.Getenv("LOKI_URL"))
	if err != nil {
		return err
	}

	p.loki = lc

	return nil
}
