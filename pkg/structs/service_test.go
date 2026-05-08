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

// TestServiceGpuFieldsOmitEmpty — nil pointers on the new GPU averaged
// telemetry fields are stripped from JSON output by omitempty (non-GPU
// services, mixed-skew with rack that has not populated readings).
func TestServiceGpuFieldsOmitEmpty(t *testing.T) {
	s := Service{Name: "svc", Count: 1}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.NotContains(t, string(data), "gpu-util", "nil gpu-util must be omitted")
	require.NotContains(t, string(data), "gpu-mem-used", "nil gpu-mem-used must be omitted")
	require.NotContains(t, string(data), "gpu-mem-total", "nil gpu-mem-total must be omitted")
}

// TestServiceGpuFieldsZeroPointerNotStripped — pointer-to-zero must
// serialize. BC firewall: distinguish "no data" (nil → omitted → null) from
// "real averaged reading at idle" (pointer to 0 → "gpu-util":0).
func TestServiceGpuFieldsZeroPointerNotStripped(t *testing.T) {
	zeroF := 0.0
	var zeroI int64 = 0
	s := Service{Name: "svc", GpuUtil: &zeroF, GpuMemUsed: &zeroI, GpuMemTotal: &zeroI}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	out := string(data)
	require.Contains(t, out, `"gpu-util":0`, "pointer-to-zero must serialize")
	require.Contains(t, out, `"gpu-mem-used":0`)
	require.Contains(t, out, `"gpu-mem-total":0`)
}

// TestServiceGpuFieldsKebabCase — JSON tag shape is kebab-case.
func TestServiceGpuFieldsKebabCase(t *testing.T) {
	util := 60.5
	var memUsed int64 = 32 * 1024 * 1024 * 1024
	var memTotal int64 = 80 * 1024 * 1024 * 1024
	s := Service{Name: "vllm", Gpu: 1, GpuUtil: &util, GpuMemUsed: &memUsed, GpuMemTotal: &memTotal}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	out := string(data)
	require.Contains(t, out, `"gpu-util":60.5`, "tag must be kebab-case")
	require.Contains(t, out, `"gpu-mem-used":34359738368`)
	require.Contains(t, out, `"gpu-mem-total":85899345920`)
}

// TestService_OldRackDecode_GpuFieldsAbsentMeansNil — a 3.24.5 rack
// returning Service JSON without the new GPU avg keys must decode them as
// nil pointers on the 3.24.6 client side, NOT as zero values.
func TestService_OldRackDecode_GpuFieldsAbsentMeansNil(t *testing.T) {
	raw := []byte(`{"name":"svc","count":3,"cpu":100,"memory":256,"gpu":1,"gpu-vendor":"nvidia"}`)
	var s Service
	require.NoError(t, json.Unmarshal(raw, &s))
	require.Nil(t, s.GpuUtil, "missing key must decode as nil pointer")
	require.Nil(t, s.GpuMemUsed)
	require.Nil(t, s.GpuMemTotal)
}

// TestServiceGpuFields_RoundTrip — marshal a Service with all GPU avg
// fields populated, unmarshal into a new struct, assert pointer-deref equality.
func TestServiceGpuFields_RoundTrip(t *testing.T) {
	util := 55.5
	var memUsed int64 = 16 * 1024 * 1024 * 1024
	var memTotal int64 = 80 * 1024 * 1024 * 1024
	want := Service{
		Name:        "vllm",
		Gpu:         1,
		GpuVendor:   "nvidia",
		GpuUtil:     &util,
		GpuMemUsed:  &memUsed,
		GpuMemTotal: &memTotal,
	}
	data, err := json.Marshal(want)
	require.NoError(t, err)
	var got Service
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotNil(t, got.GpuUtil)
	require.NotNil(t, got.GpuMemUsed)
	require.NotNil(t, got.GpuMemTotal)
	require.Equal(t, util, *got.GpuUtil)
	require.Equal(t, memUsed, *got.GpuMemUsed)
	require.Equal(t, memTotal, *got.GpuMemTotal)
}
