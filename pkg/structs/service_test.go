package structs

import (
	"encoding/json"
	"reflect"
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

func TestServiceJsonRoundTripOldRackShape(t *testing.T) {
	raw := `{"name":"svc","count":3,"cpu":100,"memory":256,"gpu":0,"gpu-vendor":"","domain":"","nlb":null,"ports":null}`
	var got Service
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, "svc", got.Name)
	require.Equal(t, 3, got.Count)
	require.Nil(t, got.Min)
	require.Nil(t, got.Max)
	require.Nil(t, got.ColdStart)
	require.Nil(t, got.Autoscale)
}

func TestServiceJsonRoundTripFullShape(t *testing.T) {
	mn, mx := 0, 10
	cold := true
	c, q := 70, 5
	want := Service{
		Name:      "vllm",
		Count:     0,
		Min:       &mn,
		Max:       &mx,
		ColdStart: &cold,
		Autoscale: &ServiceAutoscaleState{
			Enabled:        true,
			CpuThreshold:   &c,
			QueueThreshold: &q,
			MetricName:     "vllm:num_requests_waiting",
			CustomTriggers: 2,
		},
	}
	data, err := json.Marshal(want)
	require.NoError(t, err)

	var got Service
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, want, got)
}

func TestServiceUpdateOptionsTags(t *testing.T) {
	rt := reflect.TypeOf(ServiceUpdateOptions{})
	for _, field := range []string{"Min", "Max"} {
		f, ok := rt.FieldByName(field)
		require.True(t, ok, "field %s missing", field)
		want := map[string]string{"Min": "min", "Max": "max"}[field]
		require.Equal(t, want, f.Tag.Get("flag"))
		require.Equal(t, want, f.Tag.Get("param"))
	}
}

func TestServiceAutoscaleStateOmitempty(t *testing.T) {
	s := Service{Name: "svc", Count: 1}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.NotContains(t, string(data), "autoscale")
	require.NotContains(t, string(data), "cold-start")
	require.NotContains(t, string(data), `"min"`)
	require.NotContains(t, string(data), `"max"`)
}
