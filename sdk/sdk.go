package sdk

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
)

const (
	sortableTime     = "20060102.150405.000000000"
	statusCodePrefix = "F1E49A85-0AD7-4AEF-A618-C249C6E6568D:"
)

var (
	Version = "dev"
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
