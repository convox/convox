package k8s

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/aws/provisioner"
	ktmpl "github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type goldenCase struct {
	label  string
	name   string
	params map[string]interface{}
}

func goldenCases(p *Provider, m *manifest.Manifest, a *structs.App) []goldenCase {
	r := &structs.Release{Id: "RABCDEF", Build: "BABC"}
	ns := "rack-" + a.Name
	cs := []goldenCase{
		{"app", "app/app", map[string]interface{}{
			"Locked": false, "Name": a.Name, "Namespace": ns, "Owner": &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack", UID: "uid1"}},
			"Parameters": map[string]string{"Foo": "a&b", "Bar": "x\"y"}, "PodSecurityStandard": "baseline", "PodSecurityMode": "enforce",
		}},
		{"ca", "app/ca", map[string]interface{}{"CA": "Y2E=", "Namespace": ns}},
	}

	for _, b := range m.Balancers {
		ports, tcpUdp := balancerRenderPorts(b.Ports)
		ans := b.AnnotationsMap()
		cs = append(cs, goldenCase{"balancer " + b.Name, "app/balancer", map[string]interface{}{
			"Annotations": ans, "Balancer": b, "HealthCheckPort": balancerHealthCheckPort(b.Ports), "Namespace": ns,
			"Ports": ports, "Release": r, "Labels": m.Labels, "Scheme": balancerScheme(ans), "TcpUdp": tcpUdp,
		}})
	}

	for i := range m.Services {
		s := m.Services[i]
		ans := map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"}
		for k, v := range s.IngressAnnotationsMap() {
			ans[k] = v
		}
		for _, name := range []string{"app/ingress", "app/ingress-internal"} {
			cs = append(cs, goldenCase{name + " " + s.Name, name, map[string]interface{}{
				"Annotations": ans, "App": a.Name, "Class": "nginx", "ConvoxDomainTLSCertDisable": false,
				"Host": p.ServiceHost(a.Name, s), "Idles": false, "Namespace": ns, "Rack": "rack", "Service": s,
			}})
		}

		sc := s
		sc.Certificate.Id = "my-cert"
		tr := translateNginxAnnotations(sc.IngressAnnotationsMap(), sc.Name)
		cs = append(cs,
			goldenCase{"httpproxy " + s.Name, "app/httpproxy", map[string]interface{}{
				"Annotations": tr.Annotations, "App": a.Name, "BackendProtocol": contourBackendProtocol(sc.Port.Scheme), "ConvoxDomainTLS": true,
				"Host": p.ServiceHost(a.Name, sc), "Idles": false, "IngressClassName": "contour", "Namespace": ns, "ProxyProtocol": true,
				"Rack": "rack", "RateLimitRPS": tr.RateLimitRPS, "Service": sc, "TimeoutResponse": responseTimeout(tr.TimeoutResponse),
				"TimeoutIdle": fmt.Sprintf("%ds", sc.Timeout), "WhitelistCIDRs": parseWhitelistCIDRs(sc.Whitelist),
			}},
			goldenCase{"certificate " + s.Name, "app/certificate", map[string]interface{}{
				"App": a.Name, "CertDuration": s.Certificate.Duration, "CertIssuer": "letsencrypt", "HasDomains": len(s.Domains) > 0,
				"Host": p.ServiceHost(a.Name, s), "Namespace": ns, "Service": s,
			}},
		)

		se := s
		se.VolumeOptions = append([]manifest.VolumeOption{}, s.VolumeOptions...)
		for i := range se.VolumeOptions {
			if se.VolumeOptions[i].AwsEfs != nil {
				e := *se.VolumeOptions[i].AwsEfs
				if e.VolumeHandle != "" {
					e.ProcessTemplate("fs-123", a.Name, s.Name)
				}
				se.VolumeOptions[i].AwsEfs = &e
			}
		}
		cs = append(cs,
			goldenCase{"efs " + s.Name, "app/efs", map[string]interface{}{"App": a, "Namespace": ns, "Rack": "rack", "Service": se}},
			goldenCase{"azurefiles " + s.Name, "app/azurefiles", map[string]interface{}{"App": a, "Namespace": ns, "Rack": "rack", "Service": &se}},
		)

		env := map[string]string{}
		for k, v := range m.Env() {
			env[k] = v
		}
		for k, v := range s.EnvironmentDefaults() {
			env[k] = v
		}
		cs = append(cs, goldenCase{"service " + s.Name, "app/service", map[string]interface{}{
			"Annotations": s.AnnotationsMap(), "App": a, "Environment": env, "MaxSurge": 50, "MaxUnavailable": 0,
			"ProgressDeadline": 600, "Namespace": ns, "Password": "pw", "Rack": "rack", "Release": r, "Replicas": 1,
			"Resources": s.ResourceMap(), "Service": s, "KedaIsEnabled": false, "AutoscaleIsEnabled": false,
			"TriggersOverrideActive": false, "DockerHubAuth": true, "ImagePullSecretNames": []string{"convox-x-y-pull-a"}, "RegistryAuth": true,
			"Resolver": "10.0.0.10",
		}})
	}

	for _, t := range m.Timers {
		s, err := m.Service(t.Service)
		if err != nil {
			continue
		}
		cs = append(cs, goldenCase{"timer " + t.Name, "app/timer", map[string]interface{}{
			"Annotations": t.AnnotationsMap(), "App": a, "Namespace": ns, "Rack": "rack", "Release": r, "Resources": s.ResourceMap(),
			"Service": s, "Timer": t, "DockerHubAuth": true, "ImagePullSecretNames": []string{"x"}, "RegistryAuth": false, "Resolver": "10.0.0.10",
		}})
	}

	conn := provisioner.ConnectionInfo{Host: "h.rds.amazonaws.com", Port: "5432", UserName: "app", Password: "p@ss&w:rd/x", Database: "app"} //nolint:gosec // template fixture exercising special characters
	for _, res := range m.Resources {
		if res.IsContainerizedResource() {
			img := res.Image
			if img == "" {
				v := res.Options["version"]
				if v == "" {
					v = resourceDefaultImages[res.Type].defaultVersion
				}
				img = resourceImage("", res.Type, v)
			}
			cs = append(cs, goldenCase{"resource " + res.Name, "resource/" + res.Type, map[string]interface{}{
				"App": a.Name, "Namespace": ns, "Name": res.Name, "Parameters": res.Options, "Password": "abc123", "Rack": "rack",
				"Image": img, "DockerHubAuth": true,
			}})
		}
		cs = append(cs, goldenCase{"masked " + res.Name, "resource/masked", map[string]interface{}{
			"App": a.Name, "Namespace": ns, "Name": res.Name, "Rack": "rack", "Url": "postgres://h/db?a=1&region=us&para=2&amp;x",
		}})
		for _, t := range []string{"rds-postgres", "rds-mysql", "rds-mariadb", "elasticache-redis", "elasticache-memcached"} {
			cs = append(cs, goldenCase{t + " " + res.Name, "resource/" + t, map[string]interface{}{
				"App": a.Name, "Namespace": ns, "Name": res.Name, "Parameters": res.Options, "Rack": "rack",
				"Host": conn.Host, "Port": conn.Port, "User": conn.UserName, "Password": conn.Password, "Database": conn.Database,
			}})
		}
	}

	return cs
}

func TestReleaseTemplatesGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/release-templates-edge.yml")
	require.NoError(t, err)

	m, err := manifest.Load(data, map[string]string{})
	require.NoError(t, err)

	p := &Provider{Engine: &mock.TestEngine{}, Name: "rack", Domain: "example.org", DomainInternal: "internal.example.org"}
	p.templater = templater.New(ktmpl.TemplatesFS)

	var out bytes.Buffer
	stream := [][]byte{}

	for _, c := range goldenCases(p, m, &structs.App{Name: "myapp"}) {
		fmt.Fprintf(&out, "# %s\n", c.label)
		d, err := p.RenderTemplate(c.name, c.params)
		require.NoError(t, err, c.label)
		out.Write(d)
		stream = append(stream, d)
	}

	stream = append(stream,
		[]byte("##|app:myapp|type:rds-postgres|resource:db|stateId:x|tid:|##"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: last\n"),
		[]byte("##|app:myapp|type:elasticache-redis|resource:c|stateId:y|tid:t1|##"),
	)
	labeled, err := ApplyLabels(bytes.Join(stream, []byte("---\n")), "system=convox,provider=k8s,rack=rack,app=myapp,release=RABCDEF")
	require.NoError(t, err)
	out.WriteString("# applylabels\n")
	out.Write(labeled)

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile("testdata/release-templates-edge.golden", out.Bytes(), 0600))
	}

	want, err := os.ReadFile("testdata/release-templates-edge.golden")
	require.NoError(t, err)
	require.Equal(t, strings.Split(string(want), "\n"), strings.Split(out.String(), "\n"))
}
