package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
