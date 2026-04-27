package k8s_test

import (
	"context"
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

func TestRackParams(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		// Decision 8 post-rc4 shape: ConfigMap holds stubs for redacted
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
			"cidr":                sha256Hex("test6"),
			"docker_hub_password": sha256Hex("secret-pw"),
			"private_eks_pass":    sha256Hex("eks-secret"),
		}, params)
	})
}

func TestRackParamsMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		params := p.RackParams()
		require.Nil(t, params)
	})
}

// TestRackParamsRedactsCredentials locks D.7's redaction guarantee for the two
// new credential-bearing rack params: heartbeat must emit only the SHA-256 hex
// fingerprint, never the plaintext.
func TestRackParamsRedactsCredentials(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)

		const dockerSecret = "mysecret123"
		const eksSecret = "ekspass456"

		// Decision 8 post-rc4 shape: ConfigMap stubs the redacted keys.
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
		require.Equal(t, sha256Hex(dockerSecret), params["docker_hub_password"])
		require.Equal(t, sha256Hex(eksSecret), params["private_eks_pass"])
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

		// Decision 8 post-rc4 shape: even when the test exercises a
		// non-redacted key, the canonical fixture co-creates an empty
		// sidecar Secret so the consumer's Get path is exercised.
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
// behavior so D.7 does not regress to emitting hashParamValue("") for an
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

		// Decision 8 post-rc4 shape: the sidecar Secret exists with an
		// empty value when the credential is unset. After overlay the
		// merged value still equals the default empty string, so the
		// existing skip-default carve-out continues to drop it from
		// telemetry. Locks the post-D8 unset-credential path.
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

// TestRackParamsSecretAbsent_FallsBackToConfigMap pins the pre-Decision-8
// graceful-degrade path: a 3.24.5 rack that has not yet applied the new
// rack/k8s module has no telemetry-rack-params-redacted Secret. The
// ConfigMap still carries plaintext credential values (not yet stubbed).
// RackParams() must not error and must hash the ConfigMap values directly.
// Without this fallback an upgrade would silently drop the credentials
// from the heartbeat between rack-Go-deploy and TF-apply.
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
		// pre-D8 rack shape.

		params := p.RackParams()
		require.Equal(t, sha256Hex(dockerSecret), params["docker_hub_password"],
			"pre-D8 rack: ConfigMap plaintext value must be hashed when Secret is absent")
		require.Equal(t, sha256Hex(eksSecret), params["private_eks_pass"],
			"pre-D8 rack: ConfigMap plaintext value must be hashed when Secret is absent")
		require.Equal(t, "ok", params["non_secret_param"],
			"pre-D8 rack: non-redacted ConfigMap values must pass through plaintext")
	})
}

// TestRedactedParamsAlphabeticalOrder enforces the alphabetical convention on
// redactedParams so chain α6 follow-ups (e.g. D.2 webhook_signing_key) have an
// unambiguous insertion target. Order is for review hygiene; correctness uses
// strings.Contains and is order-agnostic.
func TestRedactedParamsAlphabeticalOrder(t *testing.T) {
	parts := strings.Split(*k8s.RedactedParamsForTest, ",")
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	assert.Equal(t, sorted, parts,
		"redactedParams must be alphabetical for review hygiene + chain α6 ordering")
}
