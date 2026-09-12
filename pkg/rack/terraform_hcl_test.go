package rack

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// ordinaryVars are values the escaper must leave byte-identical, including a
// bare $ and a bare %, which are special only before {. Every rack rewrites
// main.tf on its next mutation, so a byte difference here reaches the whole
// fleet at once.
var ordinaryVars = map[string]string{
	"additional_node_groups_config": "W3siaWQiOjEsInR5cGUiOiJ0My5zbWFsbCJ9XQ==",
	"availability_zones":            "us-east-2a,us-east-2b,us-east-2c",
	"cost_tracking_enable":          "true",
	"emoji_label":                   "\U0001f389",
	"empty_value":                   "",
	"name":                          "my-rack",
	"node_type":                     "t3.small",
	"percent_literal":               "100% off",
	"price_literal":                 "$5 per hour",
	"release":                       "3.25.7",
	"releases_to_retain":            "0",
}

func renderTemplate(t *testing.T, vars map[string]string, provider string) (string, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "main.tf")
	params := map[string]interface{}{
		"Name":     "my-rack",
		"Provider": provider,
		"Vars":     vars,
	}
	require.NoError(t, terraformWriteTemplate(path, vars["release"], params))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return path, string(data)
}

func parseBody(t *testing.T, path string) *hclsyntax.Body {
	t.Helper()

	f, diags := hclparse.NewParser().ParseHCLFile(path)
	require.Falsef(t, diags.HasErrors(), "parse %s: %v", path, diags)

	body, ok := f.Body.(*hclsyntax.Body)
	require.Truef(t, ok, "coerce body for %s", path)

	return body
}

// stringAttrs evaluates every string-valued attribute of body against an empty
// context. Attributes that reference anything (output "api" reads
// module.system.api) or are not strings (sensitive, skip_cert_verification)
// are skipped rather than failed.
func stringAttrs(t *testing.T, body *hclsyntax.Body) map[string]string {
	t.Helper()

	out := map[string]string{}

	for name, attr := range body.Attributes {
		v, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !v.Type().Equals(cty.String) {
			continue
		}
		out[name] = v.AsString()
	}

	return out
}

func labelledBlock(t *testing.T, body *hclsyntax.Body, blockType, label string) *hclsyntax.Body {
	t.Helper()

	for _, b := range body.Blocks {
		if b.Type == blockType && len(b.Labels) > 0 && b.Labels[0] == label {
			return b.Body
		}
	}

	t.Fatalf("block %s %q not found", blockType, label)

	return nil
}

func TestHCLEscapeLeavesOrdinaryValuesUnchanged(t *testing.T) {
	for _, v := range ordinaryVars {
		assert.Equalf(t, v, hclEscape(v), "hclEscape must be identity for %q", v)
	}
}

func TestTerraformWriteTemplateRendersOrdinaryValuesUnchanged(t *testing.T) {
	t.Setenv("CONVOX_TERRAFORM_SOURCE", "")

	before := maps.Clone(ordinaryVars)
	_, rendered := renderTemplate(t, ordinaryVars, "aws")
	assert.Equal(t, before, ordinaryVars, "the writer must not mutate the caller's Vars map")

	for k, v := range ordinaryVars {
		assert.Containsf(t, rendered, "\n\t\t\t"+k+` = "`+v+`"`, "rendering of %s changed", k)
	}

	assert.Contains(t, rendered, `source = "github.com/convox/convox//terraform/system/aws?ref=3.25.7"`)
	assert.Contains(t, rendered, "\n\t\t\tvalue = \"aws\"")
	assert.Contains(t, rendered, "\n\t\t\tvalue = \"3.25.7\"")
}

// An unwired interpolation renders identically to a wired one for every
// ordinary value, so only a special character proves the helper is applied.
// A miss on any of these three sites leaves main.tf unparseable.
func TestTerraformWriteTemplateEscapesEveryInterpolation(t *testing.T) {
	t.Setenv("CONVOX_TERRAFORM_SOURCE", `./mod"s/%s`)

	vars := map[string]string{"name": "my-rack", "release": `r"1`}
	path, _ := renderTemplate(t, vars, `aws"x`)

	body := parseBody(t, path)

	module := stringAttrs(t, labelledBlock(t, body, "module", "system"))
	assert.Equal(t, `./mod"s/aws"x`, module["source"])
	assert.Equal(t, `r"1`, module["release"])

	assert.Equal(t, `aws"x`, stringAttrs(t, labelledBlock(t, body, "output", "provider"))["value"])
	assert.Equal(t, `r"1`, stringAttrs(t, labelledBlock(t, body, "output", "release"))["value"])
}

func TestTerraformWriteBackendEscapesCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.tf")
	require.NoError(t, terraformWriteBackend(path, `https://us%22er:pw%22@example.com/p%22ath`))

	var backend *hclsyntax.Body
	for _, b := range parseBody(t, path).Blocks {
		if b.Type != "terraform" {
			continue
		}
		backend = labelledBlock(t, b.Body, "backend", "http")
	}
	require.NotNil(t, backend, `backend "http" block missing`)

	got := stringAttrs(t, backend)
	assert.Equal(t, `https://example.com/p"ath/state`, got["address"])
	assert.Equal(t, `https://example.com/p"ath/lock`, got["lock_address"])
	assert.Equal(t, `https://example.com/p"ath/lock`, got["unlock_address"])
	assert.Equal(t, `us"er`, got["username"])
	assert.Equal(t, `pw"`, got["password"])
}

func TestTerraformWriteTemplateRoundTripsHostileValues(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"quotes-and-newline", "echo \"MARKER\"\necho \"MARKER\" >> /var/log/qa", "echo \"MARKER\"\necho \"MARKER\" >> /var/log/qa"},
		{"windows-path", `C:\Users\rack`, `C:\Users\rack`},
		{"trailing-backslash", `end\`, `end\`},
		{"literal-backslash-n", `line1\nline2`, `line1\nline2`},
		{"interpolation", `region is ${aws_region}`, `region is ${aws_region}`},
		{"directive", `%{if x}a%{endif}`, `%{if x}a%{endif}`},
		{"bare-dollar", `costs $5`, `costs $5`},
		{"bare-percent", `100% done`, `100% done`},
		{"trailing-dollar", `pw ends in $`, `pw ends in $`},
		{"escaped-interpolation", `$${aws_region}`, `$${aws_region}`},
		{"astral-nonprintable", "a\U0001d173b", "a\U0001d173b"},
		{"tab", "a\tb", "a\tb"},
		{"carriage-return", "a\rb", "a\rb"},
		{"non-ascii", "caf\u00e9 \U0001f389", "caf\u00e9 \U0001f389"},
		{"non-breaking-space", "a\u00a0b", "a\u00a0b"},
		{"empty", "", ""},
		{"invalid-utf8", "bad\x80byte", "bad\ufffdbyte"},
	}

	t.Setenv("CONVOX_TERRAFORM_SOURCE", "")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vars := map[string]string{"name": "my-rack", "release": "3.25.7", "user_data": tc.in}
			path, _ := renderTemplate(t, vars, "aws")

			module := stringAttrs(t, labelledBlock(t, parseBody(t, path), "module", "system"))
			require.Contains(t, module, "user_data")
			assert.Equal(t, tc.want, module["user_data"])
		})
	}
}
