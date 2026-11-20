package cli

import (
	"fmt"
	"strings"
	"testing"
)

var appRackURLProviders = []string{"aws", "azure", "do", "gcp", "metal"}

func TestValidateAndMutateParams_AppRackURLDisable(t *testing.T) {
	const warning = "WARNING: running app processes keep RACK_URL until each app's next deploy."

	for _, provider := range appRackURLProviders {
		for _, tt := range []struct {
			value   string
			want    string
			warns   bool
			wantErr bool
		}{
			{value: "1", want: "true", warns: true},
			{value: "false", want: "false"},
			{value: "garbage", wantErr: true},
		} {
			params := map[string]string{"app_rack_url_disable": tt.value}
			var err error
			out := captureStderr(t, func() {
				err = validateAndMutateParams(params, provider, map[string]string{}, false)
			})

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: %q accepted", provider, tt.value)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s: %q rejected: %v", provider, tt.value, err)
				continue
			}
			if params["app_rack_url_disable"] != tt.want {
				t.Errorf("%s: %q stored as %q; want %q", provider, tt.value, params["app_rack_url_disable"], tt.want)
			}
			if strings.Contains(out, warning) != tt.warns {
				t.Errorf("%s: %q warning printed = %v; want %v (stderr %q)", provider, tt.value, !tt.warns, tt.warns, out)
			}
		}
	}

	err := validateAndMutateParams(map[string]string{"app_rack_url_disable": "true"}, "local", map[string]string{}, false)
	if err == nil || !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("local: got %v; want unknown parameter", err)
	}
}

func TestAppRackURLDisableWired(t *testing.T) {
	const param = "app_rack_url_disable"

	if localKnownParams[param] {
		t.Errorf("localKnownParams has %s", param)
	}

	for _, provider := range appRackURLProviders {
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
		if src := moduleEnv(t, path, "k8s")["APP_RACK_URL_DISABLE"]; src != "var."+param {
			t.Errorf("%s: module k8s env APP_RACK_URL_DISABLE is %q; want var.%s", path, src, param)
		}
	}
}
