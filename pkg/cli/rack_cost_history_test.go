package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

const costHistoryDaysParam = "cost_tracking_history_days"

func TestValidateAndMutateParams_CostTrackingHistoryDays(t *testing.T) {
	for _, provider := range []string{"aws", "azure", "gcp"} {
		for _, v := range []string{"31", "62", "400"} {
			if err := validateAndMutateParams(map[string]string{costHistoryDaysParam: v}, provider, map[string]string{}, false); err != nil {
				t.Errorf("%s: %s=%s rejected: %v", provider, costHistoryDaysParam, v, err)
			}
		}
		for _, v := range []string{"30", "401", "abc", "62.5"} {
			err := validateAndMutateParams(map[string]string{costHistoryDaysParam: v}, provider, map[string]string{}, false)
			if err == nil || err.Error() != "cost_tracking_history_days must be an integer from 31 to 400" {
				t.Errorf("%s: %s=%s: got %v", provider, costHistoryDaysParam, v, err)
			}
		}
	}

	if err := validateAndMutateParams(map[string]string{costHistoryDaysParam: "62"}, "do", map[string]string{}, false); err == nil {
		t.Errorf("%s should stay unknown for do", costHistoryDaysParam)
	}
}

func TestCostTrackingHistoryDaysWired(t *testing.T) {
	env := "name  = \"COST_TRACKING_HISTORY_DAYS\"\n            value = var." + costHistoryDaysParam + "\n"
	if !strings.Contains(string(mustReadFile(t, "../../terraform/api/k8s/main.tf")), env) {
		t.Errorf("terraform/api/k8s/main.tf does not set COST_TRACKING_HISTORY_DAYS from var.%s", costHistoryDaysParam)
	}

	assertCostHistoryDaysVariable(t, "../../terraform/api/k8s/variables.tf")

	known := map[string]map[string]bool{"aws": awsKnownParams, "azure": azureKnownParams, "gcp": gcpKnownParams}

	for _, provider := range []string{"aws", "azure", "gcp"} {
		if !known[provider][costHistoryDaysParam] {
			t.Errorf("%sKnownParams is missing %s", provider, costHistoryDaysParam)
		}

		yaml := string(mustReadFile(t, fmt.Sprintf("../../assets/provider/%s/params.yaml", provider)))
		if !strings.Contains(yaml, "- name: "+costHistoryDaysParam+"\n        default: \"62\"\n") {
			t.Errorf("%s params.yaml: %s missing or not defaulted to \"62\"", provider, costHistoryDaysParam)
		}

		for _, path := range []string{
			fmt.Sprintf("../../terraform/system/%s/variables.tf", provider),
			fmt.Sprintf("../../terraform/rack/%s/variables.tf", provider),
			fmt.Sprintf("../../terraform/api/%s/variables.tf", provider),
		} {
			assertCostHistoryDaysVariable(t, path)
		}

		for path, module := range map[string]string{
			fmt.Sprintf("../../terraform/system/%s/main.tf", provider): "rack",
			fmt.Sprintf("../../terraform/rack/%s/main.tf", provider):   "api",
			fmt.Sprintf("../../terraform/api/%s/main.tf", provider):    "k8s",
		} {
			assertCostHistoryDaysPassed(t, path, module)
		}
	}
}

func assertCostHistoryDaysVariable(t *testing.T, path string) {
	t.Helper()

	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "variable" || len(block.Labels) != 1 || block.Labels[0] != costHistoryDaysParam {
			continue
		}
		def, ok := block.Body.Attributes["default"]
		if !ok {
			t.Fatalf("%s: %s declares no default", path, costHistoryDaysParam)
		}
		v, diags := def.Expr.Value(nil)
		if diags.HasErrors() {
			t.Fatalf("%s: evaluate default: %v", path, diags)
		}
		if !v.Equals(cty.NumberIntVal(62)).True() {
			t.Errorf("%s: default is %s; want 62", path, v.GoString())
		}
		return
	}
	t.Errorf("%s: no variable %q", path, costHistoryDaysParam)
}

func assertCostHistoryDaysPassed(t *testing.T, path, module string) {
	t.Helper()

	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "module" || len(block.Labels) != 1 || block.Labels[0] != module {
			continue
		}
		attr, ok := block.Body.Attributes[costHistoryDaysParam]
		if !ok {
			t.Fatalf("%s: module %q does not pass %s", path, module, costHistoryDaysParam)
		}
		src := string(attr.Expr.Range().SliceBytes(mustReadFile(t, path)))
		if want := "var." + costHistoryDaysParam; strings.TrimSpace(src) != want {
			t.Errorf("%s: %s is %q; want %q", path, costHistoryDaysParam, src, want)
		}
		return
	}
	t.Errorf("%s: no module %q", path, module)
}
