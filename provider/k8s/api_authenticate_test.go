package k8s

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/audit"
	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	gojwt "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const testProxySignKey = "testkey"

type proxyAuthResult struct {
	code       int
	ran        bool
	authHeader string
	body       string
}

func serveProxyAuth(p *Provider, creds ...string) proxyAuthResult {
	res := proxyAuthResult{}

	stub := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		res.ran = true
		res.authHeader = r.Header.Get("Authorization")
	})

	req := httptest.NewRequest("GET", "/api/v1/namespaces", nil)
	if len(creds) == 2 {
		req.SetBasicAuth(creds[0], creds[1])
	}

	w := httptest.NewRecorder()
	p.apiProxyAuthenticate(stub)(w, req)

	res.code = w.Code
	res.body = w.Body.String()

	return res
}

func mintTokenWithRole(t *testing.T, signKey, role string, expires time.Time) string {
	t.Helper()

	tk := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.MapClaims{
		"user":      "test-user",
		"role":      role,
		"expiresAt": expires.UTC().Unix(),
	})

	s, err := tk.SignedString([]byte(signKey))
	require.NoError(t, err)

	return s
}

func mintTokenWithoutRole(t *testing.T, signKey string) string {
	t.Helper()

	tk := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.MapClaims{
		"user":      "test-user",
		"expiresAt": time.Now().UTC().Add(time.Hour).Unix(),
	})

	s, err := tk.SignedString([]byte(signKey))
	require.NoError(t, err)

	return s
}

func testProxyProvider(password string) *Provider {
	return &Provider{
		Password: password,
		JwtMngr:  jwt.NewJwtManager(testProxySignKey),
	}
}

func TestApiProxyAuthenticateJwtRoles(t *testing.T) {
	p := testProxyProvider("rackpassword")

	admin, err := p.JwtMngr.AdminToken(time.Hour)
	require.NoError(t, err)
	read, err := p.JwtMngr.ReadToken(time.Hour)
	require.NoError(t, err)
	write, err := p.JwtMngr.WriteToken(time.Hour)
	require.NoError(t, err)

	res := serveProxyAuth(p, "jwt", admin)
	require.True(t, res.ran)
	require.Equal(t, http.StatusOK, res.code)
	require.Empty(t, res.authHeader)

	for name, token := range map[string]string{"read": read, "write": write} {
		t.Run(name, func(t *testing.T) {
			res := serveProxyAuth(p, "jwt", token)
			require.False(t, res.ran)
			require.Equal(t, http.StatusForbidden, res.code)
			require.Equal(t, "admin role required for kubernetes api access\n", res.body)
		})
	}
}

// Roles containing "a" but not equal to "rwa" separate the exact-role check
// from a substring match, which pkg/jwt's three tokens cannot distinguish.
func TestApiProxyAuthenticateNonAdminRoleVariants(t *testing.T) {
	p := testProxyProvider("rackpassword")

	for _, role := range []string{"ra", "admin", "a", "rw", structs.ConvoxRoleRead} {
		t.Run(role, func(t *testing.T) {
			res := serveProxyAuth(p, "jwt", mintTokenWithRole(t, testProxySignKey, role, time.Now().Add(time.Hour)))
			require.False(t, res.ran)
			require.Equal(t, http.StatusForbidden, res.code)
		})
	}
}

func TestApiProxyAuthenticateInvalidTokens(t *testing.T) {
	p := testProxyProvider("rackpassword")

	tokens := map[string]string{
		"malformed": "not-a-token",
		"wrong-key": mintTokenWithRole(t, "otherkey", structs.ConvoxRoleAdmin, time.Now().Add(time.Hour)),
		"expired":   mintTokenWithRole(t, testProxySignKey, structs.ConvoxRoleAdmin, time.Now().Add(-time.Hour)),
		"empty":     "",
		"no-role":   mintTokenWithoutRole(t, testProxySignKey),
	}

	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			res := serveProxyAuth(p, "jwt", token)
			require.False(t, res.ran)
			require.Equal(t, http.StatusUnauthorized, res.code)
		})
	}
}

func TestApiProxyAuthenticateJwtManagerNil(t *testing.T) {
	p := &Provider{Password: "rackpassword"}

	admin, err := jwt.NewJwtManager(testProxySignKey).AdminToken(time.Hour)
	require.NoError(t, err)

	res := serveProxyAuth(p, "jwt", admin)
	require.False(t, res.ran)
	require.Equal(t, http.StatusUnauthorized, res.code)
}

func TestApiProxyAuthenticatePassword(t *testing.T) {
	p := testProxyProvider("rackpassword")

	for _, username := range []string{"convox", "anything", "jwt-but-not-exact", ""} {
		t.Run("correct-"+username, func(t *testing.T) {
			res := serveProxyAuth(p, username, "rackpassword")
			require.True(t, res.ran)
			require.Equal(t, http.StatusOK, res.code)
			require.Empty(t, res.authHeader)
		})
	}

	for _, password := range []string{"wrongpassword", "", "rackpassword2", "rackpasswor"} {
		t.Run("wrong-"+password, func(t *testing.T) {
			res := serveProxyAuth(p, "convox", password)
			require.False(t, res.ran)
			require.Equal(t, http.StatusUnauthorized, res.code)
		})
	}

	t.Run("no-credentials", func(t *testing.T) {
		res := serveProxyAuth(p)
		require.False(t, res.ran)
		require.Equal(t, http.StatusUnauthorized, res.code)
	})
}

func TestApiProxyAuthenticateEmptyRackPassword(t *testing.T) {
	p := testProxyProvider("")

	for _, password := range []string{"", "anything"} {
		t.Run("password-"+password, func(t *testing.T) {
			res := serveProxyAuth(p, "convox", password)
			require.False(t, res.ran)
			require.Equal(t, http.StatusUnauthorized, res.code)
		})
	}

	t.Run("no-credentials", func(t *testing.T) {
		res := serveProxyAuth(p)
		require.False(t, res.ran)
		require.Equal(t, http.StatusUnauthorized, res.code)
	})

	t.Run("admin-token-still-works", func(t *testing.T) {
		admin, err := p.JwtMngr.AdminToken(time.Hour)
		require.NoError(t, err)

		res := serveProxyAuth(p, "jwt", admin)
		require.True(t, res.ran)
		require.Equal(t, http.StatusOK, res.code)
		require.Empty(t, res.authHeader)
	})
}

func TestApiProxyAuthenticateAuditRecords(t *testing.T) {
	var buf bytes.Buffer
	prev := audit.Output
	audit.Output = &buf
	t.Cleanup(func() { audit.Output = prev })

	p := testProxyProvider("rackpassword")

	admin, err := p.JwtMngr.AdminToken(time.Hour)
	require.NoError(t, err)
	read, err := p.JwtMngr.ReadToken(time.Hour)
	require.NoError(t, err)
	expired := mintTokenWithRole(t, testProxySignKey, structs.ConvoxRoleAdmin, time.Now().Add(-time.Hour))

	cases := []struct {
		name   string
		creds  []string
		actor  string
		status int
		want   audit.Entry
	}{
		{"password", []string{"convox", "rackpassword"}, "", 0, audit.Entry{Auth: "password", Role: structs.ConvoxRoleAdmin, Actor: "rack-password"}},
		{"password claimed actor", []string{"convox", "rackpassword"}, "alice@example.com", 0, audit.Entry{Auth: "password", Role: structs.ConvoxRoleAdmin, Actor: "alice@example.com"}},
		{"admin jwt", []string{"jwt", admin}, "", 0, audit.Entry{Auth: "jwt", Role: structs.ConvoxRoleAdmin, Actor: "system-admin", Token: jwt.Fingerprint(admin)}},
		{"read jwt", []string{"jwt", read}, "", http.StatusForbidden, audit.Entry{Auth: "jwt", Role: structs.ConvoxRoleRead, Actor: "system-read", Token: jwt.Fingerprint(read)}},
		{"expired jwt", []string{"jwt", expired}, "alice@example.com", http.StatusUnauthorized, audit.Entry{Auth: "jwt", Token: jwt.Fingerprint(expired)}},
		{"wrong password", []string{"rackpassword", "wrong"}, "alice@example.com", http.StatusUnauthorized, audit.Entry{Auth: "password"}},
		{"no credentials", nil, "", http.StatusUnauthorized, audit.Entry{Auth: "none"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()

			var authHeader string
			stub := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				authHeader = r.Header.Get("Authorization")
				require.Equal(t, 1, strings.Count(buf.String(), "\n{"), "record written before proxying")
			})

			req := httptest.NewRequest("GET", "/api/v1/namespaces/app/pods/web-1/exec?command=sh&command=-c&command=id&stdout=true", nil)
			req.URL.Path = "/api/v1/namespaces/app/pods/web-1/exec"
			req.RemoteAddr = "10.0.5.6:41000"
			req.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.9")
			if tc.actor != "" {
				req.Header.Set("X-Convox-Actor", tc.actor)
			}
			if len(tc.creds) == 2 {
				req.SetBasicAuth(tc.creds[0], tc.creds[1])
			}

			p.apiProxyAuthenticate(stub)(httptest.NewRecorder(), req)

			require.Empty(t, authHeader)
			require.Equal(t, 1, strings.Count(buf.String(), "\n{"))
			require.NotContains(t, buf.String(), "rackpassword")
			require.NotContains(t, buf.String(), "command=")
			if len(tc.creds) == 2 {
				require.NotContains(t, buf.String(), tc.creds[1])
			}

			var got audit.Entry
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
			require.NotEmpty(t, got.Time)
			got.Time = ""

			want := tc.want
			want.Type = "kubernetes"
			want.Op = "Kubernetes"
			want.Method = "GET"
			want.Path = "/kubernetes/api/v1/namespaces/app/pods/web-1/exec"
			want.Status = tc.status
			want.IP = "203.0.113.9"
			want.Peer = "10.0.5.6"
			if tc.status == 0 || tc.status == http.StatusForbidden {
				want.Command = "sh -c id"
			}
			require.Equal(t, want, got)
		})
	}
}

func TestAppParametersUpdateLogLine(t *testing.T) {
	p := &Provider{Engine: &MockEngine{}}
	a := &structs.App{Parameters: map[string]string{}}

	out := captureStdout(t, func() {
		require.NoError(t, p.appParametersUpdate(a, map[string]string{"x\n{\"rackaudit\":\"token\"}\n": "1"}))
	})
	require.Equal(t, 1, strings.Count(out, "\n"), out)
	require.True(t, strings.HasSuffix(out, "\n"), out)

	out = captureStdout(t, func() {
		require.NoError(t, p.appParametersUpdate(a, map[string]string{}))
	})
	require.Empty(t, out)
}
