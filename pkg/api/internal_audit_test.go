package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureMiddlewareValue(t *testing.T, s *Server, req *http.Request, key string) (any, int) {
	t.Helper()

	var captured any
	rr := httptest.NewRecorder()
	c := stdapi.NewContext(rr, req)
	next := func(c *stdapi.Context) error {
		captured = c.Get(key)
		return c.RenderOK()
	}
	if err := s.authenticate(next)(c); err != nil {
		return nil, rr.Code
	}
	return captured, rr.Code
}

func mintJwtTokenForTest(t *testing.T, jm *jwt.JwtManager, role string) string {
	t.Helper()
	var (
		tok string
		err error
	)
	switch role {
	case "r":
		tok, err = jm.ReadToken(time.Hour)
	case "rw":
		tok, err = jm.WriteToken(time.Hour)
	case "rwa":
		tok, err = jm.AdminToken(time.Hour)
	default:
		t.Fatalf("unsupported role for mint: %q", role)
	}
	require.NoError(t, err)
	return tok
}

func basicAuthHeader(user, pass string) string {
	creds := user + ":" + pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// TestAuthenticate_JwtUserClaimSet verifies JWT user claim is stashed in
// ConvoxJwtUserParam for audit actor derivation.
func TestAuthenticate_JwtUserClaimSet(t *testing.T) {
	cases := []struct {
		role     string
		wantUser string
	}{
		{"r", "system-read"},
		{"rw", "system-write"},
		{"rwa", "system-admin"},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			jm := jwt.NewJwtManager("TEST")
			s := &Server{JwtMngr: jm}

			tok := mintJwtTokenForTest(t, jm, tc.role)
			req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
			req.Header.Set("Authorization", basicAuthHeader("jwt", tok))

			got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
			assert.Equal(t, http.StatusOK, code, "JWT-auth path must return 200 for valid token")
			assert.Equal(t, tc.wantUser, got, "ConvoxJwtUserParam must carry data.User")
		})
	}
}

// TestAuthenticate_RoleStillSetOnJwtPath verifies role plumbing is unaffected.
func TestAuthenticate_RoleStillSetOnJwtPath(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")
	s := &Server{JwtMngr: jm}
	tok := mintJwtTokenForTest(t, jm, "rw")

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("jwt", tok))

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxRoleParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, structs.ConvoxRoleReadWrite, got, "rw role still threaded into ctx")
}

// TestAuthenticate_BasicAuth_SetsConvoxJwtUserParam_RackPassword verifies
// basic-auth stashes "rack-password" into ConvoxJwtUserParam.
func TestAuthenticate_BasicAuth_SetsConvoxJwtUserParam_RackPassword(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code, "basic-auth happy path must return 200")
	assert.Equal(t, "rack-password", got, "ConvoxJwtUserParam must be set to rack-password on basic-auth path")
}

func TestContextFrom_PropagatesJwtUser(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	c.Set(structs.ConvoxJwtUserParam, "system-write")

	ctx := contextFrom(c)
	got, ok := ctx.Value(structs.ConvoxJwtUserCtxKey).(string)
	assert.True(t, ok)
	assert.Equal(t, "system-write", got)
}

func TestContextFrom_NoUserClaim_LeavesUnset(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))

	ctx := contextFrom(c)
	got := ctx.Value(structs.ConvoxJwtUserCtxKey)
	assert.Nil(t, got, "no JWT user in ctx; ContextActor must fall back to unknown downstream")
}

func TestContextFrom_EmptyJwtUser_LeavesUnset(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	c.Set(structs.ConvoxJwtUserParam, "")

	ctx := contextFrom(c)
	got := ctx.Value(structs.ConvoxJwtUserCtxKey)
	assert.Nil(t, got, "empty-string claim must NOT propagate; ContextActor returns unknown")
}

func TestContextFrom_PreservesXConvoxTID(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	c.Request().Header.Set("X-Convox-TID", "tid-1")

	ctx := contextFrom(c)
	tid, _ := ctx.Value(structs.ConvoxTIDCtxKey).(string)
	assert.Equal(t, "tid-1", tid)
}

func TestContextFrom_AcceptsConvoxTIDCanonical(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	c.Request().Header.Set("Convox-TID", "tid-canonical")

	ctx := contextFrom(c)
	tid, _ := ctx.Value(structs.ConvoxTIDCtxKey).(string)
	assert.Equal(t, "tid-canonical", tid,
		"Convox-TID (canonical, RFC 6648 compliant) must populate the same ctx key as X-Convox-TID")
}

func TestContextFrom_CanonicalWinsOverLegacy(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	c.Request().Header.Set("X-Convox-TID", "tid-legacy")
	c.Request().Header.Set("Convox-TID", "tid-canonical")

	ctx := contextFrom(c)
	tid, _ := ctx.Value(structs.ConvoxTIDCtxKey).(string)
	assert.Equal(t, "tid-canonical", tid,
		"when both forms present, canonical Convox-TID must win over legacy X-Convox-TID")
}

func TestContextFrom_NoTIDHeader(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))

	ctx := contextFrom(c)
	tid, _ := ctx.Value(structs.ConvoxTIDCtxKey).(string)
	assert.Equal(t, "", tid,
		"absence of both X-Convox-TID and Convox-TID must yield empty TID (single-tenant rack)")
}

func TestContextFrom_ConcurrentReads(t *testing.T) {
	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.com", nil))
			c.Set(structs.ConvoxJwtUserParam, "system-write")
			ctx := contextFrom(c)
			got, _ := ctx.Value(structs.ConvoxJwtUserCtxKey).(string)
			if got != "system-write" {
				t.Errorf("want system-write got %q", got)
			}
		}()
	}
	wg.Wait()
}
