package k8s

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/gobuffalo/packr"
)

func TestRenderTemplate(t *testing.T) {
	p := Provider{}
	p.templater = templater.New(packr.NewBox("../k8s/template"), p.templateHelpers())

	data, err := p.RenderTemplate(fmt.Sprintf("system/%s", "cert-manager-letsencrypt"), map[string]interface{}{
		"Config": structs.LetsEncryptConfig{
			Solvers: []*structs.Dns01Solver{
				{
					Id:       1,
					DnsZones: []string{"test.com"},
					Route53: &structs.Route53{
						HostedZoneID: options.String("host"),
						Region:       options.String("us"),
						Role:         options.String("role"),
					},
				},
				{
					Id:       1,
					DnsZones: []string{"test.com"},
					Route53: &structs.Route53{
						HostedZoneID: options.String("host"),
						Region:       options.String("us"),
						Role:         options.String("role"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(data))
}
