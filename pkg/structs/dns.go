package structs

import (
	"fmt"
	"os"

	"github.com/convox/convox/pkg/options"
)

type LetsEncryptConfig struct {
	Role    string         `json:"role"`
	Solvers []*Dns01Solver `json:"solvers" param:"solvers"`
}

func (l *LetsEncryptConfig) Defaults() {
	l.Role = os.Getenv("CERT_MANAGER_ROLE_ARN")
	for i := range l.Solvers {
		if l.Solvers[i].Route53 != nil && (l.Solvers[i].Route53.Region == nil || *l.Solvers[i].Route53.Region == "") {
			l.Solvers[i].Route53.Region = options.String(os.Getenv("AWS_REGION"))
		}
	}
}

type Dns01Solver struct {
	Id         int         `json:"id" param:"id" flag:"id"`
	DnsZones   []string    `json:"dns-zones" param:"dns-zones"`
	Route53    *Route53    `json:"route53" param:"route53"`
	Cloudflare *Cloudflare `json:"cloudflare" param:"cloudflare"`
}

func (d *Dns01Solver) Validate() error {
	if len(d.DnsZones) == 0 {
		return fmt.Errorf("dns zones are required")
	}

	switch {
	case d.Route53 != nil:
		if err := d.Route53.Validate(); err != nil {
			return err
		}
	case d.Cloudflare != nil:
		if err := d.Cloudflare.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("route53 or cloudflare configuration is required")
	}

	return nil
}

type Route53 struct {
	HostedZoneID *string `json:"hosted-zone-id" param:"hosted-zone-id"`
	Region       *string `json:"region" param:"region"`
	Role         *string `json:"role" param:"role"`
}

func (r *Route53) Validate() error {
	if r.Role == nil || len(*r.Role) == 0 {
		return fmt.Errorf("route53 access role is required")
	}
	return nil
}

type Cloudflare struct {
	ApiTokenSecretRefName *string `json:"api-token-secret-ref-name" param:"api-token-secret-ref-name"`
	ApiTokenSecretRefKey  *string `json:"api-token-secret-ref-key" param:"api-token-secret-ref-key"`
	ApiTokenValue         *string `json:"api-token,omitempty" param:"api-token"`
	ApiKeySecretRefName   *string `json:"api-key-secret-ref-name" param:"api-key-secret-ref-name"`
	ApiKeySecretRefKey    *string `json:"api-key-secret-ref-key" param:"api-key-secret-ref-key"`
	ApiKeyValue           *string `json:"api-key,omitempty" param:"api-key"`
	Email                 *string `json:"email" param:"email"`
}

func (c *Cloudflare) Validate() error {
	hasTokenRef := c.ApiTokenSecretRefName != nil && *c.ApiTokenSecretRefName != "" && c.ApiTokenSecretRefKey != nil && *c.ApiTokenSecretRefKey != ""
	hasTokenValue := c.ApiTokenValue != nil && *c.ApiTokenValue != ""
	hasKeyRef := c.ApiKeySecretRefName != nil && *c.ApiKeySecretRefName != "" && c.ApiKeySecretRefKey != nil && *c.ApiKeySecretRefKey != ""
	hasKeyValue := c.ApiKeyValue != nil && *c.ApiKeyValue != ""

	if (hasTokenRef || hasTokenValue) && (hasKeyRef || hasKeyValue) {
		return fmt.Errorf("cloudflare api token and api key secret references are mutually exclusive")
	}

	if !(hasTokenRef || hasTokenValue || hasKeyRef || hasKeyValue) {
		return fmt.Errorf("cloudflare api token or api key secret reference is required")
	}

	if (hasKeyRef || hasKeyValue) && (c.Email == nil || *c.Email == "") {
		return fmt.Errorf("cloudflare email is required when using api key secret")
	}

	return nil
}
