package structs

type ManifestService struct {
	Name        string                `json:"name"`
	Environment []string              `json:"environment,omitempty"`
	Scale       *ManifestServiceScale `json:"scale,omitempty"`
}

type ManifestServiceScale struct {
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}
