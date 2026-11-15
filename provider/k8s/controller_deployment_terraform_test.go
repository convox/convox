package k8s

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

type terraformModule struct {
	body     *hclsyntax.Body
	src      []byte
	defaults map[string]cty.Value
}

func parseTerraformFile(t *testing.T, path string) (*hclsyntax.Body, []byte) {
	t.Helper()

	src, err := os.ReadFile(path)
	require.NoError(t, err)

	f, diags := hclparse.NewParser().ParseHCL(src, path)
	require.Falsef(t, diags.HasErrors(), "parse %s: %v", path, diags)

	body, ok := f.Body.(*hclsyntax.Body)
	require.Truef(t, ok, "coerce body for %s", path)

	return body, src
}

func loadTerraformModule(t *testing.T, name string) terraformModule {
	t.Helper()

	dir := filepath.Join("..", "..", "terraform", name)

	m := terraformModule{defaults: map[string]cty.Value{}}
	m.body, m.src = parseTerraformFile(t, filepath.Join(dir, "main.tf"))

	variables, _ := parseTerraformFile(t, filepath.Join(dir, "variables.tf"))

	for _, b := range variables.Blocks {
		if b.Type != "variable" {
			continue
		}

		attr, ok := b.Body.Attributes["default"]
		if !ok {
			continue
		}

		if v, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			m.defaults[b.Labels[0]] = v
		}
	}

	return m
}

func (m terraformModule) deployment(t *testing.T, name string) *hclsyntax.Body {
	t.Helper()

	for _, b := range m.body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 && b.Labels[0] == "kubernetes_deployment" && b.Labels[1] == name {
			return b.Body
		}
	}

	t.Fatalf("kubernetes_deployment.%s not found", name)

	return nil
}

func nestedTerraformBlock(t *testing.T, body *hclsyntax.Body, path ...string) *hclsyntax.Block {
	t.Helper()

	var block *hclsyntax.Block

	for _, blockType := range path {
		block = nil

		for _, b := range body.Blocks {
			if b.Type == blockType {
				block = b
				break
			}
		}

		require.NotNilf(t, block, "block %s", blockType)

		body = block.Body
	}

	return block
}

func (m terraformModule) stringMap(t *testing.T, body *hclsyntax.Body, name string, set map[string]cty.Value) map[string]string {
	t.Helper()

	attr, ok := body.Attributes[name]
	require.Truef(t, ok, "attribute %s", name)

	vars := maps.Clone(m.defaults)
	maps.Copy(vars, set)

	v, diags := attr.Expr.Value(&hcl.EvalContext{
		Variables: map[string]cty.Value{"var": cty.ObjectVal(vars)},
		Functions: map[string]function.Function{"merge": stdlib.MergeFunc},
	})
	require.Falsef(t, diags.HasErrors(), "evaluate %s: %v", name, diags)

	out := map[string]string{}

	if v.IsNull() {
		return out
	}

	for k, e := range v.AsValueMap() {
		s, err := convert.Convert(e, cty.String)
		require.NoErrorf(t, err, "%s[%s]", name, k)

		out[k] = s.AsString()
	}

	return out
}

func TestSingleReplicaBudgetAnnotation(t *testing.T) {
	tests := []struct {
		module     string
		deployment string
		gate       string
		always     map[string]string
	}{
		{module: "router/nginx", deployment: "ingress-nginx", gate: "replicas_max"},
		{module: "router/nginx", deployment: "ingress-nginx-internal", gate: "replicas_max", always: map[string]string{"managed-by": "convox"}},
		{module: "api/k8s", deployment: "api", gate: "replicas"},
		{module: "resolver/k8s", deployment: "resolver", gate: "replicas"},
	}

	for _, tt := range tests {
		t.Run(tt.deployment, func(t *testing.T) {
			m := loadTerraformModule(t, tt.module)
			d := m.deployment(t, tt.deployment)
			metadata := nestedTerraformBlock(t, d, "metadata").Body

			defaults := map[string]string{}
			maps.Copy(defaults, tt.always)
			assert.Equal(t, defaults, m.stringMap(t, metadata, "annotations", nil))

			single := map[string]string{AnnotationPdbMinAvailable: "1"}
			maps.Copy(single, tt.always)
			assert.Equal(t, single, m.stringMap(t, metadata, "annotations", map[string]cty.Value{tt.gate: cty.NumberIntVal(1)}))

			spec := nestedTerraformBlock(t, d, "spec").Range().SliceBytes(m.src)
			assert.NotContains(t, string(spec), AnnotationPdbMinAvailable)
		})
	}
}

func TestInternalRouterDeploymentLabels(t *testing.T) {
	m := loadTerraformModule(t, "router/nginx")
	d := m.deployment(t, "ingress-nginx-internal")
	set := map[string]cty.Value{"rack": cty.StringVal("rack1")}

	labels := m.stringMap(t, nestedTerraformBlock(t, d, "metadata").Body, "labels", set)
	pod := m.stringMap(t, nestedTerraformBlock(t, d, "spec", "template", "metadata").Body, "labels", set)

	for _, k := range append([]string{"name", "rack", "type"}, labelsForDeployment...) {
		assert.NotEmptyf(t, labels[k], "label %s", k)
		assert.Equalf(t, pod[k], labels[k], "label %s", k)
	}
}
