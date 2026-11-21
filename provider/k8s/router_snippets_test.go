package k8s

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	convoxRouterImage = "convox/ingress-nginx:1.15.1-1@sha256:40b907401d849828ea5645b258b68dc4afe1b8f329130a0043541c7fe1bb395b"
	snippetRefusal    = "is not allowed while router_user_snippets_disable=true"
	subsFilterRefusal = "uses subs_filter, which this rack's router does not support"
)

func snippetService(name string, ans ...string) manifest.Service {
	s := manifest.Service{Name: name, Port: manifest.ServicePortScheme{Port: 3000}}
	for i := 0; i+1 < len(ans); i += 2 {
		s.IngressAnnotations = append(s.IngressAnnotations, manifest.Annotation{Key: ans[i], Value: ans[i+1]})
	}
	return s
}

func routerDeployment(container, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{Namespace: "ns1", Name: "ingress-nginx"},
		Spec: appsv1.DeploymentSpec{Template: ac.PodTemplateSpec{Spec: ac.PodSpec{
			Containers: []ac.Container{{Name: container, Image: image}},
		}}},
	}
}

func snippetProvider(t *testing.T, objects ...*appsv1.Deployment) (*Provider, *fake.Clientset) {
	t.Helper()
	p, kk, _ := minimalProvider(t)
	p.RouterType = "nginx"
	for _, d := range objects {
		_, err := kk.AppsV1().Deployments(d.Namespace).Create(context.TODO(), d, am.CreateOptions{})
		require.NoError(t, err)
	}
	return p, kk
}

func TestValidateRouterSnippets_ParameterOn(t *testing.T) {
	internalRouter := snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "add_header X 1;")
	internalRouter.InternalRouter = true

	internal := snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "add_header X 1;")
	internal.Internal = true

	noPort := snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "add_header X 1;")
	noPort.Port = manifest.ServicePortScheme{}

	tests := []struct {
		name     string
		services manifest.Services
		want     string
	}{
		{"configuration snippet", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "add_header X 1;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + snippetRefusal},
		{"server snippet dropped at render", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/server-snippet", "return 200;")}, ""},
		{"server snippet trailing space", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/server-snippet ", "return 200;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/server-snippet " + snippetRefusal},
		{"server snippet trailing tab", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/server-snippet\t", "return 200;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/server-snippet " + snippetRefusal},
		{"server snippet trailing carriage return", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/server-snippet\r", "return 200;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/server-snippet " + snippetRefusal},
		{"case variant", manifest.Services{snippetService("web", "Nginx.Ingress.Kubernetes.io/Configuration-Snippet", "add_header X 1;")},
			"service web: ingress annotation Nginx.Ingress.Kubernetes.io/Configuration-Snippet " + snippetRefusal},
		{"other prefix", manifest.Services{snippetService("web", "example.com/x-snippet", "add_header X 1;")}, ""},
		{"value mentions snippet", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/proxy-body-size", "configuration-snippet")}, ""},
		{"internal router", manifest.Services{internalRouter},
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + snippetRefusal},
		{"internal service has no ingress", manifest.Services{internal}, ""},
		{"no port", manifest.Services{noPort}, ""},
		{"sorted", manifest.Services{
			snippetService("web", "nginx.ingress.kubernetes.io/server-snippet ", "x;", "nginx.ingress.kubernetes.io/auth-snippet", "x;"),
			snippetService("api", "nginx.ingress.kubernetes.io/configuration-snippet", "x;"),
		}, "service api: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + snippetRefusal + "\n" +
			"service web: ingress annotation nginx.ingress.kubernetes.io/auth-snippet " + snippetRefusal + "\n" +
			"service web: ingress annotation nginx.ingress.kubernetes.io/server-snippet " + snippetRefusal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := snippetProvider(t)
			p.RouterUserSnippetsDisable = true

			err := p.validateRouterSnippets(&manifest.Manifest{Services: tt.services})
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.want, err.Error())

			var he *structs.HttpError
			require.ErrorAs(t, err, &he)
			require.Equal(t, 400, he.Code())
		})
	}
}

func TestValidateRouterSnippets_ParameterOnAnyRouterType(t *testing.T) {
	p, _ := snippetProvider(t)
	p.RouterUserSnippetsDisable = true
	p.RouterType = "contour"

	err := p.validateRouterSnippets(&manifest.Manifest{Services: manifest.Services{
		snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "add_header X 1;"),
	}})
	require.ErrorContains(t, err, snippetRefusal)
}

func TestValidateRouterSnippets_SubsFilter(t *testing.T) {
	internalRouter := snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs_filter a b;")
	internalRouter.InternalRouter = true

	tests := []struct {
		name       string
		routerType string
		services   manifest.Services
		want       string
	}{
		{"configuration snippet", "nginx", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs_filter a b;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + subsFilterRefusal},
		{"internal router", "nginx", manifest.Services{internalRouter},
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + subsFilterRefusal},
		{"server snippet dropped at render", "nginx", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/server-snippet", "subs_filter a b;")}, ""},
		{"other prefix", "nginx", manifest.Services{snippetService("web", "example.com/x-snippet", "subs_filter a b;")}, ""},
		{"valid snippet", "nginx", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "more_set_headers \"X-A: 1\";")}, ""},
		{"entity escaped directive", "nginx", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs&#95;filter a b;")},
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + subsFilterRefusal},
		{"entity escaped comment", "nginx", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "&#35; subs_filter a b;")}, ""},
		{"contour", "contour", manifest.Services{snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs_filter a b;")}, ""},
		{"sorted", "nginx", manifest.Services{
			snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs_filter a b;", "nginx.ingress.kubernetes.io/auth-snippet", "add_header X 1;"),
			snippetService("api", "nginx.ingress.kubernetes.io/location-snippet", "subs_filter_types text/html;"),
		}, "service api: ingress annotation nginx.ingress.kubernetes.io/location-snippet " + subsFilterRefusal + "\n" +
			"service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet " + subsFilterRefusal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := snippetProvider(t, routerDeployment("system", convoxRouterImage))
			p.RouterType = tt.routerType

			err := p.validateRouterSnippets(&manifest.Manifest{Services: tt.services})
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.want, err.Error())
		})
	}
}

func TestValidateRouterSnippets_SubsFilterRouterImage(t *testing.T) {
	m := &manifest.Manifest{Services: manifest.Services{
		snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "subs_filter a b;"),
	}}

	tests := []struct {
		name    string
		router  []*appsv1.Deployment
		refused bool
	}{
		{"convox image", []*appsv1.Deployment{routerDeployment("system", convoxRouterImage)}, true},
		{"upstream 1.12", []*appsv1.Deployment{routerDeployment("system", "registry.k8s.io/ingress-nginx/controller:v1.12.6@sha256:abc")}, false},
		{"upstream 1.13", []*appsv1.Deployment{routerDeployment("system", "registry.k8s.io/ingress-nginx/controller:v1.13.0")}, true},
		{"unknown image", []*appsv1.Deployment{routerDeployment("system", "example.com/custom/nginx:1")}, false},
		{"no router deployment", nil, false},
		{"no system container", []*appsv1.Deployment{routerDeployment("controller", convoxRouterImage)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := snippetProvider(t, tt.router...)

			err := p.validateRouterSnippets(m)
			if tt.refused {
				require.ErrorContains(t, err, subsFilterRefusal)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRouterSnippets_NoRouterReadWithoutHit(t *testing.T) {
	p, kk := snippetProvider(t, routerDeployment("system", convoxRouterImage))
	kk.ClearActions()

	err := p.validateRouterSnippets(&manifest.Manifest{Services: manifest.Services{
		snippetService("web", "nginx.ingress.kubernetes.io/configuration-snippet", "more_set_headers \"X-A: 1\";"),
	}})
	require.NoError(t, err)
	require.Empty(t, kk.Actions())
}

func TestRouterRejectedDirective(t *testing.T) {
	tests := []struct {
		name    string
		snippet string
		want    string
	}{
		{"start", "subs_filter 'http://old' 'https://new' g;", "subs_filter"},
		{"after semicolon", "more_set_headers \"X-A: 1\";subs_filter a b;", "subs_filter"},
		{"after semicolon and space", "add_header X 1; subs_filter a b;", "subs_filter"},
		{"after newline", "add_header X 1\nsubs_filter a b;", "subs_filter"},
		{"inside location block", "location /x { subs_filter a b; }", "subs_filter"},
		{"after closing brace", "location /x { add_header X 1; }subs_filter a b;", "subs_filter"},
		{"types directive", "subs_filter_types text/html;", "subs_filter"},
		{"leading whitespace", " \t\r\n  subs_filter a b;", "subs_filter"},
		{"after comment line", "# replace hosts\nsubs_filter a b;", "subs_filter"},
		{"after quoted argument", "add_header X \"a;b\"; subs_filter a b;", "subs_filter"},
		{"escaped quote keeps string open", "add_header X \"a\\\"; subs_filter b\";", ""},
		{"escaped semicolon in a word", "add_header X a\\;subs_filter b;", ""},
		{"apostrophe inside a word", "location ~ ^/o'reilly { subs_filter a b; }", "subs_filter"},
		{"hash inside a word", "add_header X a#b; subs_filter a b;", "subs_filter"},
		{"argument", "add_header X subs_filter;", ""},
		{"double quoted", "add_header X \"; subs_filter a b\";", ""},
		{"single quoted", "add_header X '; subs_filter a b';", ""},
		{"comment", "add_header X 1; # subs_filter a b;", ""},
		{"comment at start", "#subs_filter a b;", ""},
		{"unterminated quote", "add_header X \"; subs_filter a b;", ""},
		{"standard sub module", "sub_filter a b;", ""},
		{"valid snippet", "more_set_headers \"X-Frame-Options: DENY\";\nproxy_set_header X-A 1;", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, routerRejectedDirective(tt.snippet))
		})
	}
}

func TestRouterImageLacksSubsFilter(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"registry.k8s.io/ingress-nginx/controller:v1.12.6@sha256:abc", false},
		{"registry.k8s.io/ingress-nginx/controller:v1.12.0", false},
		{"registry.k8s.io/ingress-nginx/controller:v1.13.0", true},
		{"registry.k8s.io/ingress-nginx/controller:v1.15.1@sha256:abc", true},
		{"123456789012.dkr.ecr.us-east-1.amazonaws.com/ingress-nginx/controller:v1.12.6", false},
		{"ingress-nginx/controller:v2.0.0", true},
		{convoxRouterImage, true},
		{"convox/ingress-nginx:7a97210a0d5528486d013ec58eada915f62facab@sha256:4edc14733b5901ccc80d3b9722eee9ddd7de1b6ce7ce434adcaf08980c427354", true},
		{"convox/ingress-nginx@sha256:abc", true},
		{"convox/ingress-nginx", true},
		{"registry.example.com/mirror/convox/ingress-nginx:1.15.1-2", true},
		{"myconvox/ingress-nginx:1.15.1-1", false},
		{"nginx:1.27", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			require.Equal(t, tt.want, routerImageLacksSubsFilter(tt.image))
		})
	}
}

func TestDefaultRouterImageLacksSubsFilter(t *testing.T) {
	data, err := os.ReadFile("../../terraform/router/nginx/main.tf")
	require.NoError(t, err)

	m := regexp.MustCompile(`default_nginx_image\s*=\s*"([^"]+)"`).FindSubmatch(data)
	require.NotNil(t, m)
	require.True(t, routerImageLacksSubsFilter(string(m[1])), "default router image %s", m[1])
}

const snippetPromoteManifest = `services:
  web:
    build: .
    port: 3000
    ingressAnnotations:
      - nginx.ingress.kubernetes.io/configuration-snippet=%s
`

func snippetPromoteProvider(t *testing.T, userSnippetsDisable bool, snippet string) (*Provider, *atom.MockInterface) {
	t.Helper()
	t.Setenv("TEST", "true")

	p, kk, kc := minimalProvider(t)
	p.RouterType = "nginx"
	p.RouterUserSnippetsDisable = userSnippetsDisable

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{Name: p.Namespace},
	}, am.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, p.Initialize(structs.ProviderOptions{}))

	_, err = kk.AppsV1().Deployments(p.Namespace).Create(context.TODO(), routerDeployment("system", convoxRouterImage), am.CreateOptions{})
	require.NoError(t, err)

	createAppNamespace(t, kk, "rack1", "app1")
	t.Cleanup(SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond))
	setAppStatus(t, kk, "rack1-app1", "Running")
	setAppRelease(t, kk, "rack1-app1", "rel1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", fmt.Sprintf(snippetPromoteManifest, snippet))
	createRelease(t, kc, "rack1-app1", "rel2", fmt.Sprintf(snippetPromoteManifest, snippet))

	aa, ok := p.Atom.(*atom.MockInterface)
	require.True(t, ok)
	aa.On("Apply", tmock.Anything, tmock.Anything, tmock.Anything).Return(nil).Maybe()

	return p, aa
}

func TestReleasePromote_RouterSnippets(t *testing.T) {
	tests := []struct {
		name                string
		userSnippetsDisable bool
		snippet             string
		release             string
		opts                structs.ReleasePromoteOptions
		refusal             string
	}{
		{"parameter off", false, "add_header X-QA 1;", "rel2", structs.ReleasePromoteOptions{}, ""},
		{"parameter on", true, "add_header X-QA 1;", "rel2", structs.ReleasePromoteOptions{}, snippetRefusal},
		{"parameter on, force", true, "add_header X-QA 1;", "rel2", structs.ReleasePromoteOptions{Force: options.Bool(true)}, snippetRefusal},
		{"parameter on, current release", true, "add_header X-QA 1;", "rel1", structs.ReleasePromoteOptions{}, ""},
		{"subs_filter", false, "subs_filter a b;", "rel2", structs.ReleasePromoteOptions{}, subsFilterRefusal},
		{"subs_filter, force", false, "subs_filter a b;", "rel2", structs.ReleasePromoteOptions{Force: options.Bool(true)}, subsFilterRefusal},
		{"subs_filter, current release", false, "subs_filter a b;", "rel1", structs.ReleasePromoteOptions{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, aa := snippetPromoteProvider(t, tt.userSnippetsDisable, tt.snippet)

			err := p.ReleasePromote("app1", tt.release, tt.opts)
			if tt.refusal == "" {
				require.NoError(t, err)
				aa.AssertCalled(t, "Apply", tmock.Anything, tmock.Anything, tmock.Anything)
				return
			}
			require.ErrorContains(t, err, "service web: ingress annotation nginx.ingress.kubernetes.io/configuration-snippet "+tt.refusal)
			aa.AssertNotCalled(t, "Apply", tmock.Anything, tmock.Anything, tmock.Anything)
		})
	}
}
