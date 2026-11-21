package cli

import (
	"fmt"
	"strings"
	"testing"
)

var routerUserSnippetsProviders = []string{"aws", "azure", "do", "gcp", "metal"}

func TestValidateAndMutateParams_RouterUserSnippetsDisable(t *testing.T) {
	const (
		snippetWarning = "WARNING: apps already using -snippet ingress annotations keep them until their next deploy"
		pairingWarning = "WARNING: router_user_snippets_disable does not stop apps that call the rack API through RACK_URL"
	)

	for _, provider := range routerUserSnippetsProviders {
		for _, tt := range []struct {
			name    string
			params  map[string]string
			current map[string]string
			want    string
			snippet bool
			pairing bool
			wantErr bool
		}{
			{name: "true", params: map[string]string{"router_user_snippets_disable": "true"}, want: "true", snippet: true, pairing: true},
			{name: "parsebool form", params: map[string]string{"router_user_snippets_disable": "1"}, want: "true", snippet: true, pairing: true},
			{name: "false", params: map[string]string{"router_user_snippets_disable": "false"}, want: "false"},
			{name: "garbage", params: map[string]string{"router_user_snippets_disable": "yesno"}, wantErr: true},
			{name: "url disable stored", params: map[string]string{"router_user_snippets_disable": "true"}, current: map[string]string{"app_rack_url_disable": "true"}, want: "true", snippet: true},
			{name: "url disable stored false", params: map[string]string{"router_user_snippets_disable": "true"}, current: map[string]string{"app_rack_url_disable": "false"}, want: "true", snippet: true, pairing: true},
		} {
			current := tt.current
			if current == nil {
				current = map[string]string{}
			}

			var err error
			out := captureStderr(t, func() {
				err = validateAndMutateParams(tt.params, provider, current, false)
			})

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s/%s: accepted", provider, tt.name)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s/%s: rejected: %v", provider, tt.name, err)
				continue
			}
			if got := tt.params["router_user_snippets_disable"]; got != tt.want {
				t.Errorf("%s/%s: stored %q; want %q", provider, tt.name, got, tt.want)
			}
			if strings.Contains(out, snippetWarning) != tt.snippet {
				t.Errorf("%s/%s: snippet warning printed = %v; want %v (stderr %q)", provider, tt.name, !tt.snippet, tt.snippet, out)
			}
			if strings.Contains(out, pairingWarning) != tt.pairing {
				t.Errorf("%s/%s: pairing warning printed = %v; want %v (stderr %q)", provider, tt.name, !tt.pairing, tt.pairing, out)
			}
		}
	}

	err := validateAndMutateParams(map[string]string{"router_user_snippets_disable": "true"}, "local", map[string]string{}, false)
	if err == nil || !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("local: got %v; want unknown parameter", err)
	}
}

func TestRouterUserSnippetsDisableWired(t *testing.T) {
	const param = "router_user_snippets_disable"

	if localKnownParams[param] {
		t.Errorf("localKnownParams has %s", param)
	}
	if !boolParams[param] {
		t.Errorf("boolParams is missing %s", param)
	}
	for _, group := range []string{"ingress", "security"} {
		if !paramGroups[group][param] {
			t.Errorf("paramGroups[%q] is missing %s", group, param)
		}
	}

	for _, provider := range routerUserSnippetsProviders {
		if !providerKnownParams[provider][param] {
			t.Errorf("%sKnownParams is missing %s", provider, param)
		}

		for _, tier := range []string{"system", "rack", "api"} {
			path := fmt.Sprintf("../../terraform/%s/%s/variables.tf", tier, provider)
			found := false

			for _, block := range parseHCLBody(t, path).Blocks {
				if block.Type != "variable" || len(block.Labels) != 1 || block.Labels[0] != param {
					continue
				}
				found = true
				def, ok := block.Body.Attributes["default"]
				if !ok {
					t.Errorf("%s: %s declares no default", path, param)
					continue
				}
				if src := strings.TrimSpace(string(def.Expr.Range().SliceBytes(mustReadFile(t, path)))); src != "false" {
					t.Errorf("%s: %s default is %s; want false", path, param, src)
				}
			}
			if !found {
				t.Errorf("%s: no variable %s", path, param)
			}
		}

		assertFluentdParamPassed(t, fmt.Sprintf("../../terraform/system/%s/main.tf", provider), "rack", param)
		assertFluentdParamPassed(t, fmt.Sprintf("../../terraform/rack/%s/main.tf", provider), "api", param)

		path := fmt.Sprintf("../../terraform/api/%s/main.tf", provider)
		if src := moduleEnv(t, path, "k8s")["ROUTER_USER_SNIPPETS_DISABLE"]; src != "var."+param {
			t.Errorf("%s: module k8s env ROUTER_USER_SNIPPETS_DISABLE is %q; want var.%s", path, src, param)
		}
	}
}
