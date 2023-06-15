package sso

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sso/okta"
)

func Initialize(provider string, opts structs.SsoProviderOptions) (structs.SsoProvider, error) {
	switch provider {
	case "okta":
		return okta.Initialize(opts)
	default:
		return nil, fmt.Errorf("unknown sso provider: %s", provider)
	}
}
