package okta

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/convox/convox/pkg/structs"
	verifier "github.com/okta/okta-jwt-verifier-golang"
)

type Okta struct {
	opts structs.SsoProviderOptions
}

func Initialize(opts structs.SsoProviderOptions) (structs.SsoProvider, error) {
	return &Okta{opts}, nil
}

func (o *Okta) Opts() structs.SsoProviderOptions {
	return o.opts
}

func (o *Okta) RedirectPath() string {
	q := url.Values{}
	q.Add("client_id", o.opts.ClientID)
	q.Add("response_type", "code")
	q.Add("response_mode", "query")
	q.Add("scope", o.opts.Scope)
	q.Add("redirect_uri", o.opts.RedirectURL)
	q.Add("state", o.opts.State)
	q.Add("nonce", o.opts.Nonce)

	return o.opts.Issuer + "/v1/authorize?" + q.Encode()
}

func (o *Okta) ExchangeCode(code string, r *http.Request) structs.SsoExchangeCode {
	authHeader := base64.StdEncoding.EncodeToString(
		[]byte(o.opts.ClientID + ":" + o.opts.ClientSecret))

	q := r.URL.Query()
	q.Add("grant_type", "authorization_code")
	q.Set("code", code)
	q.Add("redirect_uri", o.opts.RedirectURL)

	url := o.opts.Issuer + "/v1/token?" + q.Encode()

	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte("")))
	h := req.Header
	h.Add("Authorization", "Basic "+authHeader)
	h.Add("Accept", "application/json")
	h.Add("Content-Type", "application/x-www-form-urlencoded")
	h.Add("Connection", "close")
	h.Add("Content-Length", "0")

	client := &http.Client{}
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	var exchange structs.SsoExchangeCode
	json.Unmarshal(body, &exchange)

	return exchange
}

func (o *Okta) VerifyToken(t string) error {
	tv := map[string]string{}
	tv["nonce"] = o.opts.Nonce
	tv["aud"] = o.opts.ClientID
	jv := verifier.JwtVerifier{
		Issuer:           o.opts.Issuer,
		ClaimsToValidate: tv,
	}

	result, err := jv.New().VerifyIdToken(t)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	if result != nil {
		return nil
	}

	return fmt.Errorf("token could not be verified: %s", "")
}
