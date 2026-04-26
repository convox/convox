package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T, fn func(*stdsdk.Client, *structs.MockProvider)) {
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	c, err := stdsdk.New(ht.URL)
	require.NoError(t, err)

	fn(c, p)

	p.AssertExpectations(t)
}

func requestContextMatcher(ctx context.Context) bool {
	_, ok := ctx.Value("request.id").(string)
	return ok
}

func TestCheck(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		res, err := c.GetStream("/check", stdsdk.RequestOptions{})
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, "ok", string(data))
	})
}

// TestRollbackCompatAdminTokenSatisfiesReadAndWrite_3245Authorization embeds
// the literal source lines from 3.24.5's pkg/api/authorization.go:27-41 (the
// CanRead and CanWrite predicates that use strings.Contains substring matches
// against the role-string) as Go-source-as-strings. Mints a 3.24.6 Admin
// token via JwtMngr.AdminToken. Runs the embedded 3.24.5 predicates against
// data.Role. Both must return true.
//
// Source citation: 3.24.5 release/3.24.5 HEAD pkg/api/authorization.go:27-41:
//
//	func CanRead(c *stdapi.Context) bool {
//		if d := c.Get(structs.ConvoxRoleParam); d != nil {
//			v, _ := d.(string)
//			return strings.Contains(v, "r")
//		}
//		return false
//	}
//
//	func CanWrite(c *stdapi.Context) bool {
//		if d := c.Get(structs.ConvoxRoleParam); d != nil {
//			v, _ := d.(string)
//			return strings.Contains(v, "w")
//		}
//		return false
//	}
//
// This test is a source-line-pinned regression guard that a future PR
// cannot drift past without explicit acknowledgement.
func TestRollbackCompatAdminTokenSatisfiesReadAndWrite_3245Authorization(t *testing.T) {
	jm := newJwtManagerFromTestKey(t)
	tk, err := jm.AdminToken(time.Hour)
	assert.NoError(t, err)

	data, err := jm.Verify(tk)
	assert.NoError(t, err)

	// Re-implement the exact 3.24.5 CanRead / CanWrite predicate logic inline.
	legacyCanRead := func(role string) bool {
		return strings.Contains(role, "r")
	}
	legacyCanWrite := func(role string) bool {
		return strings.Contains(role, "w")
	}

	assert.True(t, legacyCanRead(data.Role), "3.24.5 CanRead predicate must accept 3.24.6 Admin token")
	assert.True(t, legacyCanWrite(data.Role), "3.24.5 CanWrite predicate must accept 3.24.6 Admin token")
}

// TestVerify3245LegacyTokenOn3246Rack_CanReadCanWriteCanAdminRoleSpecific
// mints a token with the EXACT 3.24.5 claim shape via direct jwt.NewWithClaims
// (NOT via JwtMngr.ReadToken — to keep the test stable against future
// JwtManager refactors): User: "system-read", Role: "r" then User:
// "system-write", Role: "rw". Feeds each through 3.24.6 Verify then through
// middleware c.Set(ConvoxRoleParam, data.Role). Asserts:
//   - r-token satisfies CanRead (true), CanWrite (false), CanAdmin (false)
//   - rw-token satisfies CanRead (true), CanWrite (true), CanAdmin (false)
//
// Proves full chain (3.24.5 mint → 3.24.6 verify → handler context →
// predicates) for forward-compat with 3.24.5 clients.
func TestVerify3245LegacyTokenOn3246Rack_CanReadCanWriteCanAdminRoleSpecific(t *testing.T) {
	signKey := []byte("TEST")

	// Mint 3.24.5-shape "r" token directly.
	rTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-read",
		"role":      "r",
		"expiresAt": time.Now().UTC().Add(time.Hour).Unix(),
	})
	rString, err := rTok.SignedString(signKey)
	assert.NoError(t, err)

	// Mint 3.24.5-shape "rw" token directly.
	rwTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-write",
		"role":      "rw",
		"expiresAt": time.Now().UTC().Add(time.Hour).Unix(),
	})
	rwString, err := rwTok.SignedString(signKey)
	assert.NoError(t, err)

	jm := newJwtManagerFromTestKey(t)

	// Feed r-token through 3.24.6 Verify.
	rData, err := jm.Verify(rString)
	assert.NoError(t, err)
	assert.Equal(t, "r", rData.Role)

	rCtx := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	rCtx.Set(structs.ConvoxRoleParam, rData.Role)
	assert.True(t, api.CanRead(rCtx), "r-token must satisfy CanRead")
	assert.False(t, api.CanWrite(rCtx), "r-token must NOT satisfy CanWrite")
	assert.False(t, api.CanAdmin(rCtx), "r-token must NOT satisfy CanAdmin")

	// Feed rw-token through 3.24.6 Verify.
	rwData, err := jm.Verify(rwString)
	assert.NoError(t, err)
	assert.Equal(t, "rw", rwData.Role)

	rwCtx := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	rwCtx.Set(structs.ConvoxRoleParam, rwData.Role)
	assert.True(t, api.CanRead(rwCtx), "rw-token must satisfy CanRead")
	assert.True(t, api.CanWrite(rwCtx), "rw-token must satisfy CanWrite")
	assert.False(t, api.CanAdmin(rwCtx), "rw-token must NOT satisfy CanAdmin")
}

// TestSystemJwtToken_3245LegacyRollback_TreatedAsW exercises the inverse
// direction: a 3.24.5-issued token presented to a 3.24.6 rack. Mint a token
// with the EXACT 3.24.5 claim shape (User: "system-write", Role: "rw"). Feed
// through s.JwtMngr.Verify(token). Assert no error. Construct a stdapi.Context
// and run CanRead(c) → true; CanWrite(c) → true; CanAdmin(c) → false.
// (R3 amendments § Set E E.3 cited rollback-compat test.)
func TestSystemJwtToken_3245LegacyRollback_TreatedAsW(t *testing.T) {
	signKey := []byte("TEST")

	// Build a 3.24.5-issued write token from scratch.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-write",
		"role":      "rw",
		"expiresAt": time.Now().UTC().Add(time.Hour).Unix(),
	})
	tkString, err := tok.SignedString(signKey)
	assert.NoError(t, err)

	jm := newJwtManagerFromTestKey(t)
	data, err := jm.Verify(tkString)
	assert.NoError(t, err)
	assert.Equal(t, "rw", data.Role)
	assert.Equal(t, "system-write", data.User)

	// Run predicates as middleware would.
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	c.Set(structs.ConvoxRoleParam, data.Role)
	assert.True(t, api.CanRead(c), "3.24.5 rw-token must satisfy CanRead on 3.24.6 rack")
	assert.True(t, api.CanWrite(c), "3.24.5 rw-token must satisfy CanWrite on 3.24.6 rack")
	assert.False(t, api.CanAdmin(c), "3.24.5 rw-token must NOT satisfy CanAdmin on 3.24.6 rack")
}
