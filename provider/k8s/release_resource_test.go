package k8s

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testEcrPrefix = "123456789012.dkr.ecr.us-east-1.amazonaws.com/docker-hub"

func TestResourceVersionFromImage(t *testing.T) {
	tests := []struct {
		resourceType string
		image        string
		want         string
	}{
		{"postgres", "postgres:10.5", "10.5"},
		{"postgres", testEcrPrefix + "/library/postgres:10.5", "10.5"},
		{"postgis", testEcrPrefix + "/postgis/postgis:10-3.2", "10-3.2"},
		{"postgis", "postgis/postgis:10-3.2", "10-3.2"},
		{"mysql", testEcrPrefix + "/library/mysql:5.7.23", "5.7.23"},
		{"postgres", "docker.io/library/postgres:12", "12"},
		{"postgres", "myorg/postgres:15", "15"},
		{"postgres", "postgres:10.5@sha256:0123abcd", "10.5"},
		{"postgres", "mysql:5.7.23", ""},
		{"postgis", "postgres:10.5", ""},
		{"postgres", "postgis/postgis:10-3.2", ""},
		{"postgres", "myorg/postgres-custom:1", ""},
		{"postgres", "postgres@sha256:0123abcd", ""},
		{"postgres", "postgres", ""},
		{"postgres", "postgres:", ""},
		{"postgres", "registry:5000/postgres", ""},
		{"postgres", "postgres:5000/library/postgres", ""},
		{"unknown", "unknown:1", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.resourceType, tt.image), func(t *testing.T) {
			require.Equal(t, tt.want, resourceVersionFromImage(tt.resourceType, tt.image))
		})
	}
}

func resourceRenderProvider(prefix string, objs ...runtime.Object) (*Provider, *fake.Clientset) {
	kk := fake.NewSimpleClientset(objs...)
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1", EcrDockerHubCachePrefix: prefix}
	p.templater = templater.New(template.TemplatesFS)
	return p, kk
}

func resourceDeployment(name, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{Namespace: "rack1-app1", Name: "resource-" + name},
		Spec: appsv1.DeploymentSpec{
			Template: ac.PodTemplateSpec{
				Spec: ac.PodSpec{Containers: []ac.Container{{Name: "main", Image: image}}},
			},
		},
	}
}

func resourcePVC(name string) *ac.PersistentVolumeClaim {
	return &ac.PersistentVolumeClaim{ObjectMeta: am.ObjectMeta{Namespace: "rack1-app1", Name: "resource-" + name}}
}

func renderResource(t *testing.T, p *Provider, r manifest.Resource) (string, error) {
	t.Helper()

	data, err := p.releaseTemplateResource(&structs.App{Name: "app1"}, structs.Environment{}, r)
	return string(data), err
}

func TestReleaseTemplateResourceImage(t *testing.T) {
	defaults := map[string]string{
		"redis":     "redis:8",
		"postgres":  "postgres:16-trixie",
		"postgis":   "postgis/postgis:16-3.6",
		"mysql":     "mysql:8.4",
		"mariadb":   "mariadb:11.4",
		"memcached": "memcached:1.6",
	}

	for typ, image := range defaults {
		t.Run("new "+typ, func(t *testing.T) {
			p, _ := resourceRenderProvider("")
			out, err := renderResource(t, p, manifest.Resource{Name: "db", Type: typ})
			require.NoError(t, err)
			require.Contains(t, out, "image: "+image+"\n")
		})

		t.Run("new "+typ+" with prefix", func(t *testing.T) {
			p, _ := resourceRenderProvider(testEcrPrefix)
			out, err := renderResource(t, p, manifest.Resource{Name: "db", Type: typ})
			require.NoError(t, err)
			require.Contains(t, out, "image: "+resourceImage(testEcrPrefix, typ, resourceDefaultImages[typ].defaultVersion)+"\n")
		})
	}

	tests := []struct {
		name     string
		prefix   string
		resource manifest.Resource
		objs     []runtime.Object
		want     string
	}{
		{
			name:     "existing postgres keeps its version",
			resource: manifest.Resource{Name: "db", Type: "postgres"},
			objs:     []runtime.Object{resourceDeployment("db", "postgres:10.5"), resourcePVC("db")},
			want:     "postgres:10.5",
		},
		{
			name:     "existing prefixed mysql renders the identical string",
			prefix:   testEcrPrefix,
			resource: manifest.Resource{Name: "db", Type: "mysql"},
			objs:     []runtime.Object{resourceDeployment("db", testEcrPrefix+"/library/mysql:5.7.23"), resourcePVC("db")},
			want:     testEcrPrefix + "/library/mysql:5.7.23",
		},
		{
			name:     "existing prefixed postgis renders the identical string",
			prefix:   testEcrPrefix,
			resource: manifest.Resource{Name: "db", Type: "postgis"},
			objs:     []runtime.Object{resourceDeployment("db", testEcrPrefix+"/postgis/postgis:10-3.2"), resourcePVC("db")},
			want:     testEcrPrefix + "/postgis/postgis:10-3.2",
		},
		{
			name:     "existing prefixed postgis on a rack without prefix",
			resource: manifest.Resource{Name: "db", Type: "postgis"},
			objs:     []runtime.Object{resourceDeployment("db", testEcrPrefix+"/postgis/postgis:10-3.2"), resourcePVC("db")},
			want:     "postgis/postgis:10-3.2",
		},
		{
			name:     "existing mysql on a rack with prefix",
			prefix:   testEcrPrefix,
			resource: manifest.Resource{Name: "db", Type: "mysql"},
			objs:     []runtime.Object{resourceDeployment("db", "mysql:5.7.23"), resourcePVC("db")},
			want:     testEcrPrefix + "/library/mysql:5.7.23",
		},
		{
			name:     "version option wins over the running image",
			resource: manifest.Resource{Name: "db", Type: "postgres", Options: map[string]string{"version": "14"}},
			objs:     []runtime.Object{resourceDeployment("db", "postgres:10.5"), resourcePVC("db")},
			want:     "postgres:14",
		},
		{
			name:     "existing redis keeps its version",
			resource: manifest.Resource{Name: "cache", Type: "redis"},
			objs:     []runtime.Object{resourceDeployment("cache", "redis:4.0.10")},
			want:     "redis:4.0.10",
		},
		{
			name:     "redis ignores a leftover volume",
			resource: manifest.Resource{Name: "cache", Type: "redis"},
			objs:     []runtime.Object{resourcePVC("cache")},
			want:     "redis:8",
		},
		{
			name:     "memcached ignores a leftover volume",
			resource: manifest.Resource{Name: "cache", Type: "memcached"},
			objs:     []runtime.Object{resourcePVC("cache")},
			want:     "memcached:1.6",
		},
		{
			name:     "name is filtered like the template",
			resource: manifest.Resource{Name: "my_db", Type: "postgres"},
			objs:     []runtime.Object{resourceDeployment(nameFilter("my_db"), "postgres:10.5"), resourcePVC(nameFilter("my_db"))},
			want:     "postgres:10.5",
		},
		{
			name:     "custom image is used as is",
			prefix:   testEcrPrefix,
			resource: manifest.Resource{Name: "db", Type: "postgres", Image: "myorg/pg:1"},
			objs:     []runtime.Object{resourceDeployment("db", "postgres:10.5"), resourcePVC("db")},
			want:     "myorg/pg:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := resourceRenderProvider(tt.prefix, tt.objs...)
			out, err := renderResource(t, p, tt.resource)
			require.NoError(t, err)
			require.Contains(t, out, "image: "+tt.want+"\n")
		})
	}
}

func TestReleaseTemplateResourceUnknownVersion(t *testing.T) {
	tests := []struct {
		name     string
		resource manifest.Resource
		objs     []runtime.Object
	}{
		{
			name:     "type changed from postgres to postgis",
			resource: manifest.Resource{Name: "db", Type: "postgis"},
			objs:     []runtime.Object{resourceDeployment("db", "postgres:10.5"), resourcePVC("db")},
		},
		{
			name:     "postgres volume without a deployment",
			resource: manifest.Resource{Name: "db", Type: "postgres"},
			objs:     []runtime.Object{resourcePVC("db")},
		},
		{
			name:     "postgis volume without a deployment",
			resource: manifest.Resource{Name: "db", Type: "postgis"},
			objs:     []runtime.Object{resourcePVC("db")},
		},
		{
			name:     "mysql volume without a deployment",
			resource: manifest.Resource{Name: "db", Type: "mysql"},
			objs:     []runtime.Object{resourcePVC("db")},
		},
		{
			name:     "mariadb volume without a deployment",
			resource: manifest.Resource{Name: "db", Type: "mariadb"},
			objs:     []runtime.Object{resourcePVC("db")},
		},
		{
			name:     "digest only image",
			resource: manifest.Resource{Name: "db", Type: "postgres"},
			objs:     []runtime.Object{resourceDeployment("db", "postgres@sha256:0123abcd"), resourcePVC("db")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := resourceRenderProvider("", tt.objs...)
			_, err := renderResource(t, p, tt.resource)
			require.EqualError(t, err, "resource db has existing data but its running version is unknown, set options.version in convox.yml")
		})
	}
}

func TestReleaseTemplateResourceCustomImageSkipsLookup(t *testing.T) {
	p, kk := resourceRenderProvider("")
	_, err := renderResource(t, p, manifest.Resource{Name: "db", Type: "postgres", Image: "myorg/pg:1"})
	require.NoError(t, err)
	require.Empty(t, kk.Actions())
}

func TestReleaseTemplateResourceLookupErrors(t *testing.T) {
	for _, resource := range []string{"deployments", "persistentvolumeclaims"} {
		t.Run(resource, func(t *testing.T) {
			p, kk := resourceRenderProvider("")
			kk.PrependReactor("get", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("api unavailable")
			})
			_, err := renderResource(t, p, manifest.Resource{Name: "db", Type: "postgres"})
			require.ErrorContains(t, err, "api unavailable")
		})
	}
}
