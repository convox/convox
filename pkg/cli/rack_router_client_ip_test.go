package cli

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// With use-proxy-protocol and use-forwarded-headers, ingress-nginx 1.13+ renders this value as "real_ip_header proxy_protocol", which keeps the PROXY protocol address as the client IP.
func TestRouterProxyProtocolClientIP(t *testing.T) {
	const path = "../../terraform/router/aws/main.tf"
	const key = "forwarded-for-proxy-protocol-header"

	var data hclsyntax.Expression
	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == "kubernetes_config_map" && block.Labels[1] == "nginx-configuration" {
			if attr, ok := block.Body.Attributes["data"]; ok {
				data = attr.Expr
			}
		}
	}

	merge, ok := data.(*hclsyntax.FunctionCallExpr)
	if !ok || merge.Name != "merge" || len(merge.Args) < 2 {
		t.Fatalf("%s: kubernetes_config_map.nginx-configuration data is not a merge call", path)
	}

	src := mustReadFile(t, path)
	last := merge.Args[len(merge.Args)-1]
	if got := strings.TrimSpace(string(last.Range().SliceBytes(src))); got != "local.additional_config_map" {
		t.Errorf("%s: last merge argument is %q; want local.additional_config_map so nginx_additional_config overrides", path, got)
	}

	var values []string
	for _, arg := range merge.Args[:len(merge.Args)-1] {
		obj, ok := arg.(*hclsyntax.ObjectConsExpr)
		if !ok {
			continue
		}
		for _, item := range obj.Items {
			k, diags := item.KeyExpr.Value(nil)
			if diags.HasErrors() || !k.Type().Equals(cty.String) || k.AsString() != key {
				continue
			}
			v, diags := item.ValueExpr.Value(nil)
			if diags.HasErrors() || !v.Type().Equals(cty.String) {
				t.Fatalf("%s: %s is not a literal string: %v", path, key, diags)
			}
			values = append(values, v.AsString())
		}
	}

	if len(values) != 1 || values[0] != "proxy_protocol" {
		t.Errorf("%s: %s values before local.additional_config_map are %q; want exactly [proxy_protocol]", path, key, values)
	}
}
