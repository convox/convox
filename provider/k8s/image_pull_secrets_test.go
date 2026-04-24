package k8s

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
)

func TestImagePullSecretName(t *testing.T) {
	t.Run("short registry", func(t *testing.T) {
		n := imagePullSecretName("myai", "nim", "nvcr.io")
		require.Equal(t, "convox-myai-nim-pull-nvcr-io", n)
	})

	t.Run("registry with port", func(t *testing.T) {
		n := imagePullSecretName("myai", "nim", "harbor.corp:5000")
		require.Equal(t, "convox-myai-nim-pull-harbor-corp-5000", n)
	})

	t.Run("stays under 253 chars", func(t *testing.T) {
		n := imagePullSecretName(
			strings.Repeat("a", 100),
			strings.Repeat("b", 100),
			"very-long.registry.example.com",
		)
		require.LessOrEqual(t, len(n), 253)
	})

	t.Run("truncated name never ends in dash before suffix", func(t *testing.T) {
		// Engineer a truncation that lands on a dash: app+service+slug get
		// chopped mid-token at a position where the last kept char is a `-`.
		// Before the trim fix, output was `...a--<hash>` (double dash), which
		// though DNS-1123-valid is anomalous and can confuse operator tooling.
		svc := strings.Repeat("s", 100)
		registry := strings.Repeat("a.", 70) + "b"
		n := imagePullSecretName("myai", svc, registry)
		require.LessOrEqual(t, len(n), 253)
		require.NotContains(t, n, "--", "truncated name contains consecutive dashes: %s", n)
	})
}

func TestBuildDockerConfigJSON(t *testing.T) {
	payload, err := buildDockerConfigJSON("nvcr.io", "$oauthtoken", "s3cret")
	require.NoError(t, err)

	var parsed struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(payload, &parsed))

	entry, ok := parsed.Auths["nvcr.io"]
	require.True(t, ok, "nvcr.io entry missing from auths")
	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	require.NoError(t, err)
	require.Equal(t, "$oauthtoken:s3cret", string(decoded))
}

func TestBuildDockerConfigJSONHandlesSpecialChars(t *testing.T) {
	// Password with quote, backslash, newline — json.Marshal must escape, not break.
	payload, err := buildDockerConfigJSON("r.io", `u"ser`, "p\\n\"pass")
	require.NoError(t, err)
	require.True(t, json.Valid(payload), "payload must be valid JSON")
}

func envMap(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestRenderImagePullSecretsEmpty(t *testing.T) {
	svc := &manifest.Service{Name: "nim"}

	blocks, names, err := renderImagePullSecrets("myai", "rack1-myai", svc, envMap(nil))
	require.NoError(t, err)
	require.Nil(t, blocks)
	require.Nil(t, names)
}

func TestRenderImagePullSecretsLiteralPassword(t *testing.T) {
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", Password: "nvapi-literal"},
		},
	}

	blocks, names, err := renderImagePullSecrets("myai", "rack1-myai", svc, envMap(nil))
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, []string{"convox-myai-nim-pull-nvcr-io"}, names)

	y := string(blocks[0])
	require.Contains(t, y, "kind: Secret")
	require.Contains(t, y, "type: kubernetes.io/dockerconfigjson")
	require.Contains(t, y, "name: convox-myai-nim-pull-nvcr-io")
	require.Contains(t, y, "namespace: rack1-myai")
}

func TestRenderImagePullSecretsPasswordEnv(t *testing.T) {
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", PasswordEnv: "NGC_API_KEY"},
		},
	}

	env := envMap(map[string]string{"NGC_API_KEY": "nvapi-envsourced"})

	blocks, names, err := renderImagePullSecrets("myai", "rack1-myai", svc, env)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Len(t, names, 1)

	// YAML data values are base64-encoded. The dockerconfigjson payload ITSELF
	// contains a base64-encoded user:pass auth field. Decode both layers and
	// verify the resolved env value reaches the innermost user:pass tuple.
	y := string(blocks[0])
	idx := strings.Index(y, ".dockerconfigjson:")
	require.Greater(t, idx, 0)
	rest := strings.TrimSpace(y[idx+len(".dockerconfigjson:"):])
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[:nl]
	}
	outer, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rest))
	require.NoError(t, err)

	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(outer, &cfg))

	entry, ok := cfg.Auths["nvcr.io"]
	require.True(t, ok)
	userpass, err := base64.StdEncoding.DecodeString(entry.Auth)
	require.NoError(t, err)
	require.Equal(t, "$oauthtoken:nvapi-envsourced", string(userpass))
}

func TestRenderImagePullSecretsMissingEnvErrors(t *testing.T) {
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", PasswordEnv: "NGC_API_KEY"},
		},
	}

	_, _, err := renderImagePullSecrets("myai", "rack1-myai", svc, envMap(nil))
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "NGC_API_KEY", "error must name the env var")
	require.Contains(t, msg, "nim", "error must name the service")
	require.Contains(t, msg, "imagePullSecrets[0]", "error must name the index")
	require.Contains(t, msg, "convox env set", "error must hint at the fix")
	require.Contains(t, msg, "-a myai", "error must bake the actual app name, not a <app> placeholder")
	require.NotContains(t, msg, "<app>", "error must substitute the app parameter, not leave a literal placeholder")
}

func TestRenderImagePullSecretsEmptyEnvStringErrors(t *testing.T) {
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", PasswordEnv: "NGC_API_KEY"},
		},
	}

	env := envMap(map[string]string{"NGC_API_KEY": ""})
	_, _, err := renderImagePullSecrets("myai", "rack1-myai", svc, env)
	require.Error(t, err)
	require.Contains(t, err.Error(), "NGC_API_KEY")
}

func TestRenderImagePullSecretsMultipleRegistries(t *testing.T) {
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", PasswordEnv: "NGC_API_KEY"},
			{Registry: "quay.io", Username: "robot", Password: "lit"},
		},
	}

	env := envMap(map[string]string{"NGC_API_KEY": "nvapi-aaa"})

	blocks, names, err := renderImagePullSecrets("myai", "rack1-myai", svc, env)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	require.Equal(t, []string{
		"convox-myai-nim-pull-nvcr-io",
		"convox-myai-nim-pull-quay-io",
	}, names)

	// Locked ordering: blocks[i] YAML must carry the Secret name listed at
	// names[i]. A regression that shuffled blocks vs names would still pass
	// a Len-only check but produce a Pod referencing Secrets that don't
	// exist in the cluster.
	require.Contains(t, string(blocks[0]), "name: "+names[0])
	require.NotContains(t, string(blocks[0]), "name: "+names[1])
	require.Contains(t, string(blocks[1]), "name: "+names[1])
	require.NotContains(t, string(blocks[1]), "name: "+names[0])
}

func TestImagePullSecretNamesMatchesRender(t *testing.T) {
	// The run-Pod path and timer path use imagePullSecretNames() (names only),
	// while releaseTemplateServices uses renderImagePullSecrets() (names + YAML).
	// Both must agree on the generated names or the Pod will reference a
	// Secret that doesn't exist.
	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "u", Password: "p"},
			{Registry: "quay.io", Username: "u", Password: "p"},
			{Registry: "harbor.corp:5000", Username: "u", Password: "p"},
		},
	}

	_, renderNames, err := renderImagePullSecrets("myai", "rack1-myai", svc, envMap(nil))
	require.NoError(t, err)

	namesOnly := imagePullSecretNames("myai", "nim", svc.ImagePullSecrets)
	require.Equal(t, renderNames, namesOnly)
}

func TestImagePullSecretNamesEmpty(t *testing.T) {
	require.Nil(t, imagePullSecretNames("myai", "nim", nil))
	require.Nil(t, imagePullSecretNames("myai", "nim", []manifest.ServiceImagePullSecret{}))
}

func TestRenderImagePullSecretsSentinelLeak(t *testing.T) {
	const sentinel = "SENTINEL_LEAK_DO_NOT_LOG_12345"

	svc := &manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "$oauthtoken", Password: sentinel},
		},
	}

	blocks, _, err := renderImagePullSecrets("myai", "rack1-myai", svc, envMap(nil))
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	// The sentinel password must never appear in plaintext — only inside the
	// base64-encoded dockerconfigjson blob. Grep the raw YAML for the
	// sentinel string; it must not be there.
	if strings.Contains(string(blocks[0]), sentinel) {
		t.Errorf("sentinel password leaked to YAML output:\n%s", string(blocks[0]))
	}
}
