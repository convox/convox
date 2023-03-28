package structs

import "net/http"

type SsoProvider interface {
	ExchangeCode(r *http.Request, code string) SsoExchangeCode
	Name() string
	Opts() SsoProviderOptions
	RedirectPath() string
	VerifyToken(t string) error
}

type SsoProviderOptions struct {
	ClientID     string
	ClientSecret string
	Issuer       string
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

type SsoAuthOptions struct {
	UserID   string `json:"user_id"`
	Token    string `json:"token"`
	Sso      string `json:"sso"`
	Provider string `json:"provider"`
	Issuer   string `json:"issuer"`
}
