package gcp

import (
	"context"
	"encoding/base64"
	"os"

	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/storage"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s"
	"github.com/gobuffalo/packr"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type Provider struct {
	*k8s.Provider

	Bucket   string
	Key      []byte
	Project  string
	Registry string

	LogAdmin *logadmin.Client
	Storage  *storage.Client

	templater *templater.Templater
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider: k,
		Bucket:   os.Getenv("BUCKET"),
		Project:  os.Getenv("PROJECT"),
		Registry: os.Getenv("REGISTRY"),
	}

	key, err := base64.StdEncoding.DecodeString(os.Getenv("KEY"))
	if err != nil {
		return nil, err
	}

	p.Key = key

	p.templater = templater.New(packr.NewBox("../aws/template"), p.templateHelpers())

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeGcpServices(); err != nil {
		return err
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	runtime.ErrorHandlers = []func(error){}

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}

func (p *Provider) initializeGcpServices() error {
	ctx := context.Background()

	la, err := logadmin.NewClient(ctx, p.Project)
	if err != nil {
		return err
	}

	p.LogAdmin = la

	s, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	p.Storage = s

	return nil
}
