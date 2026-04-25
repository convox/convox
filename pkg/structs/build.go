package structs

import (
	"time"
)

type Build struct {
	Id          string `json:"id"`
	App         string `json:"app"`
	Description string `json:"description"`
	Entrypoint  string `json:"entrypoint"`
	GitSha      string `json:"git-sha"`
	Logs        string `json:"logs"`
	Manifest    string `json:"manifest"`
	Process     string `json:"process"`
	Release     string `json:"release"`
	Reason      string `json:"reason"`
	Repository  string `json:"repository"`
	Status      string `json:"status"`

	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended"`

	Tags map[string]string `json:"-"`
}

type Builds []Build

type BuildCreateOptions struct {
	BuildArgs      *[]string `flag:"build-args" param:"build-args"`
	Description    *string   `flag:"description,d" param:"description"`
	Development    *bool     `flag:"development" param:"development"`
	External       *bool     `flag:"external" param:"external"`
	Manifest       *string   `flag:"manifest,m" param:"manifest"`
	NoCache        *bool     `flag:"no-cache" param:"no-cache"`
	WildcardDomain *bool     `flag:"wildcard-domain" param:"wildcard-domain"`

	GitSha *string `param:"git-sha"`
}

type BuildImportImageOptions struct {
	SrcCredsUser *string `flag:"src-creds-user" param:"src_creds_user" json:"src_creds_user,omitempty"`
	SrcCredsPass *string `flag:"src-creds-pass" param:"src_creds_pass" json:"src_creds_pass,omitempty"`
}

type BuildListOptions struct {
	Limit *int `flag:"limit,l" query:"limit"`
}

type BuildUpdateOptions struct {
	Ended      *time.Time `param:"ended"`
	Entrypoint *string    `param:"entrypoint"`
	Logs       *string    `param:"logs"`
	Manifest   *string    `param:"manifest"`
	Release    *string    `param:"release"`
	Started    *time.Time `param:"started"`
	Status     *string    `param:"status"`
}

func NewBuild(app string) *Build {
	return &Build{
		App:    app,
		Id:     id("B", 10),
		Status: "created",
		Tags:   map[string]string{},
	}
}
