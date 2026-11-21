package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/audit"
	cjwt "github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdsdk"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const auditPassword = "rackpassword"

type auditBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *auditBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *auditBuffer) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func (b *auditBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *auditBuffer) records(t *testing.T) []audit.Entry {
	t.Helper()

	es := []audit.Entry{}
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		if line == "" {
			continue
		}
		var e audit.Entry
		require.NoError(t, json.Unmarshal([]byte(line), &e), line)
		es = append(es, e)
	}

	return es
}

func (b *auditBuffer) waitRecords(t *testing.T, n int) []audit.Entry {
	t.Helper()

	require.Eventually(t, func() bool { return strings.Count(b.String(), "\n{") >= n }, 5*time.Second, 10*time.Millisecond)

	return b.records(t)
}

type auditServer struct {
	url string
	p   *structs.MockProvider
	out *auditBuffer
	jm  *cjwt.JwtManager
}

func testAuditServer(t *testing.T, fn func(*auditServer)) {
	t.Helper()

	out := &auditBuffer{}
	prev := audit.Output
	audit.Output = out
	t.Cleanup(func() { audit.Output = prev })

	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(p)
	s.Password = auditPassword
	s.Logger = logger.Discard

	ht := httptest.NewServer(s)
	defer ht.Close()

	fn(&auditServer{url: ht.URL, p: p, out: out, jm: cjwt.NewJwtManager("test")})

	p.AssertExpectations(t)
}

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func (as *auditServer) do(t *testing.T, method, path string, headers map[string]string, body io.Reader) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(method, as.url+path, body)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return res.StatusCode, data
}

func (as *auditServer) socket(t *testing.T, path string, headers map[string]string) io.ReadCloser {
	t.Helper()

	c, err := stdsdk.New(as.url)
	require.NoError(t, err)

	r, err := c.Websocket(path, stdsdk.RequestOptions{Headers: headers})
	require.NoError(t, err)

	return r
}

func (as *auditServer) token(t *testing.T, role string) string {
	t.Helper()

	var tk string
	var err error
	switch role {
	case "read":
		tk, err = as.jm.ReadToken(time.Hour)
	case "write":
		tk, err = as.jm.WriteToken(time.Hour)
	default:
		tk, err = as.jm.AdminToken(time.Hour)
	}
	require.NoError(t, err)

	return tk
}

func TestAuditRecordPerRequest(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		read := as.token(t, "read")
		expired, err := as.jm.AdminToken(-time.Hour)
		require.NoError(t, err)

		as.p.On("AppList").Return(structs.Apps{}, nil)

		cases := []struct {
			name    string
			headers map[string]string
			status  int
			want    audit.Entry
		}{
			{"password claimed actor", map[string]string{"Authorization": basicAuth("convox", auditPassword), "X-Convox-Actor": "alice@example.com"}, 200,
				audit.Entry{Auth: "password", Role: structs.ConvoxRoleAdmin, Actor: "alice@example.com"}},
			{"password", map[string]string{"Authorization": basicAuth("convox", auditPassword)}, 200,
				audit.Entry{Auth: "password", Role: structs.ConvoxRoleAdmin, Actor: "rack-password"}},
			{"read jwt", map[string]string{"Authorization": basicAuth("jwt", read)}, 200,
				audit.Entry{Auth: "jwt", Role: structs.ConvoxRoleRead, Actor: "system-read", Token: cjwt.Fingerprint(read)}},
			{"expired jwt", map[string]string{"Authorization": basicAuth("jwt", expired), "X-Convox-Actor": "alice@example.com"}, 401,
				audit.Entry{Auth: "jwt", Token: cjwt.Fingerprint(expired)}},
			{"password in username", map[string]string{"Authorization": basicAuth(auditPassword+"x", "")}, 401,
				audit.Entry{Auth: "password"}},
			{"no credentials", map[string]string{"X-Convox-Actor": "alice@example.com"}, 401,
				audit.Entry{Auth: "none"}},
			{"tid", map[string]string{"Authorization": basicAuth("convox", auditPassword), "X-Convox-TID": "t1"}, 200,
				audit.Entry{Auth: "password", Role: structs.ConvoxRoleAdmin, Actor: "rack-password", TID: "t1"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				as.out.reset()

				headers := map[string]string{"X-Forwarded-For": "198.51.100.1, 203.0.113.9"}
				for k, v := range tc.headers {
					headers[k] = v
				}

				status, _ := as.do(t, "GET", "/apps?secret=s3cr3t", headers, nil)
				require.Equal(t, tc.status, status)

				out := as.out.String()
				require.NotContains(t, out, auditPassword)
				require.NotContains(t, out, "s3cr3t")
				require.NotContains(t, out, read)
				require.NotContains(t, out, expired)

				es := as.out.records(t)
				require.Len(t, es, 1)
				got := es[0]
				require.NotEmpty(t, got.Time)
				require.NotEmpty(t, got.ID)
				got.Time, got.ID = "", ""

				want := tc.want
				want.Type, want.Op, want.Method, want.Path, want.Status = "api", "AppList", "GET", "/apps", tc.status
				want.IP, want.Peer = "203.0.113.9", "127.0.0.1"
				require.Equal(t, want, got)
			})
		}
	})
}

func TestAuditRecordAuthorizeDenied(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		read := as.token(t, "read")

		status, _ := as.do(t, "POST", "/apps", map[string]string{"Authorization": basicAuth("jwt", read)}, nil)
		require.Equal(t, http.StatusUnauthorized, status)

		es := as.out.records(t)
		require.Len(t, es, 1)
		require.Equal(t, 401, es[0].Status)
		require.Equal(t, structs.ConvoxRoleRead, es[0].Role)
		require.Equal(t, "system-read", es[0].Actor)
	})
}

func TestAuditRecordMintThenUse(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("AppList").Return(structs.Apps{}, nil)

		headers := map[string]string{
			"Authorization": basicAuth("convox", auditPassword),
			"Content-Type":  "application/x-www-form-urlencoded",
			"Connection":    "Upgrade",
			"Upgrade":       "websocket",
		}
		status, body := as.do(t, "POST", "/system/jwt/token", headers, strings.NewReader("role=admin&durationInHour=8"))
		require.Equal(t, http.StatusOK, status)

		var jt structs.SystemJwt
		require.NoError(t, json.Unmarshal(body, &jt))
		require.NotEmpty(t, jt.Token)

		status, _ = as.do(t, "GET", "/apps", map[string]string{"Authorization": basicAuth("jwt", jt.Token)}, nil)
		require.Equal(t, http.StatusOK, status)

		require.NotContains(t, as.out.String(), jt.Token)

		es := as.out.records(t)
		require.Len(t, es, 2)

		require.Equal(t, "token", es[0].Type)
		require.Equal(t, "SystemJwtToken", es[0].Op)
		require.Equal(t, 200, es[0].Status)
		require.Equal(t, cjwt.Fingerprint(jt.Token), es[0].Minted)
		require.Equal(t, "admin", es[0].Granted)
		require.Equal(t, 8, es[0].Hours)

		require.Equal(t, es[0].Minted, es[1].Token)
		require.Equal(t, "system-admin", es[1].Actor)
	})
}

func TestAuditRecordProcessRunCommand(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("AppGet", "app1").Return(&structs.App{Name: "app1"}, nil)
		as.p.On("ProcessRun", "app1", "web", mock.Anything).Return(&structs.Process{Id: "web-1"}, nil)

		headers := map[string]string{
			"Authorization": basicAuth("convox", auditPassword),
			"Command":       "rake db:migrate",
			"Environment":   "DATABASE_PASSWORD=hunter2",
		}
		status, _ := as.do(t, "POST", "/apps/app1/services/web/processes?command=decoy", headers, nil)
		require.Equal(t, http.StatusOK, status)

		require.NotContains(t, as.out.String(), "decoy")
		require.NotContains(t, as.out.String(), "hunter2")

		es := as.out.records(t)
		require.Len(t, es, 1)
		require.Equal(t, "exec", es[0].Type)
		require.Equal(t, "rake db:migrate", es[0].Command)
		require.Equal(t, "web-1", es[0].Pid)
	})
}

func TestAuditRecordSessionOpen(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		release := make(chan struct{})

		as.p.On("AppGet", "app1").Return(&structs.App{Name: "app1"}, nil)
		as.p.On("ProcessExec", "app1", "web-1", "bash", mock.Anything, mock.Anything).Run(func(mock.Arguments) { <-release }).Return(0, nil)

		r := as.socket(t, "/apps/app1/processes/web-1/exec", map[string]string{
			"Authorization": basicAuth("convox", auditPassword),
			"Command":       "bash",
		})

		es := as.out.waitRecords(t, 1)
		require.Len(t, es, 1)
		require.Equal(t, "exec", es[0].Type)
		require.Equal(t, "ProcessExec", es[0].Op)
		require.Equal(t, 101, es[0].Status)
		require.Equal(t, "bash", es[0].Command)

		close(release)
		_, err := io.ReadAll(r)
		require.NoError(t, err)

		require.Len(t, as.out.records(t), 1)
	})
}

func TestAuditRecordSessionError(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("AppGet", "app1").Return(&structs.App{Name: "app1"}, nil)

		read := as.token(t, "read")
		r := as.socket(t, "/apps/app1/processes/web-1/exec", map[string]string{
			"Authorization": basicAuth("jwt", read),
			"Command":       "bash",
		})
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Contains(t, string(data), "process exec requires write access")

		es := as.out.waitRecords(t, 2)
		require.Len(t, es, 2)
		require.Equal(t, 101, es[0].Status)
		require.Equal(t, 403, es[1].Status)
		require.Equal(t, es[0].ID, es[1].ID)
		require.Equal(t, "bash", es[1].Command)
	})
}

func TestAuditRecordProxySessionEnd(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("Proxy", "web.app1.svc.cluster.local", 5000, mock.Anything, mock.Anything).Return(fmt.Errorf("TCP connection closed from destination"))

		r := as.socket(t, "/proxy/web.app1.svc.cluster.local/5000", map[string]string{"Authorization": basicAuth("convox", auditPassword)})
		_, err := io.ReadAll(r)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		es := as.out.records(t)
		require.Len(t, es, 1)
		require.Equal(t, "exec", es[0].Type)
		require.Equal(t, 101, es[0].Status)
	})
}

func TestAuditRecordPanic(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("AppList").Run(func(mock.Arguments) { panic(http.ErrAbortHandler) }).Return(structs.Apps{}, nil)

		req, err := http.NewRequest("GET", as.url+"/apps", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", basicAuth("convox", auditPassword))
		if res, err := http.DefaultClient.Do(req); err == nil {
			res.Body.Close()
		}

		es := as.out.waitRecords(t, 1)
		require.Len(t, es, 1)
		require.Equal(t, 500, es[0].Status)
		require.Equal(t, "AppList", es[0].Op)
	})
}

func TestAuditRecordProxyHttpServiceOriginalRequest(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		write := as.token(t, "write")

		as.do(t, "GET", "/custom/http/proxy/internal/path?x=1", map[string]string{
			"Authorization": basicAuth("jwt", write),
			"X-Host":        "web.app1.svc.cluster.local",
			"X-Port":        "5000",
		}, nil)

		es := as.out.records(t)
		require.Len(t, es, 1)
		require.Equal(t, "/custom/http/proxy/internal/path", es[0].Path)
		require.Equal(t, "jwt", es[0].Auth)
		require.Equal(t, cjwt.Fingerprint(write), es[0].Token)
	})
}

func auditLogLines(lines ...string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "")))
}

const (
	auditRecord    = `{"rackaudit":"api","time":"2026-11-20T10:00:00Z","op":"AppList","method":"GET","path":"/apps","status":200,"auth":"none","ip":"10.0.0.1","peer":"10.0.0.1"}`
	auditForged    = `{"time":"2026-11-20T10:00:00Z","http_user_agent":"{\"rackaudit\":\"api\"}"}`
	auditTrailing  = `{"rackaudit":"token","actor":"alice"} client_supplied="z"`
	auditPlainLine = `id=abc ns=api at=AppList method="GET" path="/apps" response=200 elapsed=1.000`
)

func TestSystemLogsDropsAuditRecords(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("SystemLogs", structs.LogsOptions{Since: options.Duration(2 * time.Minute), Prefix: options.Bool(true)}).Return(auditLogLines(
			"2026-11-20T10:00:00Z service/api/api-1 "+auditPlainLine+"\n",
			"2026-11-20T10:00:00Z service/api/api-1 "+auditRecord+"\n",
			"2026-11-20T10:00:00Z /nginx-access-logs "+auditRecord+"\n",
			"2026-11-20T10:00:00Z /nginx-access-logs "+auditForged+"\n",
			"2026-11-20T10:00:01Z service/router/router-1 tail",
		), nil).Once()

		r := as.socket(t, "/system/logs", map[string]string{"Authorization": basicAuth("convox", auditPassword), "Prefix": "true"})
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, ""+
			"2026-11-20T10:00:00Z service/api/api-1 "+auditPlainLine+"\n"+
			"2026-11-20T10:00:00Z /nginx-access-logs "+auditRecord+"\n"+
			"2026-11-20T10:00:00Z /nginx-access-logs "+auditForged+"\n"+
			"2026-11-20T10:00:01Z service/router/router-1 tail", string(data))

		as.p.On("SystemLogs", structs.LogsOptions{Since: options.Duration(2 * time.Minute)}).Return(auditLogLines(
			auditPlainLine+"\n",
			auditRecord+"\n",
			auditForged+"\n",
		), nil).Once()

		read := as.token(t, "read")
		r = as.socket(t, "/system/logs", map[string]string{"Authorization": basicAuth("jwt", read)})
		data, err = io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, auditPlainLine+"\n"+auditForged+"\n", string(data))
	})
}

func TestSystemLogsFollowsPastEmptyWrites(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		pr, pw := io.Pipe()
		go func() {
			for i := 0; i < 150; i++ {
				if _, err := pw.Write([]byte{}); err != nil {
					return
				}
			}
			_, _ = pw.Write([]byte("line\n"))
			pw.Close()
		}()

		as.p.On("SystemLogs", structs.LogsOptions{Since: options.Duration(2 * time.Minute)}).Return(pr, nil)

		r := as.socket(t, "/system/logs", map[string]string{"Authorization": basicAuth("convox", auditPassword)})
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "line\n", string(data))
	})
}

func TestSystemAuditLogs(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		opts := structs.LogsOptions{Filter: options.String(audit.Marker), Prefix: options.Bool(true), Since: options.Duration(time.Hour)}
		lines := func() io.ReadCloser {
			return auditLogLines(
				"2026-11-20T10:00:00Z service/api/api-1 "+auditRecord+"\n",
				"2026-11-20T10:00:00Z service/api/api-1 "+auditPlainLine+"\n",
				"2026-11-20T10:00:00Z service/api/api-1 "+auditTrailing+"\n",
				"2026-11-20T10:00:00Z /nginx-access-logs "+auditRecord+"\n",
				"error: AccessDeniedException: not authorized\n",
			)
		}
		as.p.On("SystemLogs", opts).Return(lines(), nil).Once()
		as.p.On("SystemLogs", opts).Return(lines(), nil).Once()

		for _, auth := range []string{basicAuth("convox", auditPassword), basicAuth("jwt", as.token(t, "admin"))} {
			r := as.socket(t, "/system/audit/logs", map[string]string{"Authorization": auth, "Since": "1h", "Filter": "other", "Prefix": "false"})
			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, auditRecord+"\nerror: AccessDeniedException: not authorized\n", string(data))
		}
	})
}

func TestSystemAuditLogsRefused(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		cases := map[string]map[string]string{
			"read":  {"Authorization": basicAuth("jwt", as.token(t, "read"))},
			"write": {"Authorization": basicAuth("jwt", as.token(t, "write"))},
			"tid":   {"Authorization": basicAuth("convox", auditPassword), "X-Convox-TID": "t1"},
		}

		for name, headers := range cases {
			t.Run(name, func(t *testing.T) {
				as.out.reset()

				r := as.socket(t, "/system/audit/logs", headers)
				data, err := io.ReadAll(r)
				require.NoError(t, err)
				require.Contains(t, string(data), "ERROR: ")

				es := as.out.waitRecords(t, 2)
				require.Equal(t, 101, es[0].Status)
				require.Equal(t, 403, es[1].Status)
				require.Equal(t, "SystemAuditLogs", es[1].Op)
			})
		}
	})
}

func TestAuditRecordInstanceShellCommand(t *testing.T) {
	testAuditServer(t, func(as *auditServer) {
		as.p.On("InstanceShell", "i-1", mock.Anything, mock.MatchedBy(func(opts structs.InstanceShellOptions) bool {
			return opts.Command != nil && *opts.Command == "cat /etc/hosts"
		})).Return(0, nil)

		h := http.Header{}
		h.Set("Authorization", basicAuth("convox", auditPassword))
		h.Set("Command", "cat /etc/hosts")
		h.Set("Private-Key", "c2VjcmV0LWtleQ==")

		ws, res, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(as.url, "http")+"/instances/i-1/shell?command=decoy", h)
		require.NoError(t, err)
		res.Body.Close()
		defer ws.Close()

		for {
			kind, data, err := ws.ReadMessage()
			require.NoError(t, err)
			if kind == websocket.BinaryMessage && len(data) == 0 {
				break
			}
		}

		out := as.out.String()
		require.NotContains(t, out, "decoy")
		require.NotContains(t, out, "c2VjcmV0LWtleQ==")

		es := as.out.records(t)
		require.Len(t, es, 1)
		require.Equal(t, "exec", es[0].Type)
		require.Equal(t, "InstanceShell", es[0].Op)
		require.Equal(t, "cat /etc/hosts", es[0].Command)
	})
}
