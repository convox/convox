package structs

import "net/http"

type SsoProvider interface {
	RedirectPath() string
	ExchangeCode(code string, r *http.Request) SsoExchangeCode
	VerifyToken(t string) error
	Opts() SsoProviderOptions
}

type SsoProviderOptions struct {
	ClientID     string
	ClientSecret string
	Issuer       string
	RedirectURL  string
	Scope        string
	State        string
	Nonce        string
}

type SsoExchangeCode struct {
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
	Scope            string `json:"scope,omitempty"`
	IdToken          string `json:"id_token,omitempty"`
}
