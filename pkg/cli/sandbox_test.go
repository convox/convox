package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeSandboxAPI serves ssh-access and an ssh tunnel that answers "pong:" plus whatever it receives, then closes.
func fakeSandboxAPI(t *testing.T) (*httptest.Server, *[]map[string]interface{}) {
	t.Helper()
	var accessRequests []map[string]interface{}
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/apis/v1/sandboxes/sbx-1/ssh-access":
			if r.Header.Get("X-Deploy-Token") != "dk" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad key"}}`))
				return
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			accessRequests = append(accessRequests, body)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"token": "0123456789abcdef0123456789abcdef", "sandbox_id": "sbx-1", "user": "convox", "expires_at": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			})
		case r.URL.Path == "/apis/v1/sandboxes/sbx-1/ssh":
			if r.Header.Get("X-Convox-Ssh-Token") != "0123456789abcdef0123456789abcdef" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"the SSH access token is invalid or expired"}}`))
				return
			}
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer c.Close()
			_, data, err := c.ReadMessage()
			if err == nil {
				_ = c.WriteMessage(websocket.BinaryMessage, append([]byte("pong:"), data...))
			}
			_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &accessRequests
}

func TestSandboxProxy(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		srv, _ := fakeSandboxAPI(t)

		res, err := testExecute(e, "sandbox proxy sbx-1 --token 0123456789abcdef0123456789abcdef --api-url "+srv.URL, strings.NewReader("hello"))
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, res.Stderr)
		require.Equal(t, "pong:hello", res.Stdout)

		res, err = testExecute(e, "sandbox proxy sbx-1 --token nope --api-url "+srv.URL, strings.NewReader("x"))
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "the SSH access token is invalid or expired")

		res, err = testExecute(e, "sandbox proxy sbx-1 --api-url "+srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--token is required")
	})
}

func expectKeygen(me *mockstdcli.Executor) {
	me.On("Execute", "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "convox-sandbox", "-f", mock.Anything).
		Run(func(args mock.Arguments) {
			key := args.String(9)
			_ = os.WriteFile(key, []byte("PRIVATE"), 0o600)
			_ = os.WriteFile(key+".pub", []byte("ssh-ed25519 AAAA convox-sandbox\n"), 0o644)
		}).Return([]byte{}, nil).Once()
}

func TestSandboxSSHConfig(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		srv, requests := fakeSandboxAPI(t)
		t.Setenv("CONVOX_DEPLOY_KEY", "dk")
		me := &mockstdcli.Executor{}
		expectKeygen(me)
		e.Executor = me

		res, err := testExecute(e, "sandbox ssh-config sbx-1 --api-url "+srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, res.Stderr)
		key := filepath.Join(e.Settings, "sandbox-ssh", "id_ed25519")
		require.Contains(t, res.Stdout, "Host convox-sandbox-sbx-1")
		require.Contains(t, res.Stdout, "IdentityFile "+key)
		require.Contains(t, res.Stdout, "2026-01-02T03:04:05Z")
		require.Len(t, *requests, 1)
		require.Equal(t, "ssh-ed25519 AAAA convox-sandbox", (*requests)[0]["public_key"])
		require.Equal(t, float64(1440), (*requests)[0]["expires_in_minutes"])

		res, err = testExecute(e, "sandbox ssh-config sbx-1 --expires 5 --api-url "+srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, res.Stderr)
		require.Equal(t, float64(5), (*requests)[1]["expires_in_minutes"], "the key is created once and reused")
		me.AssertExpectations(t)
	})
}

func TestSandboxSSH(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		srv, _ := fakeSandboxAPI(t)
		t.Setenv("CONVOX_DEPLOY_KEY", "dk")
		me := &mockstdcli.Executor{}
		expectKeygen(me)
		var sshArgs []string
		me.On("Terminal", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				for _, a := range args {
					sshArgs = append(sshArgs, a.(string))
				}
			}).Return(nil).Once()
		e.Executor = me

		res, err := testExecute(e, "sandbox ssh sbx-1 --api-url "+srv.URL+" -- uname -a", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, res.Stderr)
		require.Equal(t, "ssh", sshArgs[0])
		joined := strings.Join(sshArgs, " ")
		require.Contains(t, joined, "-i "+filepath.Join(e.Settings, "sandbox-ssh", "id_ed25519"))
		require.Contains(t, joined, "ProxyCommand=")
		require.Contains(t, joined, "sandbox proxy sbx-1 --token-file ")
		require.NotContains(t, joined, "0123456789abcdef0123456789abcdef")
		require.Equal(t, []string{"convox@sbx-1", "uname", "-a"}, sshArgs[len(sshArgs)-3:])

		t.Setenv("CONVOX_DEPLOY_KEY", "")
		res, err = testExecute(e, "sandbox ssh sbx-1 --api-url "+srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "CONVOX_DEPLOY_KEY")
	})
}
