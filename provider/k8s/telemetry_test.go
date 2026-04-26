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
				"docker_hub_password": "secret-pw",
				"private_eks_pass":    "eks-secret",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
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

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"docker_hub_password": dockerSecret,
				"private_eks_pass":    eksSecret,
				"region":              "us-west-2",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
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

		params := p.RackParams()
		_, present := params["docker_hub_password"]
		require.False(t, present,
			"docker_hub_password equal to default must be skipped, not emitted as hash of empty string")
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
