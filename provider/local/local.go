package local

import (
	"context"
	"fmt"
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

	// Ensure a self-signed CA secret exists before the parent initializes
	// cert-manager. The k8s provider's installCertManagerConfig goroutine
	// will find this secret and create the self-signed ClusterIssuer from it.
	if err := p.EnsureSelfSignedCA(); err != nil {
		fmt.Printf("warning: could not ensure self-signed CA: %s\n", err)
	}

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
