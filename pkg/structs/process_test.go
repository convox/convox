package structs

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessGpuFieldsOmitEmpty verifies that nil pointers on the new GPU
// telemetry fields are stripped from JSON output by `omitempty`. This is the
// non-GPU-pod path (vast majority of pods) and the mixed-skew path where the
// rack hasn't populated readings.
func TestProcessGpuFieldsOmitEmpty(t *testing.T) {
	p := Process{Id: "abc", App: "foo", Name: "web"}
	b, err := json.Marshal(p)
	require.NoError(t, err)
	s := string(b)
	assert.NotContains(t, s, "gpu-util", "nil gpu-util must be omitted")
	assert.NotContains(t, s, "gpu-mem-used", "nil gpu-mem-used must be omitted")
	assert.NotContains(t, s, "gpu-mem-total", "nil gpu-mem-total must be omitted")
}

// TestProcessGpuFieldsZeroPointerNotStripped verifies pointer-to-zero is
// distinct from nil — a pointer to 0 must serialize. This is the BC firewall
// fix: Vue / Console3 must be able to disambiguate "no data" (nil → omitted →
// GraphQL null) from "real reading at idle" (pointer to 0 → "gpu-util":0).
func TestProcessGpuFieldsZeroPointerNotStripped(t *testing.T) {
	zeroF := 0.0
	var zeroI int64 = 0
	p := Process{Id: "abc", GpuUtil: &zeroF, GpuMemUsed: &zeroI, GpuMemTotal: &zeroI}
	b, err := json.Marshal(p)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"gpu-util":0`, "pointer-to-zero must serialize, not omitted")
	assert.Contains(t, s, `"gpu-mem-used":0`)
	assert.Contains(t, s, `"gpu-mem-total":0`)
}

// TestProcessGpuFieldsKebabCase verifies the JSON tag shape — kebab-case
// matches the 3.24.6 convention (gpu-vendor, cold-start, gpu-hours, etc.).
func TestProcessGpuFieldsKebabCase(t *testing.T) {
	util := 73.5
	var memUsed int64 = 1024 * 1024 * 1024
	var memTotal int64 = 80 * 1024 * 1024 * 1024
	p := Process{Id: "abc", GpuUtil: &util, GpuMemUsed: &memUsed, GpuMemTotal: &memTotal}
	b, err := json.Marshal(p)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"gpu-util":73.5`, "tag must be kebab-case")
	assert.Contains(t, s, `"gpu-mem-used":1073741824`)
	assert.Contains(t, s, `"gpu-mem-total":85899345920`)
}

// TestProcess_OldRackDecode_KeysAbsentMeansNil simulates a 3.24.5 rack
// returning Process JSON without the new GPU keys. The pointer-typed fields
// must decode as nil (NOT zero) so the resolver can return GraphQL null.
// Without pointer types, missing keys would decode as 0.0 / 0 (Go zero
// values) and the resolver could not distinguish "no data" from "idle GPU."
func TestProcess_OldRackDecode_KeysAbsentMeansNil(t *testing.T) {
	raw := []byte(`{"id":"abc","app":"foo","name":"web","gpu":1}`)
	var p Process
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Nil(t, p.GpuUtil, "missing key must decode as nil pointer")
	assert.Nil(t, p.GpuMemUsed)
	assert.Nil(t, p.GpuMemTotal)
	// Service field is value-typed (string); missing key decodes as empty
	// string — graceful default. See TestProcess_ServiceField_OmitEmpty for
	// the omitempty wire-shape side.
	assert.Equal(t, "", p.Service, "missing service key must decode as empty string")
}

// TestProcess_MarshalNaN_ReturnsError pins the documented behavior that
// encoding/json returns json.UnsupportedValueError when any *float64 field
// points to a non-finite value (e.g. NaN from a divide-by-zero in a future
// aggregation regression). A regression that silently substitutes 0.0 / null
// for NaN would otherwise pass undetected; this test catches the class.
func TestProcess_MarshalNaN_ReturnsError(t *testing.T) {
	nan := math.NaN()
	p := Process{
		Id:      "abc",
		App:     "foo",
		Name:    "web",
		GpuUtil: &nan,
	}
	_, err := json.Marshal(&p)
	require.Error(t, err)
	var ueErr *json.UnsupportedValueError
	assert.ErrorAs(t, err, &ueErr,
		"encoding/json must return UnsupportedValueError for non-finite float64; "+
			"any future change that silently substitutes 0.0 / null for NaN must "+
			"preserve this error path or this test must be re-justified.")
}

// TestProcess_ServiceField_OmitEmpty — empty Service (system pod, build pod
// with non-standard labelling) must be stripped from JSON via omitempty.
func TestProcess_ServiceField_OmitEmpty(t *testing.T) {
	p := Process{Id: "abc", App: "system"}
	b, err := json.Marshal(p)
	require.NoError(t, err)
	// Match the JSON-key shape, not the bare substring — the Process struct
	// has no other field whose name happens to contain "service" today, but
	// future additions could; "service":" pinpoints the JSON key boundary.
	assert.NotContains(t, string(b), `"service":`, "empty service must be omitted from JSON")
}

// TestProcess_ServiceField_Serializes — populated Service serializes with the
// lowercase JSON tag.
func TestProcess_ServiceField_Serializes(t *testing.T) {
	p := Process{Id: "abc", App: "myapp", Service: "web"}
	b, err := json.Marshal(p)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"service":"web"`)
}

// TestProcessGpuFields_RoundTrip — marshal a Process with all GPU fields
// populated via pointers, unmarshal into a new struct, assert pointer-deref
// equality.
func TestProcessGpuFields_RoundTrip(t *testing.T) {
	util := 42.0
	var memUsed int64 = 8 * 1024 * 1024 * 1024
	var memTotal int64 = 80 * 1024 * 1024 * 1024
	want := Process{
		Id:          "abc",
		App:         "myapp",
		Name:        "inference",
		Service:     "inference",
		Gpu:         1,
		GpuUtil:     &util,
		GpuMemUsed:  &memUsed,
		GpuMemTotal: &memTotal,
	}
	b, err := json.Marshal(want)
	require.NoError(t, err)
	var got Process
	require.NoError(t, json.Unmarshal(b, &got))
	require.NotNil(t, got.GpuUtil)
	require.NotNil(t, got.GpuMemUsed)
	require.NotNil(t, got.GpuMemTotal)
	assert.Equal(t, util, *got.GpuUtil)
	assert.Equal(t, memUsed, *got.GpuMemUsed)
	assert.Equal(t, memTotal, *got.GpuMemTotal)
	assert.Equal(t, "inference", got.Service)
}
