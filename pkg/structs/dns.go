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
	Id       int      `json:"id" param:"id" flag:"id"`
	DnsZones []string `json:"dns-zones" param:"dns-zones"`
	Route53  *Route53 `json:"route53" param:"route53"`
}

func (d *Dns01Solver) Validate() error {
	if len(d.DnsZones) == 0 {
		return fmt.Errorf("dns zones are required")
	}
	if d.Route53 != nil {
		if d.Route53.Role == nil || len(*d.Route53.Role) == 0 {
			return fmt.Errorf("route53 access role is required")
		}
	}
	return nil
}

type Route53 struct {
	HostedZoneID *string `json:"hosted-zone-id" param:"hosted-zone-id"`
	Region       *string `json:"region" param:"region"`
	Role         *string `json:"role" param:"role"`
}
