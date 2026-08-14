package sdk

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
)

const (
	sortableTime       = "20060102.150405.000000000"
	statusCodePrefix   = "F1E49A85-0AD7-4AEF-A618-C249C6E6568D:"
	statusCodeMaxLen   = len(statusCodePrefix) + 16
	ecsExecSessionByte = '\x00'
)

var (
	Version = "dev"

	// ErrExecIncomplete reports a stream that ended without the exit-code marker.
	ErrExecIncomplete = errors.New("the rack did not report an exit status for this command")
)

type Client struct {
	*stdsdk.Client
	Debug        bool
	Rack         string
	MachineID    string
	Session      SessionFunc
	ExtraHeaders map[string]string
}

type SessionFunc func(c *Client) string

// ensure interface parity
var _ structs.Provider = &Client{}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func New(endpoint string) (*Client, error) {
	s, err := stdsdk.New(coalesce(endpoint, "https://rack.convox"))
	if err != nil {
		return nil, err
	}

	c := &Client{
		Client: s,
		Debug:  os.Getenv("CONVOX_DEBUG") == "true",
	}

	// Auto-populate the audit-actor header from the CONVOX_ACTOR env
	// var when set. Used by the workflow worker (which spawns convox
	// CLI subprocesses with CONVOX_ACTOR=<user-email>) and the build
	// pod (which is launched with the same env injected by the rack
	// provider). Pre-3.24.6 racks ignore the header and fall back to
	// the existing rack-password behavior — back-compat clean.
	if a := strings.TrimSpace(os.Getenv("CONVOX_ACTOR")); a != "" {
		if c.ExtraHeaders == nil {
			c.ExtraHeaders = map[string]string{}
		}
		c.ExtraHeaders["X-Convox-Actor"] = a
	}

	c.Client.Headers = c.Headers

	return c, nil
}

func NewFromEnv() (*Client, error) {
	return New(os.Getenv("RACK_URL"))
}

func (c *Client) Endpoint() (*url.URL, error) {
	return c.Client.Endpoint, nil
}

func (c *Client) Headers() http.Header {
	h := http.Header{}

	for k, v := range c.ExtraHeaders {
		h.Set(k, v)
	}

	h.Set("User-Agent", fmt.Sprintf("convox.go/%s", Version))
	h.Set("Version", Version)

	if c.Client.Endpoint.User != nil {
		h.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(c.Client.Endpoint.User.String()))))
	}

	if c.Rack != "" {
		h.Set("Rack", c.Rack)
	}

	if c.Session != nil {
		h.Set("Session", c.Session(c))
	}

	return h
}

func (c *Client) Get(path string, opts stdsdk.RequestOptions, out interface{}) error {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.Get(path, opts, out)
}

func (c *Client) Post(path string, opts stdsdk.RequestOptions, out interface{}) error {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.Post(path, opts, out)
}

func (c *Client) Put(path string, opts stdsdk.RequestOptions, out interface{}) error {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.Put(path, opts, out)
}

func (c *Client) Delete(path string, opts stdsdk.RequestOptions, out interface{}) error {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.Delete(path, opts, out)
}

func (c *Client) Head(path string, opts stdsdk.RequestOptions, out *bool) error {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.Head(path, opts, out)
}

func (c *Client) PostStream(path string, opts stdsdk.RequestOptions) (*http.Response, error) {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.PostStream(path, opts)
}

func (c *Client) PutStream(path string, opts stdsdk.RequestOptions) (*http.Response, error) {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.PutStream(path, opts)
}

func (c *Client) GetStream(path string, opts stdsdk.RequestOptions) (*http.Response, error) {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
	}

	return c.Client.GetStream(path, opts)
}

func (c *Client) Websocket(path string, opts stdsdk.RequestOptions) (io.ReadCloser, error) {
	if strings.HasPrefix(path, "/app") {
		path = c.AddPrefixIfCloud(path)
		// trigger session authentication
		c.Get("/machines", stdsdk.RequestOptions{}, nil)
	} else {
		// trigger session authentication
		c.Get("/racks", stdsdk.RequestOptions{}, nil)
	}

	return c.Client.Websocket(path, opts)
}

func (c *Client) WebsocketExit(path string, ro stdsdk.RequestOptions, rw io.ReadWriter) (int, error) {
	ws, err := c.Websocket(path, ro)
	if err != nil {
		return 0, err
	}

	return websocketExitStream(ws, rw)
}

// websocketExitStream still reports a markerless end as exit 0, unlike
// execStream: the endpoints behind it are interactive shells, not deploy gates.
func websocketExitStream(ws io.Reader, rw io.ReadWriter) (int, error) {
	buf := make([]byte, 10*1024)
	code := 0

	for {
		n, err := ws.Read(buf)
		if err == io.EOF {
			return code, nil
		}
		if err != nil {
			return code, err
		}

		if i := strings.Index(string(buf[0:n]), statusCodePrefix); i > -1 {
			if _, err := rw.Write(buf[0:i]); err != nil {
				return 0, err
			}

			m := i + len(statusCodePrefix)

			code, err = strconv.Atoi(strings.TrimSpace(string(buf[m:n])))
			if err != nil {
				return 0, fmt.Errorf("unable to read exit code")
			}

			continue
		}

		if _, err := rw.Write(buf[0:n]); err != nil {
			return 0, err
		}
	}
}

type ecsExecSession struct {
	SessionID  string `json:"sessionId"`
	StreamURL  string `json:"streamUrl"`
	TokenValue string `json:"tokenValue"`
	Region     string `json:"region"`
}

var runSessionManagerPlugin = func(session ecsExecSession) (int, error) {
	pluginPath, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return -1, fmt.Errorf("session-manager-plugin not found in PATH. Install it: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}

	sessionJSON, err := json.Marshal(map[string]string{
		"SessionId":  session.SessionID,
		"StreamUrl":  session.StreamURL,
		"TokenValue": session.TokenValue,
	})
	if err != nil {
		return -1, err
	}

	targetJSON, err := json.Marshal(map[string]string{
		"Target": session.SessionID,
	})
	if err != nil {
		return -1, err
	}

	endpoint := fmt.Sprintf("https://ssm.%s.amazonaws.com", session.Region)

	cmd := exec.Command(pluginPath, string(sessionJSON), session.Region, "StartSession", "", string(targetJSON), endpoint)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}

	return 0, nil
}

// execStream relays an exec websocket. A v2 rack with ECSExec enabled signals an
// ECS Exec session by prefixing the FIRST payload with ecsExecSessionByte and a
// JSON session blob (handed to session-manager-plugin); any other rack streams
// the process output and the exit-code marker, which we strip. The marker can
// straddle a read, so the scan window carries the tail of what was already
// written and the code accumulates until its newline.
func execStream(ws io.Reader, rw io.ReadWriter) (int, error) {
	buf := make([]byte, 10*1024)
	code := 0
	sawExit := false
	first := true
	var ecsSessionData []byte
	var tail, marker []byte

	for {
		n, err := ws.Read(buf)
		if err == io.EOF {
			if ecsSessionData != nil {
				return -1, fmt.Errorf("ECS Exec session ended before it was established; please retry")
			}
			if c, ok := statusCodeFromMarker(marker); ok {
				return c, nil
			}
			if !sawExit {
				return code, ErrExecIncomplete
			}
			return code, nil
		}
		if err != nil {
			return code, err
		}

		if ecsSessionData != nil {
			ecsSessionData = append(ecsSessionData, buf[0:n]...)
			var session ecsExecSession
			if err := json.Unmarshal(ecsSessionData, &session); err != nil {
				continue
			}
			return runSessionManagerPlugin(session)
		}

		if first {
			first = false
			if n > 0 && buf[0] == ecsExecSessionByte {
				ecsSessionData = append([]byte{}, buf[1:n]...)
				var session ecsExecSession
				if err := json.Unmarshal(ecsSessionData, &session); err != nil {
					continue
				}
				return runSessionManagerPlugin(session)
			}
		}

		chunk := buf[0:n]

		for len(chunk) > 0 {
			if marker != nil {
				seek := chunk
				if room := statusCodeMaxLen - len(marker); len(seek) > room {
					seek = seek[0:room]
				}

				if j := bytes.IndexByte(seek, '\n'); j > -1 {
					marker = append(marker, chunk[0:j]...)
					chunk = chunk[j+1:]

					if c, ok := statusCodeFromMarker(marker); ok {
						code = c
						sawExit = true
					} else if _, err := rw.Write(append(marker, '\n')); err != nil {
						return 0, err
					}

					marker = nil
					tail = nil

					continue
				}

				marker = append(marker, seek...)
				chunk = chunk[len(seek):]

				if len(marker) >= statusCodeMaxLen {
					if _, err := rw.Write(marker); err != nil {
						return 0, err
					}
					tail = appendTail(tail, marker)
					marker = nil
				}

				continue
			}

			win := append(append([]byte(nil), tail...), chunk...)

			i := bytes.Index(win, []byte(statusCodePrefix))
			if i < 0 {
				if _, err := rw.Write(chunk); err != nil {
					return 0, err
				}
				tail = appendTail(tail, chunk)

				break
			}

			if i > len(tail) {
				if _, err := rw.Write(chunk[0 : i-len(tail)]); err != nil {
					return 0, err
				}
			}

			marker = append([]byte(nil), statusCodePrefix...)
			chunk = chunk[i+len(statusCodePrefix)-len(tail):]
			tail = nil
		}
	}
}

func appendTail(tail, b []byte) []byte {
	keep := len(statusCodePrefix) - 1

	if len(b) >= keep {
		return append(tail[:0], b[len(b)-keep:]...)
	}

	tail = append(tail, b...)

	if len(tail) > keep {
		tail = append(tail[:0], tail[len(tail)-keep:]...)
	}

	return tail
}

func statusCodeFromMarker(marker []byte) (int, bool) {
	if len(marker) <= len(statusCodePrefix) {
		return 0, false
	}

	code, err := strconv.Atoi(strings.TrimSpace(string(marker[len(statusCodePrefix):])))
	if err != nil {
		return 0, false
	}

	return code, true
}

type execBody struct {
	r    io.Reader
	done chan struct{}
}

// newExecBody holds the stdin EOF until the exec is over. The empty binary frame
// stdsdk sends on that EOF reads as a closed stream at a Console-proxied rack,
// which then kills the running command.
func newExecBody(r io.Reader) *execBody {
	return &execBody{r: r, done: make(chan struct{})}
}

func (b *execBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)

	if err == io.EOF {
		if n > 0 {
			return n, nil
		}

		<-b.done
	}

	return n, err
}

func (b *execBody) close() {
	close(b.done)
}

func (c *Client) WithContext(ctx context.Context) structs.Provider {
	cc := *c
	cc.Client = cc.Client.WithContext(ctx)
	return &cc
}

func (c *Client) AddPrefixIfCloud(path string) string {
	if c.MachineID != "" {
		return fmt.Sprintf("/machines/%s%s", c.MachineID, path)
	}
	return path
}
