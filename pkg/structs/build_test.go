package structs_test

import (
	"encoding/json"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildImportImageOptions_KebabParam_RoundTrip is the happy-path
// contract for the kebab rename. stdsdk.MarshalOptions reads the `param:`
// tag verbatim and emits matching form keys, which the rack's stdapi
// UnmarshalOptions then binds back. The wire format is the kebab form.
//
// This locks in the kebab convention shared by BuildCreateOptions
// (build-args, wildcard-domain, git-sha, no-cache) — see
// pkg/structs/build.go BuildCreateOptions for the precedent.
func TestBuildImportImageOptions_KebabParam_RoundTrip(t *testing.T) {
	opts := structs.BuildImportImageOptions{
		SrcCredsUser: options.String("u"),
		SrcCredsPass: options.String("p"),
	}

	ro, err := stdsdk.MarshalOptions(opts)
	require.NoError(t, err)

	// Kebab keys must appear; legacy snake keys must NOT.
	require.Contains(t, ro.Params, "src-creds-user")
	require.Contains(t, ro.Params, "src-creds-pass")
	require.NotContains(t, ro.Params, "src_creds_user")
	require.NotContains(t, ro.Params, "src_creds_pass")

	assert.Equal(t, "u", ro.Params["src-creds-user"])
	assert.Equal(t, "p", ro.Params["src-creds-pass"])
}

// TestBuildImportImageOptions_PasswordNotInJSON is the redaction guard for
// SrcCredsPass. The field carries a plaintext registry password; any
// accidental json.Marshal of BuildImportImageOptions through structured
// logs, telemetry, or audit payloads must NOT leak it. The `json:"-"` tag
// is the cheapest defense — the field never marshals out.
//
// SrcCredsUser still serializes (kebab form) — it's not a secret and other
// surfaces (CLI flag echo, build status messages) may legitimately echo it.
func TestBuildImportImageOptions_PasswordNotInJSON(t *testing.T) {
	opts := structs.BuildImportImageOptions{
		SrcCredsUser: options.String("u"),
		SrcCredsPass: options.String("supersecret"),
	}

	data, err := json.Marshal(opts)
	require.NoError(t, err)
	out := string(data)

	// Username is fine to surface.
	assert.Contains(t, out, `"src-creds-user":"u"`)

	// Password value must NOT appear anywhere (defense against accidental
	// log/telemetry leakage of the plaintext credential).
	assert.NotContains(t, out, "supersecret",
		"plaintext password leaked into JSON output — json:\"-\" should suppress it")

	// The key itself must also be omitted via json:"-".
	assert.NotContains(t, out, "src-creds-pass",
		"src-creds-pass key appeared in JSON output — json:\"-\" should suppress it")
	assert.NotContains(t, out, "src_creds_pass",
		"legacy snake key must not appear either")
}
