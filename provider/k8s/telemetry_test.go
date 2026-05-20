package k8s_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func sha256Hex(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// hmacSHA256Hex computes HMAC-SHA256(key, value) and returns the hex string.
// Used by tests to compute the expected hash output for the new salted hashParamValue.
func hmacSHA256Hex(key, value string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestRackParams(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		// ConfigMap holds stubs for redacted
		// keys; the sidecar Secret holds the real plaintext that the
		// Go consumer overlays before SHA-256 hashing.
		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"params1":             "test1",
				"params2":             "test2",
				"params3":             "test3",
				"params4":             "test4",
				"params5":             "test5",
				"rack_name":           "rack",
				"cidr":                "test6",
				"docker_hub_password": "",
				"private_eks_pass":    "",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		sec := &ac.Secret{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params-redacted",
			},
			Type: ac.SecretTypeOpaque,
			Data: map[string][]byte{
				"docker_hub_password": []byte("secret-pw"),
				"private_eks_pass":    []byte("eks-secret"),
			},
		}
		_, err = fc.CoreV1().Secrets(p.Namespace).Create(context.TODO(), sec, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, map[string]interface{}{
			"params1":             "test1",
			"params2":             "test2",
			"params3":             "test3",
			"params4":             "test4",
			"params5":             "test5",
			"cidr":                hmacSHA256Hex("uid1", "test6"),
			"docker_hub_password": hmacSHA256Hex("uid1", "secret-pw"),
			"private_eks_pass":    hmacSHA256Hex("uid1", "eks-secret"),
		}, params)
	})
}

func TestRackParamsMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		params := p.RackParams()
		require.Nil(t, params)
	})
}

// TestRackParamsRedactsCredentials locks the redaction guarantee for the two
// new credential-bearing rack params: heartbeat must emit only the SHA-256 hex
// fingerprint, never the plaintext.
func TestRackParamsRedactsCredentials(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		const dockerSecret = "mysecret123"
		const eksSecret = "ekspass456"

		// ConfigMap stubs the redacted keys.
		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": "",
				"private_eks_pass":    "",
				"region":              "us-west-2",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		sec := &ac.Secret{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params-redacted",
			},
			Type: ac.SecretTypeOpaque,
			Data: map[string][]byte{
				"docker_hub_password": []byte(dockerSecret),
				"private_eks_pass":    []byte(eksSecret),
			},
		}
		_, err = fc.CoreV1().Secrets(p.Namespace).Create(context.TODO(), sec, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()

		// Credential params hashed, not plaintext.
		require.Equal(t, hmacSHA256Hex("uid1", dockerSecret), params["docker_hub_password"])
		require.Equal(t, hmacSHA256Hex("uid1", eksSecret), params["private_eks_pass"])
		require.NotEqual(t, dockerSecret, params["docker_hub_password"])
		require.NotEqual(t, eksSecret, params["private_eks_pass"])

		// region is in skipParams; must not appear at all.
		_, regionPresent := params["region"]
		require.False(t, regionPresent, "region must be skipped, not emitted")

		// Defense-in-depth: scan ALL emitted values for either plaintext literal.
		for k, v := range params {
			vs, ok := v.(string)
			if !ok {
				continue
			}
			require.NotContains(t, vs, dockerSecret,
				"plaintext docker_hub_password leaked under key %q", k)
			require.NotContains(t, vs, eksSecret,
				"plaintext private_eks_pass leaked under key %q", k)
		}
	})
}

// TestRackParamsLeavesNonRedactedPlaintext guards against a future regression
// that introduces blanket redaction. Params not in redactedParams must remain
// plaintext in the heartbeat payload.
func TestRackParamsLeavesNonRedactedPlaintext(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"non_secret_param": "plainvalue",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		// Even when the test exercises a non-redacted key, the
		// canonical fixture co-creates an empty sidecar Secret so the
		// consumer's Get path is exercised.
		sec := &ac.Secret{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params-redacted",
			},
			Type: ac.SecretTypeOpaque,
			Data: map[string][]byte{},
		}
		_, err = fc.CoreV1().Secrets(p.Namespace).Create(context.TODO(), sec, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, "plainvalue", params["non_secret_param"],
			"non-redacted params must remain plaintext")
	})
}

// TestRackParamsEmptyCredentialNotEmitted locks the existing skip-default
// behavior so telemetry does not regress to emitting hashParamValue("") for an
// unset credential. The default-rack-params ConfigMap carries the same empty
// string for an absent credential, so the carve-out at the param-equals-default
// branch keeps it out of the heartbeat entirely.
func TestRackParamsEmptyCredentialNotEmitted(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": "",
			},
		}
		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		dcm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-default-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": "",
			},
		}
		_, err = fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), dcm, am.CreateOptions{})
		require.NoError(t, err)

		// The sidecar Secret exists with an empty value when the
		// credential is unset. After overlay the merged value still
		// equals the default empty string, so the existing
		// skip-default carve-out continues to drop it from telemetry.
		// Locks the unset-credential path.
		sec := &ac.Secret{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params-redacted",
			},
			Type: ac.SecretTypeOpaque,
			Data: map[string][]byte{
				"docker_hub_password": []byte(""),
			},
		}
		_, err = fc.CoreV1().Secrets(p.Namespace).Create(context.TODO(), sec, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		_, present := params["docker_hub_password"]
		require.False(t, present,
			"docker_hub_password equal to default must be skipped, not emitted as hash of empty string")
	})
}

// TestRackParamsSecretAbsent_FallsBackToConfigMap pins the pre-3.24.6
// graceful-degrade path: a 3.24.5 rack that has not yet applied the
// new rack/k8s module has no telemetry-rack-params-redacted Secret.
// The ConfigMap still carries plaintext credential values (not yet
// stubbed). RackParams() must not error and must hash the ConfigMap
// values directly. Without this fallback an upgrade would silently
// drop the credentials from the heartbeat between rack-Go-deploy and
// TF-apply.
func TestRackParamsSecretAbsent_FallsBackToConfigMap(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		const dockerSecret = "preD8-docker-pw" //nolint:gosec // test fixture, not a real credential
		const eksSecret = "preD8-eks-pass"     //nolint:gosec // test fixture, not a real credential

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": dockerSecret,
				"private_eks_pass":    eksSecret,
				"non_secret_param":    "ok",
			},
		}
		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		// Deliberately do NOT create the redacted-params Secret —
		// simulates a rack that predates the Secret-based redaction.

		params := p.RackParams()
		require.Equal(t, hmacSHA256Hex("uid1", dockerSecret), params["docker_hub_password"],
			"pre-D8 rack: ConfigMap plaintext value must be hashed when Secret is absent")
		require.Equal(t, hmacSHA256Hex("uid1", eksSecret), params["private_eks_pass"],
			"pre-D8 rack: ConfigMap plaintext value must be hashed when Secret is absent")
		require.Equal(t, "ok", params["non_secret_param"],
			"pre-D8 rack: non-redacted ConfigMap values must pass through plaintext")
	})
}

// TestRackParams_PreD8Compat_SecretAbsent pins the upgrade-window
// contract for the telemetry Secret-overlay graceful-fallback path:
// a rack deployed with rack-Go code that knows to look for the
// redacted-params Secret AND has had its ConfigMap re-stubbed to
// empty values for the redacted keys, but where the Secret resource
// itself has not yet been applied to the cluster (transient
// TF-mid-apply state). The Secret Get returns NotFound and the
// consumer falls through to the ConfigMap empty-stub values directly.
// Per the existing skip-default rule, an empty value equal to the
// default (also empty) is dropped from the heartbeat entirely —
// never hashed-as-empty-string and emitted (which would leak
// presence-without-value to the receiver and incorrectly imply the
// credential is set).
//
// Mirrors TestRackParamsSecretAbsent_FallsBackToConfigMap (which pins
// the pre-Secret ConfigMap-plaintext + Secret-absent combination); this
// test pins the OTHER end of the upgrade window — ConfigMap already has
// empty stubs but Secret has not yet landed.
func TestRackParams_PreD8Compat_SecretAbsent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		// ConfigMap in post-D8 shape: redacted keys stubbed empty.
		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": "",
				"private_eks_pass":    "",
				"non_secret_param":    "ok",
			},
		}
		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		// Default-rack-params ConfigMap with the same empty defaults so the
		// skip-default carve-out fires for the empty stubs (the production
		// telemetry-default-rack-params CM ships these as empty).
		dcm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-default-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": "",
				"private_eks_pass":    "",
			},
		}
		_, err = fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), dcm, am.CreateOptions{})
		require.NoError(t, err)

		// Deliberately do NOT create the redacted-params Secret —
		// Secret-absent transient state during D8 TF apply.

		// Must not panic on Secret Get NotFound; must skip empty stubs.
		params := p.RackParams()

		// Empty-stub redacted keys must NOT appear (skipped via
		// equal-to-default carve-out, NOT hashed-empty-string-emitted).
		_, dockerPresent := params["docker_hub_password"]
		_, eksPresent := params["private_eks_pass"]
		assert.False(t, dockerPresent,
			"docker_hub_password empty-stub + Secret absent must NOT emit hashed empty string")
		assert.False(t, eksPresent,
			"private_eks_pass empty-stub + Secret absent must NOT emit hashed empty string")
		// Non-redacted plaintext values must still pass through.
		assert.Equal(t, "ok", params["non_secret_param"],
			"non-redacted ConfigMap values must pass through plaintext")
	})
}

// TestRedactedParamsAlphabeticalOrder enforces alphabetical order on
// redactedParams so new entries have an unambiguous insertion point.
func TestRedactedParamsAlphabeticalOrder(t *testing.T) {
	parts := strings.Split(*k8s.RedactedParamsForTest, ",")
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	assert.Equal(t, sorted, parts,
		"redactedParams must be alphabetical for consistent insertion order")
}

// TestHashParamValue_SaltedByNamespaceUID pins the security invariant: two
// Providers backed by namespaces with DIFFERENT UIDs must produce DIFFERENT
// hashes for the same plaintext; a single Provider must produce the SAME hash
// for the same plaintext on repeated calls (deterministic-per-rack).
func TestHashParamValue_SaltedByNamespaceUID(t *testing.T) {
	// Build two independent fake clientsets with different namespace UIDs.
	fc1 := fake.NewSimpleClientset()
	fc2 := fake.NewSimpleClientset()

	ns1 := &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack-ns-a", UID: "uid-rack-a"}}
	ns2 := &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack-ns-b", UID: "uid-rack-b"}}

	_, err := fc1.CoreV1().Namespaces().Create(context.TODO(), ns1, am.CreateOptions{})
	require.NoError(t, err)
	_, err = fc2.CoreV1().Namespaces().Create(context.TODO(), ns2, am.CreateOptions{})
	require.NoError(t, err)

	p1 := &k8s.Provider{Cluster: fc1, Namespace: "rack-ns-a"}
	p2 := &k8s.Provider{Cluster: fc2, Namespace: "rack-ns-b"}

	const plaintext = "super-secret-credential"

	hash1a := k8s.HashParamValueForTest(p1, plaintext)
	hash1b := k8s.HashParamValueForTest(p1, plaintext) // repeat: must be same
	hash2 := k8s.HashParamValueForTest(p2, plaintext)

	// Same rack, same plaintext -> same hash (deterministic).
	assert.Equal(t, hash1a, hash1b, "same rack same plaintext must produce same hash")

	// Different rack -> different hash (salted by UID).
	assert.NotEqual(t, hash1a, hash2, "different rack UIDs must produce different hashes for same plaintext")

	// Neither hash must equal bare sha256 of the plaintext (no unsalted leakage).
	assert.NotEqual(t, sha256Hex(plaintext), hash1a, "output must not be bare sha256")
	assert.NotEqual(t, sha256Hex(plaintext), hash2, "output must not be bare sha256")
}
