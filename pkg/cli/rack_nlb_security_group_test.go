package cli

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

const routerAWSMain = "../../terraform/router/aws/main.tf"

func routerServiceAnnotations(t *testing.T, name, nlbSecurityGroup string) map[string]map[string]string {
	t.Helper()

	locals := map[string]hclsyntax.Expression{}
	annotations := map[string]hclsyntax.Expression{}
	for _, block := range parseHCLBody(t, routerAWSMain).Blocks {
		switch {
		case block.Type == "locals":
			for key, attr := range block.Body.Attributes {
				locals[key] = attr.Expr
			}
		case block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == "kubernetes_service":
			for _, inner := range block.Body.Blocks {
				if attr, ok := inner.Body.Attributes["annotations"]; inner.Type == "metadata" && ok {
					annotations[block.Labels[1]] = attr.Expr
				}
			}
		}
	}

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{
				"idle_timeout":       cty.StringVal("3600"),
				"lbc_helm_id":        cty.StringVal("lbc"),
				"name":               cty.StringVal(name),
				"nlb_security_group": cty.StringVal(nlbSecurityGroup),
				"proxy_protocol":     cty.True,
				"tags":               cty.MapValEmpty(cty.String),
			}),
		},
		Functions: map[string]function.Function{
			"join":      stdlib.JoinFunc,
			"length":    stdlib.StrlenFunc,
			"merge":     stdlib.MergeFunc,
			"trimspace": stdlib.TrimSpaceFunc,
		},
	}

	local := map[string]cty.Value{}
	for _, l := range []string{"tags", "nlb_security_group", "nlb_security_group_annotation", "lb_names", "lb_name_annotations"} {
		expr, ok := locals[l]
		if !ok {
			t.Fatalf("%s: no local %s", routerAWSMain, l)
		}
		ctx.Variables["local"] = cty.ObjectVal(local)
		v, diags := expr.Value(ctx)
		if diags.HasErrors() {
			t.Fatalf("%s: evaluate local.%s with name=%q nlb_security_group=%q: %v", routerAWSMain, l, name, nlbSecurityGroup, diags)
		}
		local[l] = v
	}
	ctx.Variables["local"] = cty.ObjectVal(local)

	services := map[string]map[string]string{}
	for _, svc := range []string{"router", "router_extra", "router-internal"} {
		expr, ok := annotations[svc]
		if !ok {
			t.Fatalf("%s: no annotations on kubernetes_service.%s", routerAWSMain, svc)
		}
		v, diags := expr.Value(ctx)
		if diags.HasErrors() {
			t.Fatalf("%s: evaluate kubernetes_service.%s annotations with name=%q nlb_security_group=%q: %v", routerAWSMain, svc, name, nlbSecurityGroup, diags)
		}

		got := map[string]string{}
		for it := v.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			got[k.AsString()] = ev.AsString()
		}
		if got["service.beta.kubernetes.io/aws-load-balancer-type"] != "nlb" {
			t.Fatalf("kubernetes_service.%s: annotations did not evaluate to the router map: %v", svc, got)
		}
		services[svc] = got
	}

	return services
}

// The in-tree AWS cloud provider at Kubernetes 1.35 rejects this key on a legacy NLB by presence, so an empty value must not render it.
func TestRouterSecurityGroupAnnotationOmittedWhenEmpty(t *testing.T) {
	const key = "service.beta.kubernetes.io/aws-load-balancer-security-groups"

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
		for name, got := range routerServiceAnnotations(t, "my-rack", tc.value) {
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

func TestRouterLoadBalancerNameFallback(t *testing.T) {
	const key = "service.beta.kubernetes.io/aws-load-balancer-name"

	cases := []struct {
		length                  int
		router, internal, extra bool
	}{
		{16, true, true, true},
		{17, true, false, true},
		{19, true, false, true},
		{20, true, false, false},
		{25, true, false, false},
		{26, false, false, false},
	}

	for _, tc := range cases {
		name := strings.Repeat("a", tc.length)
		got := routerServiceAnnotations(t, name, "")

		for _, svc := range []struct {
			resource string
			prefix   string
			present  bool
		}{
			{"router", "router-", tc.router},
			{"router-internal", "router-internal-", tc.internal},
			{"router_extra", "router-extra-", tc.extra},
		} {
			want := ""
			if svc.present {
				want = svc.prefix + name
			}
			v, present := got[svc.resource][key]
			if present != svc.present || v != want {
				t.Errorf("kubernetes_service.%s with a %d character rack name: annotation present=%v value=%q; want present=%v value=%q", svc.resource, tc.length, present, v, svc.present, want)
			}
		}
	}
}
