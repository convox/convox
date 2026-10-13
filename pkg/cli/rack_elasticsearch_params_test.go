package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

var elasticsearchProviders = []string{"azure", "do", "gcp", "metal"}

func TestValidateAndMutateParams_ElasticsearchDisk(t *testing.T) {
	tests := []struct {
		value   string
		current string
		force   bool
		wantErr bool
	}{
		{"5Gi", "", false, false},
		{"10Gi", "", false, false},
		{"120Gi", "", false, false},
		{"20Gi", "20Gi", false, false},
		{"30Gi", "20Gi", false, false},
		{"10Gi", "10G", false, false},
		{"9Gi", "10G", false, true},
		{"10Gi", "not-a-size", false, false},
		{"10Gi", "20Gi", false, true},
		{"10Gi", "20Gi", true, false},
		{"4Gi", "", false, true},
		{"05Gi", "", false, true},
		{"5G", "", false, true},
		{"5gi", "", false, true},
		{"5", "", false, true},
		{"5.5Gi", "", false, true},
		{"", "", false, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/%t", tt.value, tt.current, tt.force), func(t *testing.T) {
			for _, provider := range elasticsearchProviders {
				current := map[string]string{}
				if tt.current != "" {
					current["elasticsearch_disk"] = tt.current
				}
				err := validateAndMutateParams(map[string]string{"elasticsearch_disk": tt.value}, provider, current, tt.force)
				if tt.wantErr && err == nil {
					t.Errorf("%s: %q accepted", provider, tt.value)
				}
				if !tt.wantErr && err != nil {
					t.Errorf("%s: %q rejected: %v", provider, tt.value, err)
				}
			}
		})
	}

	for _, provider := range []string{"aws", "local"} {
		if err := validateAndMutateParams(map[string]string{"elasticsearch_disk": "10Gi"}, provider, map[string]string{}, false); err == nil {
			t.Errorf("elasticsearch_disk should be unknown on %s", provider)
		}
	}
}

func TestElasticsearchDiskWired(t *testing.T) {
	for _, provider := range elasticsearchProviders {
		for _, tier := range []string{"system", "rack", "api"} {
			path := fmt.Sprintf("../../terraform/%s/%s/variables.tf", tier, provider)
			found := false

			for _, block := range parseHCLBody(t, path).Blocks {
				if block.Type != "variable" || len(block.Labels) != 1 || block.Labels[0] != "elasticsearch_disk" {
					continue
				}
				found = true
				def, ok := block.Body.Attributes["default"]
				if !ok {
					t.Errorf("%s: elasticsearch_disk declares no default", path)
					continue
				}
				if src := strings.TrimSpace(string(def.Expr.Range().SliceBytes(mustReadFile(t, path)))); src != `"5Gi"` {
					t.Errorf("%s: elasticsearch_disk default is %s; want \"5Gi\"", path, src)
				}
			}
			if !found {
				t.Errorf("%s: no variable elasticsearch_disk", path)
			}
		}

		assertFluentdParamPassed(t, fmt.Sprintf("../../terraform/system/%s/main.tf", provider), "rack", "elasticsearch_disk")
		assertFluentdParamPassed(t, fmt.Sprintf("../../terraform/rack/%s/main.tf", provider), "api", "elasticsearch_disk")

		path := fmt.Sprintf("../../terraform/api/%s/main.tf", provider)
		if src := moduleEnv(t, path, "k8s")["ELASTICSEARCH_DISK"]; src != "var.elasticsearch_disk" {
			t.Errorf("%s: module k8s env ELASTICSEARCH_DISK is %q; want var.elasticsearch_disk", path, src)
		}
	}
}

func moduleEnv(t *testing.T, path, module string) map[string]string {
	t.Helper()

	src := mustReadFile(t, path)
	env := map[string]string{}

	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "module" || len(block.Labels) != 1 || block.Labels[0] != module {
			continue
		}
		attr, ok := block.Body.Attributes["env"]
		if !ok {
			t.Fatalf("%s: module %q has no env", path, module)
		}
		obj, ok := attr.Expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			t.Fatalf("%s: module %q env is not an object", path, module)
		}
		for _, item := range obj.Items {
			key := strings.TrimSpace(string(item.KeyExpr.Range().SliceBytes(src)))
			env[key] = strings.TrimSpace(string(item.ValueExpr.Range().SliceBytes(src)))
		}
	}

	return env
}
