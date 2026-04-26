package manifest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestServiceImagePullSecretValidate(t *testing.T) {
	cases := []struct {
		name    string
		sec     manifest.ServiceImagePullSecret
		wantErr string
	}{
		{
			name:    "ok passwordEnv",
			sec:     manifest.ServiceImagePullSecret{Registry: "nvcr.io", Username: "$oauthtoken", PasswordEnv: "NGC_API_KEY"},
			wantErr: "",
		},
		{
			name:    "ok literal password",
			sec:     manifest.ServiceImagePullSecret{Registry: "quay.io", Username: "robot", Password: "s3cret"},
			wantErr: "",
		},
		{
			name:    "ok registry with port",
			sec:     manifest.ServiceImagePullSecret{Registry: "harbor.corp:5000", Username: "u", Password: "p"},
			wantErr: "",
		},
		{
			name:    "both password and passwordEnv",
			sec:     manifest.ServiceImagePullSecret{Registry: "nvcr.io", Username: "u", Password: "p", PasswordEnv: "E"},
			wantErr: "use passwordEnv to reference an env var, OR password for a literal value, not both",
		},
		{
			name:    "neither password nor passwordEnv",
			sec:     manifest.ServiceImagePullSecret{Registry: "nvcr.io", Username: "u"},
			wantErr: "password or passwordEnv is required",
		},
		{
			name:    "uppercase registry rejected",
			sec:     manifest.ServiceImagePullSecret{Registry: "NVCR.IO", Username: "u", Password: "p"},
			wantErr: "must be lowercase",
		},
		{
			name:    "http scheme rejected",
			sec:     manifest.ServiceImagePullSecret{Registry: "http://nvcr.io", Username: "u", Password: "p"},
			wantErr: "must not include a scheme",
		},
		{
			name:    "https scheme rejected",
			sec:     manifest.ServiceImagePullSecret{Registry: "https://nvcr.io", Username: "u", Password: "p"},
			wantErr: "must not include a scheme",
		},
		{
			name:    "path rejected",
			sec:     manifest.ServiceImagePullSecret{Registry: "nvcr.io/nim", Username: "u", Password: "p"},
			wantErr: "must not include a path",
		},
		{
			name:    "invalid dns label",
			sec:     manifest.ServiceImagePullSecret{Registry: "bad..registry", Username: "u", Password: "p"},
			wantErr: "is not a valid registry hostname",
		},
		{
			name:    "empty username",
			sec:     manifest.ServiceImagePullSecret{Registry: "nvcr.io", Password: "p"},
			wantErr: "username is required",
		},
		{
			name:    "empty registry",
			sec:     manifest.ServiceImagePullSecret{Username: "u", Password: "p"},
			wantErr: "registry is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sec.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestServiceImagePullSecretsDuplicateRegistryRejected(t *testing.T) {
	// Two imagePullSecrets for the SAME registry on the SAME service would
	// produce identical Secret names and the second silently overwrites the
	// first during kubectl apply. Validation must reject the ambiguity at
	// convox.yml parse time.
	y := []byte(`services:
  nim:
    image: nvcr.io/foo:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: a
        password: p1
      - registry: nvcr.io
        username: b
        password: p2
`)
	m, err := manifest.Load(y, map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
	require.Contains(t, err.Error(), "nvcr.io")
}

func TestServiceImagePullSecretsYAMLRoundTripNonSecretFields(t *testing.T) {
	// Round-tripping a Service through yaml.Marshal → yaml.Unmarshal MUST
	// preserve registry/username/passwordEnv. Password is an egress-guard
	// exception — see TestServiceImagePullSecretPasswordDroppedOnYAMLMarshal.
	in := manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{
				Registry:    "nvcr.io",
				Username:    "$oauthtoken",
				PasswordEnv: "NGC_API_KEY",
			},
			{
				Registry: "quay.io",
				Username: "robot",
			},
		},
	}

	data, err := yaml.Marshal(&in)
	require.NoError(t, err)

	var out manifest.Service
	require.NoError(t, yaml.Unmarshal(data, &out))

	require.Len(t, out.ImagePullSecrets, 2)
	require.Equal(t, "nvcr.io", out.ImagePullSecrets[0].Registry)
	require.Equal(t, "$oauthtoken", out.ImagePullSecrets[0].Username)
	require.Equal(t, "NGC_API_KEY", out.ImagePullSecrets[0].PasswordEnv)
	require.Equal(t, "quay.io", out.ImagePullSecrets[1].Registry)
	require.Equal(t, "robot", out.ImagePullSecrets[1].Username)
}

// TestServiceImagePullSecretPasswordDroppedOnYAMLMarshal locks in the
// defensive egress guard: a Service containing a literal Password MUST NOT
// re-serialize that password when yaml.Marshal is called on the struct.
// The user's raw convox.yml bytes are preserved separately as Release.Manifest;
// this test protects any in-process code path that chooses to marshal the
// parsed struct (tests, debug prints, future tooling) from leaking creds.
func TestServiceImagePullSecretPasswordDroppedOnYAMLMarshal(t *testing.T) {
	const sentinel = "SENTINEL_LEAK_VIA_YAML_MARSHAL"

	in := manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "u", Password: sentinel},
		},
	}

	data, err := yaml.Marshal(&in)
	require.NoError(t, err)

	require.NotContains(t, string(data), sentinel, "yaml.Marshal of Service leaked literal password:\n%s", string(data))
	require.NotContains(t, string(data), "password:", "yaml.Marshal of Service emitted password key:\n%s", string(data))
}

// TestServiceImagePullSecretLiteralPasswordParsesFromYAML confirms that the
// defensive MarshalYAML does not break the parse path — users writing
// `password: literal` in convox.yml still populate the field correctly.
func TestServiceImagePullSecretLiteralPasswordParsesFromYAML(t *testing.T) {
	y := []byte(`services:
  nim:
    image: nvcr.io/foo:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: u
        password: parsed-literal
`)
	m, err := manifest.Load(y, map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())

	svc, err := m.Service("nim")
	require.NoError(t, err)
	require.Len(t, svc.ImagePullSecrets, 1)
	require.Equal(t, "parsed-literal", svc.ImagePullSecrets[0].Password)
}

func TestServiceImagePullSecretsOmitempty(t *testing.T) {
	s := manifest.Service{Name: "web"}

	data, err := yaml.Marshal(&s)
	require.NoError(t, err)

	if strings.Contains(string(data), "imagePullSecrets") {
		t.Errorf("empty ImagePullSecrets should be omitted, got:\n%s", string(data))
	}
}

func TestServiceImagePullSecretPasswordNotInJSON(t *testing.T) {
	const sentinel = "SENTINEL_LEAK_DO_NOT_LOG"

	sec := manifest.ServiceImagePullSecret{
		Registry: "nvcr.io",
		Username: "u",
		Password: sentinel,
	}

	data, err := json.Marshal(&sec)
	require.NoError(t, err)

	if strings.Contains(string(data), sentinel) {
		t.Errorf("sentinel password leaked through json.Marshal:\n%s", string(data))
	}
	if strings.Contains(string(data), "password") {
		t.Errorf("password field leaked through json.Marshal:\n%s", string(data))
	}
}

// TestServicePasswordNotInJSONViaNestedMarshal exercises the realistic leak
// surface: json-encoding a manifest.Service value whose ImagePullSecrets
// slice contains a sentinel password. The `json:"-"` tag on the nested
// Password field must survive nested marshaling.
func TestServicePasswordNotInJSONViaNestedMarshal(t *testing.T) {
	const sentinel = "SENTINEL_LEAK_VIA_NESTED_SERVICE"

	s := manifest.Service{
		Name: "nim",
		ImagePullSecrets: []manifest.ServiceImagePullSecret{
			{Registry: "nvcr.io", Username: "u", Password: sentinel},
			{Registry: "quay.io", Username: "b", Password: sentinel + "_2"},
		},
	}

	data, err := json.Marshal(&s)
	require.NoError(t, err)

	if strings.Contains(string(data), sentinel) {
		t.Errorf("sentinel password leaked through nested json.Marshal of Service:\n%s", string(data))
	}
	if strings.Contains(string(data), "password") {
		t.Errorf("password key leaked through nested json.Marshal of Service:\n%s", string(data))
	}
}

// TestManifestLoadImagePullSecrets exercises the real customer flow:
// convox.yml BYTES → Manifest.Load() → field populated + Validate passes.
func TestManifestLoadImagePullSecrets(t *testing.T) {
	y := []byte(`services:
  nim:
    image: nvcr.io/foo:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: $oauthtoken
        passwordEnv: NGC_API_KEY
      - registry: quay.io
        username: robot
        password: literal
`)
	m, err := manifest.Load(y, map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())

	svc, err := m.Service("nim")
	require.NoError(t, err)
	require.Len(t, svc.ImagePullSecrets, 2)
	require.Equal(t, "nvcr.io", svc.ImagePullSecrets[0].Registry)
	require.Equal(t, "$oauthtoken", svc.ImagePullSecrets[0].Username)
	require.Equal(t, "NGC_API_KEY", svc.ImagePullSecrets[0].PasswordEnv)
	require.Equal(t, "", svc.ImagePullSecrets[0].Password)
	require.Equal(t, "quay.io", svc.ImagePullSecrets[1].Registry)
	require.Equal(t, "literal", svc.ImagePullSecrets[1].Password)
}

// TestManifestValidateImagePullSecretsHookSurfacesErrors confirms the
// validation hook in validateServices propagates per-entry errors with the
// service name and index prefix so users can find the offending entry.
func TestManifestValidateImagePullSecretsHookSurfacesErrors(t *testing.T) {
	y := []byte(`services:
  web:
    image: public/image:latest
  nim:
    image: nvcr.io/foo:latest
    imagePullSecrets:
      - registry: NVCR.IO
        username: u
        password: p
`)
	m, err := manifest.Load(y, map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "service nim imagePullSecrets[0]")
	require.Contains(t, err.Error(), "must be lowercase")
}

// TestManifestValidateImagePullSecretsMultipleServicesIsolated confirms that
// validation state (seen-registries, error accumulator) does not leak
// between services — each service's duplicate-registry set is independent.
func TestManifestValidateImagePullSecretsMultipleServicesIsolated(t *testing.T) {
	// Both services declare nvcr.io. Neither should trigger a duplicate
	// error because the duplicate check is PER service, not across services.
	y := []byte(`services:
  a:
    image: nvcr.io/foo:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: u
        password: p
  b:
    image: nvcr.io/bar:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: u
        password: p
`)
	m, err := manifest.Load(y, map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}

// TestServiceImagePullSecretRegistryEdgeCases locks in rejections for
// hostname-shaped-but-invalid values that the regex must catch.
func TestServiceImagePullSecretRegistryEdgeCases(t *testing.T) {
	bad := []string{
		"-example.com", // leading dash
		"example-.com", // trailing dash on label
		".example.com", // leading dot
		"example.com.", // trailing dot
		"foo_bar.io",   // underscore
		"a..b.io",      // consecutive dots
		"example com",  // space
	}
	for _, r := range bad {
		t.Run(r, func(t *testing.T) {
			sec := manifest.ServiceImagePullSecret{Registry: r, Username: "u", Password: "p"}
			require.Error(t, sec.Validate(), "expected %q to fail validation", r)
		})
	}

	good := []string{
		"nvcr.io",
		"quay.io",
		"ghcr.io",
		"gcr.io",
		"registry.example.com",
		"harbor.corp:5000",
		"127.0.0.1:5000",
		"localhost:5000",
	}
	for _, r := range good {
		t.Run(r, func(t *testing.T) {
			sec := manifest.ServiceImagePullSecret{Registry: r, Username: "u", Password: "p"}
			require.NoError(t, sec.Validate(), "expected %q to pass validation", r)
		})
	}
}
