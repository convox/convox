package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientAddr(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		xff    []string
		real   string
		ip     string
		peer   string
	}{
		{"no header", "10.0.0.5:4000", nil, "", "10.0.0.5", "10.0.0.5"},
		{"one entry", "10.0.0.5:4000", []string{"203.0.113.9"}, "", "203.0.113.9", "10.0.0.5"},
		{"leftmost ignored", "10.0.0.5:4000", []string{"198.51.100.1, 203.0.113.9"}, "", "203.0.113.9", "10.0.0.5"},
		{"last header line", "10.0.0.5:4000", []string{"198.51.100.1", "203.0.113.9"}, "", "203.0.113.9", "10.0.0.5"},
		{"with port", "10.0.0.5:4000", []string{"203.0.113.9:4711"}, "", "203.0.113.9", "10.0.0.5"},
		{"ipv6", "10.0.0.5:4000", []string{"2001:db8::1"}, "", "2001:db8::1", "10.0.0.5"},
		{"ipv6 with port", "10.0.0.5:4000", []string{"[2001:db8::1]:443"}, "", "2001:db8::1", "10.0.0.5"},
		{"mapped ipv4", "10.0.0.5:4000", []string{"::ffff:203.0.113.9"}, "", "203.0.113.9", "10.0.0.5"},
		{"empty tail", "10.0.0.5:4000", []string{"203.0.113.9, "}, "", "10.0.0.5", "10.0.0.5"},
		{"garbage", "10.0.0.5:4000", []string{`1.2.3.4" ip="9.9.9.9`}, "", "10.0.0.5", "10.0.0.5"},
		{"x-real-ip ignored", "10.0.0.5:4000", nil, "203.0.113.9", "10.0.0.5", "10.0.0.5"},
		{"ipv6 peer", "[::1]:5000", nil, "", "::1", "::1"},
		{"ipv6 zone dropped", "10.0.0.5:4000", []string{"fe80::1%\u202e\u009b31m"}, "", "fe80::1", "10.0.0.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/apps", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if tc.real != "" {
				r.Header.Set("X-Real-IP", tc.real)
			}

			ip, peer := ClientAddr(r)
			require.Equal(t, tc.ip, ip)
			require.Equal(t, tc.peer, peer)
		})
	}
}

func TestActor(t *testing.T) {
	h := http.Header{}
	require.Equal(t, "rack-password", Actor(h))

	h.Set("Convox-Actor", "bob@example.com")
	require.Equal(t, "bob@example.com", Actor(h))

	h.Set("X-Convox-Actor", " alice@example.com ")
	require.Equal(t, "alice@example.com", Actor(h))

	h.Set("X-Convox-Actor", "\u200b")
	require.Equal(t, "bob@example.com", Actor(h))

	h.Set("X-Convox-Actor", "ali\u202ece")
	require.Equal(t, "alice", Actor(h))
}

func captureOutput(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := Output
	Output = &buf
	t.Cleanup(func() { Output = prev })

	return &buf
}

func TestWrite(t *testing.T) {
	buf := captureOutput(t)

	Write(&Entry{Type: "api", Op: "AppList", Method: "GET", Path: "/apps", Status: 200, Auth: "none", IP: "10.0.0.5", Peer: "10.0.0.5"})

	out := buf.String()
	require.True(t, strings.HasPrefix(out, "\n{\"rackaudit\":\"api\",\"time\":\""), out)
	require.True(t, strings.HasSuffix(out, "}\n"), out)
	require.Equal(t, 2, strings.Count(out, "\n"))
	line := strings.TrimPrefix(out, "\n")

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(line), &got))
	require.Equal(t, "/apps", got["path"])
	require.Equal(t, float64(200), got["status"])
	for _, k := range []string{"id", "role", "actor", "token", "tid", "pid", "command", "minted", "granted", "hours"} {
		require.NotContains(t, got, k)
	}
}

func TestWriteCaps(t *testing.T) {
	buf := captureOutput(t)

	Write(&Entry{Type: "exec", Path: "/" + strings.Repeat("p", 600), Command: strings.Repeat("\u00e9", 400)})

	var got Entry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got.Path, 512)
	require.LessOrEqual(t, len(got.Command), 512)
	require.Equal(t, strings.Repeat("\u00e9", 256), got.Command)
}

func TestNewEntry(t *testing.T) {
	r := httptest.NewRequest("POST", "/apps/x?secret=1", nil)
	r.RemoteAddr = "10.0.0.5:4000"

	e := NewEntry(r)
	require.Equal(t, &Entry{Method: "POST", Path: "/apps/x", Auth: "none", IP: "10.0.0.5", Peer: "10.0.0.5"}, e)

	r.SetBasicAuth("convox", "secret")
	require.Equal(t, "password", NewEntry(r).Auth)
	require.Empty(t, NewEntry(r).Token)

	r.SetBasicAuth("jwt", "aaa.bbb.Y2Nj")
	e = NewEntry(r)
	require.Equal(t, "jwt", e.Auth)
	require.Len(t, e.Token, 12)

	r.SetBasicAuth("jwt", "not-a-token")
	e = NewEntry(r)
	require.Equal(t, "jwt", e.Auth)
	require.Empty(t, e.Token)
}

func TestWriteStaysUnderLineSplit(t *testing.T) {
	buf := captureOutput(t)

	long := strings.Repeat("<", 8000) + string([]byte{0xff, 0xfe})
	Write(&Entry{Type: "api", Method: long, Path: long, TID: long, Command: long, Actor: SanitizeActor(long)})

	require.Equal(t, 2, strings.Count(buf.String(), "\n"))
	require.Less(t, buf.Len(), 16*1024)
	require.True(t, json.Valid(buf.Bytes()))
}
