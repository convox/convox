package k8s

import (
	"bytes"
	"html"
	"html/template"
	"testing"

	"github.com/convox/convox/pkg/common"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func renderValue(t *testing.T, tmpl, value string) string {
	t.Helper()

	tt, err := template.New("t").Funcs(template.FuncMap{"yamlQuote": yamlQuote, "yamlScalar": yamlScalar, "safe": (&Provider{}).templateHelpers()["safe"]}).Parse(tmpl)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tt.Execute(&buf, value))

	out, err := common.FormatYAML(buf.Bytes())
	require.NoError(t, err)

	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &v))
	require.Len(t, v, 1, string(out))

	k, ok := v["k"].(string)
	require.True(t, ok, string(out))

	return k
}

func TestYamlScalar(t *testing.T) {
	tests := []struct {
		In  string
		Out string
	}{
		{"abc", "abc"},
		{"true", "true"},
		{"/data", "/data"},
		{"", ""},
		{"a b", `"a b"`},
		{" a b ", `"a b"`},
		{"@x", `"@x"`},
		{"gpu\n", "gpu"},
		{"true\n", "true"},
		{"/data\n\n", "/data"},
		{"a b\r\n", `"a b"`},
		{"a&b", `"a\x26b"`},
		{"x\n---\nkind: ClusterRole", `"x\n---\nkind: ClusterRole"`},
	}

	for _, test := range tests {
		require.Equal(t, test.Out, string(yamlScalar(test.In)), "%q", test.In)
	}
}

func TestYamlQuoteRoundTrip(t *testing.T) {
	for _, s := range []string{
		"plain",
		`a "quoted" value`,
		`C:\path\to`,
		"a&b&amp;c",
		"&#34;&#10;---&#10;kind: ClusterRole",
		"line1\n---\nline3",
		"tab\there",
		"nel\u0085ls\u2028ps\u2029",
		"héllo wörld",
		" padded ",
		"",
	} {
		require.Equal(t, s, renderValue(t, "k: {{ yamlQuote . }}\n", s), "%q", s)
	}
}

func TestSafeMatchesEntityDecoding(t *testing.T) {
	for _, s := range []string{"?a=1&para=2", "&amp;", "?a=1&region=us", "&copy;", "AT&T", "a&b", "x&#34;&#10;---&#10;kind: ClusterRole"} {
		require.Equal(t, html.UnescapeString(s), renderValue(t, "k: {{ safe . }}\n", s), "%q", s)
	}
}
