package structs

import "time"

type Release struct {
	Id string `json:"id"`

	App         string `json:"app"`
	Build       string `json:"build"`
	Env         string `json:"env"`
	Manifest    string `json:"manifest"`
	Description string `json:"description"`

	Created time.Time `json:"created"`
}

type Releases []Release

type ReleaseCreateOptions struct {
	Build         *string `param:"build"`
	Description   *string `param:"description"`
	Env           *string `param:"env"`
	ParentRelease *string `param:"parent-release"`
}

type ReleaseCreateFromOptions struct {
	BuildFrom             *string `flag:"build-from"`
	EnvFrom               *string `flag:"env-from"`
	UseActiveReleaseBuild *bool   `flag:"use-active-release-build"`
	UseActiveReleaseEnv   *bool   `flag:"use-active-release-env"`
	Promote               *bool   `flag:"promote"`
}

type ReleaseListOptions struct {
	Limit *int `flag:"limit,l" query:"limit"`
}

type ReleasePromoteOptions struct {
	Development *bool `param:"development"`
	Force       *bool `param:"force"`
	Idle        *bool `param:"idle"`
	Min         *int  `param:"min"`
	Max         *int  `param:"max"`
	Timeout     *int  `param:"timeout"`
}

// ReleasePromoteWatchAnnotation is the namespace annotation key for rollout-watcher state persistence.
const ReleasePromoteWatchAnnotation = "convox.com/release-promote-watch"

// ReleasePromoteWatchState is the annotation payload for rollout-watcher cold-start recovery.
type ReleasePromoteWatchState struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReleaseID     string `json:"releaseId"`
	// AtomVersion is the release-id at promote time; name pinned for rolling-upgrade back-compat.
	AtomVersion string    `json:"atomVersion"`
	StartedAt   time.Time `json:"startedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Actor       string    `json:"actor"`
}

func NewRelease(app string) *Release {
	return &Release{
		App:     app,
		Created: time.Now().UTC(),
		Id:      id("R", 10),
	}
}

func (rs Releases) Less(i, j int) bool {
	return rs[i].Created.After(rs[j].Created)
}
