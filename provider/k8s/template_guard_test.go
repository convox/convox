package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/templater"
	ktmpl "github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestCheckRenderedKinds(t *testing.T) {
	params := map[string]interface{}{"Namespace": "rack-app"}

	tests := []struct {
		Name     string
		Template string
		Data     string
		Err      string
	}{
		{"Allowed", "app/service", "kind: Secret\nmetadata:\n  name: env-web\n  namespace: rack-app\n---\nkind: Deployment\nmetadata:\n  name: web\n", ""},
		{"NullDocument", "app/timer", "---\n\n---\nkind: CronJob\nmetadata:\n  name: t\n", ""},
		{"WrongKind", "app/service", "kind: ClusterRoleBinding\nmetadata:\n  name: x\n", `template app/service rendered kind "ClusterRoleBinding"`},
		{"WrongNamespace", "app/service", "kind: Secret\nmetadata:\n  name: x\n  namespace: kube-system\n", `template app/service rendered Secret in namespace "kube-system"`},
		{"OtherNamespace", "app/app", "kind: Namespace\nmetadata:\n  name: kube-system\n", `template app/app rendered namespace "kube-system"`},
		{"ClusterScopedFromService", "app/service", "kind: PersistentVolume\nmetadata:\n  name: x\n", `template app/service rendered kind "PersistentVolume"`},
		{"Resource", "resource/postgres", "kind: Deployment\nmetadata:\n  name: x\n---\nkind: Role\nmetadata:\n  name: x\n", `template resource/postgres rendered kind "Role"`},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := checkRenderedKinds(test.Template, []byte(test.Data), params)
			if test.Err == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.Err)
		})
	}
}

func TestRenderTemplateGuardWired(t *testing.T) {
	p := &Provider{Engine: &mock.TestEngine{}, Name: "rack"}
	p.templater = templater.New(ktmpl.TemplatesFS)

	_, err := p.RenderTemplate("resource/elasticache-redis", map[string]interface{}{
		"App": "myapp", "Namespace": "rack-myapp", "Rack": "rack", "Host": "h", "Port": "6379", "Password": "",
		"Name": "c\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: x\n  labels:\n    a: b",
	})
	require.EqualError(t, err, `template resource/elasticache-redis rendered kind "ClusterRole"`)
}

func TestApplyLabelsKeepsBlockScalar(t *testing.T) {
	data := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\ndata:\n  e: see ---\n  k: |-\n    x\n    ---\n    kind: ClusterRole\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n")

	out, err := ApplyLabels(data, "app=x")
	require.NoError(t, err)

	docs := atom.SplitDocuments(out)
	require.Len(t, docs, 2)

	var v struct{ Data map[string]string }
	require.NoError(t, yaml.Unmarshal(docs[0], &v))
	require.Equal(t, "x\n---\nkind: ClusterRole", v.Data["k"])
	require.Equal(t, "see ---", v.Data["e"])
}
