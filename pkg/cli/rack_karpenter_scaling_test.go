package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/rack"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const systemNodeMinCountParam = "karpenter_system_node_min_count_per_az"

// TestKarpenterSystemNodeMinCount asserts the system node group renders one node
// per zone at the parameter's default, so an existing rack plans no change, and
// the parameter's value when it is set. CI runs terraform validate with
// continue-on-error, so nothing else catches a change here.
func TestKarpenterSystemNodeMinCount(t *testing.T) {
	const path = "../../terraform/cluster/aws/main.tf"

	body := parseHCLBody(t, path)

	scaling := map[string]hclsyntax.Expression{}
	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) != 2 || block.Labels[0] != "aws_eks_node_group" || block.Labels[1] != "cluster" {
			continue
		}
		for _, inner := range block.Body.Blocks {
			if inner.Type != "scaling_config" {
				continue
			}
			for _, attr := range []string{"desired_size", "min_size"} {
				if a, ok := inner.Body.Attributes[attr]; ok {
					scaling[attr] = a.Expr
				}
			}
		}
	}

	cases := []struct {
		name      string
		karpenter bool
		perAZ     int64
		want      int64
	}{
		{"karpenter off is unaffected by the parameter", false, 5, 1},
		{"default renders today's literal", true, 1, 1},
		{"a raised minimum renders the parameter", true, 4, 4},
		{"the ceiling renders the parameter", true, 100, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &hcl.EvalContext{
				Variables: map[string]cty.Value{
					"var": cty.ObjectVal(map[string]cty.Value{
						"karpenter_enabled":     cty.BoolVal(tc.karpenter),
						"min_on_demand_count":   cty.NumberIntVal(1),
						"node_capacity_type":    cty.StringVal("ON_DEMAND"),
						systemNodeMinCountParam: cty.NumberIntVal(tc.perAZ),
					}),
				},
			}

			for _, attr := range []string{"desired_size", "min_size"} {
				expr, ok := scaling[attr]
				if !ok {
					t.Fatalf("no scaling_config %s on aws_eks_node_group.cluster in %s", attr, path)
				}

				for _, index := range []int64{0, 1, 2} {
					ctx.Variables["count"] = cty.ObjectVal(map[string]cty.Value{"index": cty.NumberIntVal(index)})

					v, diags := expr.Value(ctx)
					if diags.HasErrors() {
						t.Fatalf("evaluate %s at index %d: %v", attr, index, diags)
					}
					if !v.Equals(cty.NumberIntVal(tc.want)).True() {
						t.Errorf("%s at index %d is %s; want %d", attr, index, v.GoString(), tc.want)
					}
				}
			}
		})
	}
}

// TestKarpenterSystemNodeMaxPerAZMatchesModule pins rack.KarpenterSystemNodeMaxPerAZ,
// which bounds the parameter in the CLI and in the reconcile, against the max_size the
// module renders. A minimum above that maximum makes the apply invalid, and the pre-apply
// reconcile raises desired first, so the nodes launch before it fails.
func TestKarpenterSystemNodeMaxPerAZMatchesModule(t *testing.T) {
	const path = "../../terraform/cluster/aws/main.tf"

	var maxSize hclsyntax.Expression
	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "resource" || len(block.Labels) != 2 || block.Labels[0] != "aws_eks_node_group" || block.Labels[1] != "cluster" {
			continue
		}
		for _, inner := range block.Body.Blocks {
			if inner.Type != "scaling_config" {
				continue
			}
			if a, ok := inner.Body.Attributes["max_size"]; ok {
				maxSize = a.Expr
			}
		}
	}
	if maxSize == nil {
		t.Fatalf("no scaling_config max_size on aws_eks_node_group.cluster in %s", path)
	}

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{
				"karpenter_enabled":   cty.True,
				"max_on_demand_count": cty.NumberIntVal(1),
				"node_capacity_type":  cty.StringVal("MIXED"),
			}),
		},
	}

	for _, index := range []int64{0, 1, 2} {
		ctx.Variables["count"] = cty.ObjectVal(map[string]cty.Value{"index": cty.NumberIntVal(index)})

		v, diags := maxSize.Value(ctx)
		if diags.HasErrors() {
			t.Fatalf("evaluate max_size at index %d: %v", index, diags)
		}

		got, _ := v.AsBigFloat().Int64()
		if got < int64(rack.KarpenterSystemNodeMaxPerAZ) {
			t.Errorf("max_size at index %d is %d, below rack.KarpenterSystemNodeMaxPerAZ %d: the parameter would accept a minimum the module cannot render. This tree is missing the karpenter-enable-nodegroup-ceiling change to %s.", index, got, rack.KarpenterSystemNodeMaxPerAZ, path)
		}
	}
}

// TestKarpenterSystemNodeMinCountWired asserts the parameter is declared in both
// modules with the same default and passed through to the cluster module. Nothing
// ties params.yaml to the CLI maps to the Terraform variables, so a missing
// declaration or pass-through ships an inert parameter with a green CI run.
func TestKarpenterSystemNodeMinCountWired(t *testing.T) {
	for _, path := range []string{"../../terraform/cluster/aws/variables.tf", "../../terraform/system/aws/variables.tf"} {
		var found bool
		for _, block := range parseHCLBody(t, path).Blocks {
			if block.Type != "variable" || len(block.Labels) != 1 || block.Labels[0] != systemNodeMinCountParam {
				continue
			}
			found = true

			def, ok := block.Body.Attributes["default"]
			if !ok {
				t.Fatalf("%s: %s declares no default", path, systemNodeMinCountParam)
			}
			v, diags := def.Expr.Value(nil)
			if diags.HasErrors() {
				t.Fatalf("%s: evaluate default: %v", path, diags)
			}
			if !v.Equals(cty.NumberIntVal(1)).True() {
				t.Errorf("%s: default is %s; want 1", path, v.GoString())
			}
		}
		if !found {
			t.Errorf("%s: no variable %q", path, systemNodeMinCountParam)
		}
	}

	const path = "../../terraform/system/aws/main.tf"
	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "module" || len(block.Labels) != 1 || block.Labels[0] != "cluster" {
			continue
		}
		attr, ok := block.Body.Attributes[systemNodeMinCountParam]
		if !ok {
			t.Fatalf(`%s: module "cluster" does not pass %s`, path, systemNodeMinCountParam)
		}
		src := string(attr.Expr.Range().SliceBytes(mustReadFile(t, path)))
		if want := "var." + systemNodeMinCountParam; strings.TrimSpace(src) != want {
			t.Errorf("%s: %s is %q; want %q", path, systemNodeMinCountParam, src, want)
		}
	}
}

func parseHCLBody(t *testing.T, path string) *hclsyntax.Body {
	t.Helper()

	f, diags := hclparse.NewParser().ParseHCLFile(path)
	if diags.HasErrors() {
		t.Fatalf("hclparse %s: %v", path, diags)
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatalf("hclparse: failed to coerce body for %s", path)
	}
	return body
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
