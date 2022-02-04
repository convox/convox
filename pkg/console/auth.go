package console

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/convox/convox/pkg/token"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdsdk"
)

var (
	reSessionAuthentication = regexp.MustCompile(`^Session path="([^"]+)" token="([^"]+)"$`)
)

type AuthenticationError struct {
	error
}

func (ae AuthenticationError) AuthenticationError() error {
	return ae.error
}

type session struct {
	ID string `json:"id"`
}

func (c *Client) authenticator(cl *stdsdk.Client, res *http.Response) (http.Header, error) {
	m := reSessionAuthentication.FindStringSubmatch(res.Header.Get("WWW-Authenticate"))
	if len(m) < 3 {
		return nil, nil
	}

	body := []byte{}
	headers := map[string]string{}

	if m[2] == "true" {
		ares, err := cl.GetStream(m[1], stdsdk.RequestOptions{})
		if err != nil {
			return nil, err
		}
		defer ares.Body.Close()

		dres, err := ioutil.ReadAll(ares.Body)
		if err != nil {
			return nil, err
		}

		c.handler.Writef("Waiting for security token... ")

		data, err := token.Authenticate(dres)
		if err != nil {
			return nil, AuthenticationError{err}
		}

		c.handler.Writef("<ok>OK</ok>\n")

		body = data
		headers["Challenge"] = ares.Header.Get("Challenge")
		fmt.Println("Challenge header: ", headers["Challenge"])
	}

	var s session

	ro := stdsdk.RequestOptions{
		Body:    bytes.NewReader(body),
		Headers: stdsdk.Headers(headers),
	}

	if err := cl.Post(m[1], ro, &s); err != nil {
		return nil, AuthenticationError{err}
	}

	if s.ID == "" {
		return nil, fmt.Errorf("invalid session")
	}

	if err := c.handler.SettingWriteKey("session", cl.Endpoint.Host, s.ID); err != nil {
		return nil, err
	}

	h := http.Header{}

	h.Set("Session", s.ID)

	return h, nil
}

func (c *Client) session(cl *sdk.Client) string {
	sid, _ := c.handler.SettingReadKey("session", cl.Client.Endpoint.Host)
	return sid
}
