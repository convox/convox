package console

import (
	"fmt"
	"net/http"

	"github.com/convox/convox/sdk"
)

type Client struct {
	*sdk.Client
	handler Handler
}

type Handler interface {
	SettingReadKey(string, string) (string, error)
	SettingWriteKey(string, string, string) error
	Writef(string, ...interface{}) error
}

func NewClient(endpoint string, rack string, handler Handler) (*Client, error) {
	s, err := sdk.New(endpoint)
	if err != nil {
		return nil, err
	}

	cl := &Client{
		Client:  s,
		handler: handler,
	}

	s.Authenticator = cl.authenticator
	s.Rack = rack
	s.Session = cl.session

	return cl, nil
}

func NewSsoClient(endpoint, rack string, params map[string]string, handler Handler) (*Client, error) {
	s, err := sdk.New(endpoint)
	if err != nil {
		return nil, err
	}

	cl := &Client{
		Client:  s,
		handler: handler,
	}

	s.Client.Headers = func() http.Header {
		var version = "dev"

		h := http.Header{}

		h.Set("User-Agent", fmt.Sprintf("convox.go/%s", version))
		h.Set("Version", version)
		h.Set("Issuer", params["issuer"])
		h.Set("SSO-Provider", params["provider"])
		h.Set("Authorization", fmt.Sprintf("Bearer %s", params["bearer_token"]))

		if rack != "" {
			h.Set("Rack", rack)
		}

		return h
	}

	return cl, nil
}
