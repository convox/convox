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

// ReleasePromoteWatchAnnotation is the namespace annotation key used by the
// rollout-watcher goroutine to persist per-promote watch state. The watcher
// goroutine re-emits a terminal `app:promote:<verb>` event after the rollout
// reaches a steady state (success / error / cancelled). The annotation
// allows api-pod restarts to recover in-flight watches via cold-start GC.
//
// New key in 3.24.6 — older racks neither write nor read it. Purely
// additive: a pre-3.24.6 client decoding a 3.24.6 namespace ignores the
// annotation, and a 3.24.6 client decoding a pre-3.24.6 namespace finds
// no annotation and proceeds normally.
const ReleasePromoteWatchAnnotation = "convox.com/release-promote-watch"

// ReleasePromoteWatchState is the JSON payload of the
// `convox.com/release-promote-watch` namespace annotation. Persisted on
// every promote so the rollout-watcher goroutine survives api-pod
// restart (cold-start GC at startup re-launches per-app watchers).
//
// SchemaVersion=1 ships with 3.24.6. Future field-additive changes bump
// to SchemaVersion=2; rolling-upgrade safety requires that old api-pods
// LOG-AND-SKIP any SchemaVersion != 1 (future >= 2 OR legacy 0) rather
// than delete — a delete during a 5-15min rolling-upgrade window would
// lose state for an in-flight watch the new api-pod owns. Only TRULY
// corrupt JSON (unmarshal error) is GC-deleted immediately, since
// there's no payload to attribute an event to.
type ReleasePromoteWatchState struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReleaseID     string `json:"releaseId"`
	// AtomVersion stores the release-id captured at promote time (sourced
	// from p.Atom.Status() which returns the release name from
	// ReleaseCache). Despite the field name and JSON tag, this is NOT an
	// Atom CR's spec.currentVersion — it is the release-id mirror used by
	// the watcher to detect supersession against the namespace annotation
	// `convox.com/app-release` (which the AtomController writes). Field
	// name and JSON tag are pinned for back-compat across rolling upgrades;
	// renaming would break log-and-skip on annotations written by an older
	// api-pod within the rolling-upgrade window.
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
