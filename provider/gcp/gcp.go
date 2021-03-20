package gcp

import (
	"context"
	"encoding/base64"
	"os"

	"cloud.google.com/go/storage"
	"github.com/convox/convox/pkg/elastic"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Provider struct {
	*k8s.Provider

	Bucket   string
	Key      []byte
	Project  string
	Region   string
	Registry string

	elastic *elastic.Client
	storage *storage.Client
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
		Region:   os.Getenv("REGION"),
		Registry: os.Getenv("REGISTRY"),
	}

	key, err := base64.StdEncoding.DecodeString(os.Getenv("KEY"))
	if err != nil {
		return nil, err
	}

	p.Key = key

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeGcpServices(); err != nil {
		return err
	}

	if err := p.initializeResourceQuotas(); err != nil {
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

func (p *Provider) initializeGcpServices() error {
	ec, err := elastic.New(os.Getenv("ELASTIC_URL"))
	if err != nil {
		return err
	}

	p.elastic = ec

	ctx := context.Background()

	s, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	p.storage = s

	return nil
}

func (p *Provider) initializeResourceQuotas() error {
	_, err := p.Cluster.CoreV1().ResourceQuotas(p.Namespace).Create(&ac.ResourceQuota{
		ObjectMeta: am.ObjectMeta{
			Namespace: p.Namespace,
			Name:      "gcp-critical-pods",
		},
		Spec: ac.ResourceQuotaSpec{
			Hard: ac.ResourceList{
				"pods": resource.MustParse("1G"),
			},
			ScopeSelector: &ac.ScopeSelector{
				MatchExpressions: []ac.ScopedResourceSelectorRequirement{
					{
						ScopeName: "PriorityClass",
						Operator:  "In",
						Values: []string{
							"system-node-critical",
							"system-cluster-critical",
						},
					},
				},
			},
		},
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
