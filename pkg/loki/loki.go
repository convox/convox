package loki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	endpoint url.URL
}

type Entry struct {
	Timestamp time.Time
	Line      string
}

type Logs struct {
	Streams []Stream `json:"streams,omitempty"`
}

type Stream struct {
	Labels  map[string]string `json:"stream"`
	Entries []Entry           `json:"values"`
}

func New(endpoint string) (*Client, error) {
	e, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	c := Client{
		endpoint: *e,
	}

	return &c, nil
}

func (c *Client) Post(labels map[string]string, ts time.Time, message string) error {
	logs := Logs{
		Streams: []Stream{
			{
				Labels: labels,
				Entries: []Entry{
					{
						Timestamp: ts,
						Line:      message,
					},
				},
			},
		},
	}

	data, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	res, err := http.Post(c.url("/loki/api/v1/push").String(), "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}

type TailOptions struct {
	Start *time.Time
}

func (c *Client) Tail(query string, opts TailOptions) (io.ReadCloser, error) {
	qs := url.Values{}

	qs.Add("query", query)

	if opts.Start != nil {
		qs.Add("start", fmt.Sprint(opts.Start.UTC().UnixNano()))
	}

	u := c.url("/loki/api/v1/tail")

	u.RawQuery = qs.Encode()
	u.Scheme = "ws"

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	go streamWebsocket(w, ws)

	return r, nil
}

func (c *Client) url(path string) *url.URL {
	u := c.endpoint

	u.Path += path

	return &u
}

func (e *Entry) MarshalJSON() ([]byte, error) {
	l, err := json.Marshal(e.Line)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[\"%d\",%s]", e.Timestamp.UnixNano(), l)), nil
}

func (e *Entry) UnmarshalJSON(data []byte) error {
	var unmarshal []string

	err := json.Unmarshal(data, &unmarshal)
	if err != nil {
		return err
	}

	t, err := strconv.ParseInt(unmarshal[0], 10, 64)
	if err != nil {
		return err
	}

	e.Timestamp = time.Unix(0, t)
	e.Line = unmarshal[1]

	return nil
}

func streamWebsocket(w io.WriteCloser, ws *websocket.Conn) {
	defer w.Close()

	for {
		var logs Logs

		if err := ws.ReadJSON(&logs); err != nil {
			fmt.Fprintf(w, "ERROR: %s\n", err)
			return
		}

		for _, s := range logs.Streams {
			switch s.Labels["stream"] {
			case "stdout", "stderr":
				writeStreamCri(w, s)
			default:
				writeStreamNamed(w, s)
			}
		}
	}
}

func writeStreamCri(w io.Writer, s Stream) {
	for _, e := range s.Entries {
		parts := strings.SplitN(e.Line, " ", 4)

		if len(parts) < 4 {
			return
		}

		out := fmt.Sprintf("%s %s/%s/%s %s", e.Timestamp.Format(time.RFC3339), s.Labels["type"], s.Labels["name"], s.Labels["pod"], parts[3])

		fmt.Fprint(w, out)

		if parts[2] == "F" {
			fmt.Fprint(w, "\n")
		}
	}
}

func writeStreamNamed(w io.Writer, s Stream) {
	for _, e := range s.Entries {
		fmt.Fprintf(w, "%s %s %s\n", e.Timestamp.Format(time.RFC3339), s.Labels["stream"], e.Line)
	}
}
