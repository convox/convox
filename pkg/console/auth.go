package console

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
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

		dres, err := io.ReadAll(ares.Body)
		if err != nil {
			return nil, err
		}

		c.handler.Writef("Waiting for security token...")

		if os.Getenv("CONVOX_WEB_U2F_DISABLE") != "true" {
			browserToken := base64.StdEncoding.EncodeToString(dres)
			endpoint, err := c.Endpoint()
			if err != nil {
				return nil, AuthenticationError{err}
			}

			target := url.URL{
				Scheme: endpoint.Scheme,
				Host:   endpoint.Host,
				Path:   "/login/u2f",
			}

			// listen to callback url
			dataChan := make(chan []byte)
			tc := time.NewTicker(5 * time.Minute)
			addr, srv, err := c.u2fCallbackServer(dataChan)
			if err != nil {
				return nil, AuthenticationError{err}
			}

			qParam := target.Query()
			qParam.Add("token", browserToken)
			qParam.Add("callback_url", addr)
			target.RawQuery = qParam.Encode()
			c.handler.Writef("\nOpen this link in your browser to complete the authentication:\n%s \n", target.String())

			common.OpenBrowser(target.String())

			defer srv.Close()
			select {
			case <-tc.C:
				return nil, AuthenticationError{fmt.Errorf("callback server timeout")}
			case body = <-dataChan:
				c.handler.Writef("<ok>Done</ok>\n")
			}
		} else {

			//usb yubikey
			data, err := token.Authenticate(dres)
			if err != nil {
				return nil, AuthenticationError{err}
			}
			body = data
			c.handler.Writef("<ok>Done</ok>\n")
		}

		headers["Challenge"] = ares.Header.Get("Challenge")
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

func (c *Client) u2fCallbackServer(dataChan chan []byte) (string, *http.Server, error) {
	// Listen on any available port with ":0"
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", nil, fmt.Errorf("failed to listen on port: %v", err)
	}

	addr := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	srv := &http.Server{}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := r.URL.Query().Get("data")
		if data == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("data is missing"))
			return
		}

		dataBytes, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("malformed data: %s", err)))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("you may now close the window"))
		dataChan <- dataBytes
	})

	go func() {
		if err := srv.Serve(listener); err != nil && !strings.Contains(err.Error(), "http: Server closed") {
			log.Fatalf("failed to start u2f callback server: %s", err)
		}
	}()

	return addr, srv, nil
}
