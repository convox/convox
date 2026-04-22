package structs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServiceNlbPortJSONRoundtrip verifies wire-level JSON tag correctness
// for the hardening fields (cross-zone, allow-cidr, preserve-client-IP).
// Guards against accidental tag typos (e.g., "crosszone" for "cross-zone")
// and omitempty misconfiguration.
func TestServiceNlbPortJSONRoundtrip(t *testing.T) {
	tr := true
	fl := false
	want := ServiceNlbPort{
		Port:             8443,
		ContainerPort:    8080,
		Protocol:         "tls",
		Scheme:           "public",
		Certificate:      "arn:aws:acm:us-east-1:123456789012:certificate/abc",
		CrossZone:        &tr,
		AllowCIDR:        []string{"10.0.0.0/24", "10.1.0.0/24"},
		PreserveClientIP: &fl,
	}

	data, err := json.Marshal(want)
	require.NoError(t, err)

	// Exact JSON tag names on the wire.
	require.Contains(t, string(data), `"cross-zone":true`)
	require.Contains(t, string(data), `"allow-cidr":["10.0.0.0/24","10.1.0.0/24"]`)
	require.Contains(t, string(data), `"preserve-client-ip":false`)

	var got ServiceNlbPort
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, want, got)

	// Zero-value struct must omit the three omitempty fields entirely.
	empty := ServiceNlbPort{Port: 8443, ContainerPort: 8080}
	emptyData, err := json.Marshal(empty)
	require.NoError(t, err)
	require.NotContains(t, string(emptyData), "cross-zone")
	require.NotContains(t, string(emptyData), "allow-cidr")
	require.NotContains(t, string(emptyData), "preserve-client-ip")
}

// TestServiceNlbPortJSONNullHandling verifies that a v2 rack emitting
// an explicit `null` for allow-cidr (rather than omitting the key) decodes
// to the same empty-slice semantics as an omitted key.
func TestServiceNlbPortJSONNullHandling(t *testing.T) {
	var got ServiceNlbPort
	raw := `{"port":8443,"container-port":8080,"protocol":"tcp","scheme":"public","certificate":"","allow-cidr":null}`
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Nil(t, got.AllowCIDR)
	require.Nil(t, got.CrossZone)
	require.Nil(t, got.PreserveClientIP)
}
