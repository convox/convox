package rack

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

const dependencyAnnotation = "convox.io/dependency"

var chartOrderedDeployments = []struct {
	file string
	name string
}{
	{"router/nginx/main.tf", "ingress-nginx"},
	{"api/k8s/main.tf", "api"},
}

type terraformFile struct {
	body *hclsyntax.Body
	src  []byte
}

func readTerraformFile(t *testing.T, rel string) terraformFile {
	t.Helper()

	root, err := repoRootFromTestFile()
	require.NoError(t, err)

	path := filepath.Join(root, "terraform", rel)

	src, err := os.ReadFile(path)
	require.NoError(t, err)

	return terraformFile{body: parseBody(t, path), src: src}
}

func (f terraformFile) text(r hcl.Range) string {
	return strings.TrimSpace(string(r.SliceBytes(f.src)))
}

func terraformResource(t *testing.T, body *hclsyntax.Body, kind, name string) *hclsyntax.Body {
	t.Helper()

	for _, b := range body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 && b.Labels[0] == kind && b.Labels[1] == name {
			return b.Body
		}
	}

	t.Fatalf("resource %s.%s not found", kind, name)

	return nil
}

func nestedBlocks(body *hclsyntax.Body, blockType string) []*hclsyntax.Block {
	var out []*hclsyntax.Block

	for _, b := range body.Blocks {
		if b.Type == blockType {
			out = append(out, b)
		}
	}

	return out
}

func nestedBlock(t *testing.T, body *hclsyntax.Body, path ...string) *hclsyntax.Block {
	t.Helper()

	var block *hclsyntax.Block

	for _, blockType := range path {
		blocks := nestedBlocks(body, blockType)
		require.Lenf(t, blocks, 1, "block %s", blockType)

		block = blocks[0]
		body = block.Body
	}

	return block
}

func evalAnnotations(t *testing.T, expr hclsyntax.Expression, helmIDs string) map[string]string {
	t.Helper()

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{"cluster_helm_ids": cty.StringVal(helmIDs)}),
		},
		Functions: map[string]function.Function{"merge": stdlib.MergeFunc},
	}

	v, diags := expr.Value(ctx)
	require.Falsef(t, diags.HasErrors(), "evaluate annotations: %v", diags)

	out := map[string]string{}

	if v.IsNull() {
		return out
	}

	for k, e := range v.AsValueMap() {
		out[k] = e.AsString()
	}

	return out
}

func TestClusterHelmIDsCoversEveryChart(t *testing.T) {
	root, err := repoRootFromTestFile()
	require.NoError(t, err)

	entries, err := os.ReadDir(filepath.Join(root, "terraform", "cluster", "aws"))
	require.NoError(t, err)

	var want []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}

		f := readTerraformFile(t, filepath.Join("cluster", "aws", e.Name()))

		for _, b := range f.body.Blocks {
			if b.Type != "resource" || len(b.Labels) != 2 || b.Labels[0] != "helm_release" {
				continue
			}

			name := b.Labels[1]

			count, ok := b.Body.Attributes["count"]
			if !ok {
				want = append(want, fmt.Sprintf("helm_release.%s.id", name))
				continue
			}

			cond, ok := count.Expr.(*hclsyntax.ConditionalExpr)
			require.Truef(t, ok, "count of helm_release.%s is not a conditional", name)
			require.Equalf(t, "1", f.text(cond.TrueResult.Range()), "count of helm_release.%s", name)
			require.Equalf(t, "0", f.text(cond.FalseResult.Range()), "count of helm_release.%s", name)

			want = append(want, fmt.Sprintf(`%s ? helm_release.%s[0].id : ""`, f.text(cond.Condition.Range()), name))
		}
	}

	outputs := readTerraformFile(t, "cluster/aws/outputs.tf")

	value, ok := labelledBlock(t, outputs.body, "output", "cluster_helm_ids").Attributes["value"]
	require.True(t, ok, "cluster_helm_ids has no value")

	call, ok := value.Expr.(*hclsyntax.FunctionCallExpr)
	require.True(t, ok, "cluster_helm_ids is not a function call")
	require.Equal(t, "join", call.Name)
	require.Len(t, call.Args, 2)

	list, ok := call.Args[1].(*hclsyntax.TupleConsExpr)
	require.True(t, ok, "cluster_helm_ids does not join a list")

	var got []string

	for _, e := range list.Exprs {
		got = append(got, outputs.text(e.Range()))
	}

	assert.ElementsMatch(t, want, got)
}

func TestClusterHelmIDsReachesDeployments(t *testing.T) {
	for _, hop := range []struct {
		file   string
		module string
		want   string
	}{
		{"system/aws/main.tf", "rack", "module.cluster.cluster_helm_ids"},
		{"rack/aws/main.tf", "api", "var.cluster_helm_ids"},
		{"rack/aws/main.tf", "router", "var.cluster_helm_ids"},
		{"api/aws/main.tf", "k8s", "var.cluster_helm_ids"},
		{"router/aws/main.tf", "nginx", "var.cluster_helm_ids"},
	} {
		f := readTerraformFile(t, hop.file)

		attr, ok := labelledBlock(t, f.body, "module", hop.module).Attributes["cluster_helm_ids"]
		require.Truef(t, ok, "%s module %s does not pass cluster_helm_ids", hop.file, hop.module)
		assert.Equalf(t, hop.want, f.text(attr.Expr.Range()), "%s module %s", hop.file, hop.module)
	}
}

func TestSystemDeploymentsDependencyAnnotation(t *testing.T) {
	for _, d := range chartOrderedDeployments {
		t.Run(d.name, func(t *testing.T) {
			f := readTerraformFile(t, d.file)
			deployment := terraformResource(t, f.body, "kubernetes_deployment", d.name)

			annotations, ok := nestedBlock(t, deployment, "metadata").Body.Attributes["annotations"]
			require.True(t, ok, "metadata has no annotations")

			unset := evalAnnotations(t, annotations.Expr, "")
			assert.NotContains(t, unset, dependencyAnnotation)

			want := maps.Clone(unset)
			want[dependencyAnnotation] = "a,b"
			assert.Equal(t, want, evalAnnotations(t, annotations.Expr, "a,b"))

			spec := nestedBlock(t, deployment, "spec")
			assert.NotContains(t, f.text(spec.Range()), "cluster_helm_ids")
		})
	}
}

func TestRouterAndAPITolerateSystemNode(t *testing.T) {
	for _, d := range chartOrderedDeployments {
		t.Run(d.name, func(t *testing.T) {
			f := readTerraformFile(t, d.file)
			deployment := terraformResource(t, f.body, "kubernetes_deployment", d.name)
			pod := nestedBlock(t, deployment, "spec", "template", "spec").Body

			for _, b := range nestedBlocks(pod, "dynamic") {
				assert.NotEqual(t, []string{"toleration"}, b.Labels)
			}

			tolerations := nestedBlocks(pod, "toleration")
			require.Len(t, tolerations, 1)
			assert.Equal(t, map[string]string{
				"key":      "convox.io/system-node",
				"operator": "Equal",
				"value":    "true",
				"effect":   "NoSchedule",
			}, stringAttrs(t, tolerations[0].Body))
		})
	}
}
