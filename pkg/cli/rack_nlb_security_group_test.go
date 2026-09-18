package cli

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// The in-tree AWS cloud provider at Kubernetes 1.35 rejects this key on a legacy NLB by presence, so an empty value must not render it.
func TestRouterSecurityGroupAnnotationOmittedWhenEmpty(t *testing.T) {
	const path = "../../terraform/router/aws/main.tf"
	const key = "service.beta.kubernetes.io/aws-load-balancer-security-groups"

	locals := map[string]hclsyntax.Expression{}
	annotations := map[string]hclsyntax.Expression{}
	for _, block := range parseHCLBody(t, path).Blocks {
		switch {
		case block.Type == "locals":
			for name, attr := range block.Body.Attributes {
				locals[name] = attr.Expr
			}
		case block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == "kubernetes_service":
			for _, inner := range block.Body.Blocks {
				if attr, ok := inner.Body.Attributes["annotations"]; inner.Type == "metadata" && ok {
					annotations[block.Labels[1]] = attr.Expr
				}
			}
		}
	}

	cases := []struct {
		value string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"sg-0123abcd", "sg-0123abcd"},
		{"  sg-0123abcd  ", "sg-0123abcd"},
	}

	for _, tc := range cases {
		ctx := &hcl.EvalContext{
			Variables: map[string]cty.Value{
				"var": cty.ObjectVal(map[string]cty.Value{
					"idle_timeout":       cty.StringVal("3600"),
					"lbc_helm_id":        cty.StringVal("lbc"),
					"name":               cty.StringVal("my-rack"),
					"nlb_security_group": cty.StringVal(tc.value),
					"proxy_protocol":     cty.True,
					"tags":               cty.MapValEmpty(cty.String),
				}),
			},
			Functions: map[string]function.Function{
				"join":      stdlib.JoinFunc,
				"merge":     stdlib.MergeFunc,
				"trimspace": stdlib.TrimSpaceFunc,
			},
		}

		local := map[string]cty.Value{}
		for _, name := range []string{"tags", "nlb_security_group", "nlb_security_group_annotation"} {
			expr, ok := locals[name]
			if !ok {
				t.Fatalf("%s: no local %s", path, name)
			}
			ctx.Variables["local"] = cty.ObjectVal(local)
			v, diags := expr.Value(ctx)
			if diags.HasErrors() {
				t.Fatalf("%s: evaluate local.%s with nlb_security_group=%q: %v", path, name, tc.value, diags)
			}
			local[name] = v
		}
		ctx.Variables["local"] = cty.ObjectVal(local)

		for _, name := range []string{"router", "router_extra", "router-internal"} {
			expr, ok := annotations[name]
			if !ok {
				t.Fatalf("%s: no annotations on kubernetes_service.%s", path, name)
			}
			v, diags := expr.Value(ctx)
			if diags.HasErrors() {
				t.Fatalf("%s: evaluate kubernetes_service.%s annotations with nlb_security_group=%q: %v", path, name, tc.value, diags)
			}

			got := map[string]string{}
			for it := v.ElementIterator(); it.Next(); {
				k, ev := it.Element()
				got[k.AsString()] = ev.AsString()
			}
			if got["service.beta.kubernetes.io/aws-load-balancer-type"] != "nlb" {
				t.Fatalf("kubernetes_service.%s: annotations did not evaluate to the router map: %v", name, got)
			}

			want := tc.want
			if name == "router-internal" {
				want = ""
			}
			sg, present := got[key]
			if present != (want != "") || sg != want {
				t.Errorf("kubernetes_service.%s with nlb_security_group=%q: annotation present=%v value=%q; want present=%v value=%q", name, tc.value, present, sg, want != "", want)
			}
		}
	}
}
