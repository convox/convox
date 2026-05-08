package structs

// ManifestService is the wire shape for a single service's manifest block,
// returned by AppManifestService. Mirrors the subset of manifest.Service
// fields Console3 needs for service-detail rendering: name, literal
// service-level environment overrides, replica bounds. Additive only —
// never remove or rename fields.
type ManifestService struct {
	Name        string                `json:"name"`
	Environment []string              `json:"environment,omitempty"`
	Scale       *ManifestServiceScale `json:"scale,omitempty"`
}

// ManifestServiceScale exposes the resolved replica-bound pair (min/max)
// as nullable pointers. The provider implementation synthesizes these
// from manifest.ServiceScale's two yaml forms (top-level `min:`/`max:`
// pointers OR legacy `count: N-M` ServiceScaleCount), and these values
// reflect the post-ApplyDefaults state — for a service with NO scale
// block, the manifest defaults populate Count={1,1} so Min/Max=1/1
// rather than nil. Nil at the field level only occurs if a future code
// path emits a ManifestService directly without going through manifest
// loading.
//
// ColdStart is NOT included — it's runtime state populated from
// Deployment annotations and exposed via the existing
// structs.Service.ColdStart returned by ServiceList/ServiceGet. The
// per-service detail page reads cold-start from there.
type ManifestServiceScale struct {
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}
