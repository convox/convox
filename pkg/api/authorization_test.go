package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/api"
	cjwt "github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestAuthorize(t *testing.T) {
	s := &api.Server{}

	testData := []struct {
		c      *stdapi.Context
		access bool
	}{
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
				api.SetReadRole(c)
				return c
			}(),
			access: true,
		},
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
				return c
			}(),
			access: false,
		},
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodPost, "http://text.com", nil))
				api.SetReadRole(c)
				return c
			}(),
			access: false,
		},
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodPost, "http://text.com", nil))
				api.SetReadWriteRole(c)
				return c
			}(),
			access: true,
		},
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
				api.SetAdminRole(c)
				return c
			}(),
			access: true,
		},
		{
			c: func() *stdapi.Context {
				c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodPost, "http://text.com", nil))
				api.SetAdminRole(c)
				return c
			}(),
			access: true,
		},
	}

	for _, td := range testData {
		err := s.Authorize(func(c *stdapi.Context) error {
			return nil
		})(td.c)
		if td.access {
			assert.Nil(t, err)
		} else {
			assert.NotNil(t, err)
		}
	}
}

// TestCanAdminTrueForAdminRole verifies that the Admin role-string ("rwa") satisfies
// CanAdmin AND CanRead AND CanWrite (substring superset rollback safety).
func TestCanAdminTrueForAdminRole(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	api.SetAdminRole(c)

	assert.True(t, api.CanAdmin(c), "Admin role must satisfy CanAdmin")
	assert.True(t, api.CanRead(c), "Admin role must satisfy CanRead (substring superset)")
	assert.True(t, api.CanWrite(c), "Admin role must satisfy CanWrite (substring superset)")
}

// TestCanAdminFalseForReadAndWriteRoles verifies that legacy roles "r" and "rw"
// do NOT satisfy CanAdmin (no "a" substring).
func TestCanAdminFalseForReadAndWriteRoles(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	api.SetReadRole(c)
	assert.False(t, api.CanAdmin(c), "Read role must NOT satisfy CanAdmin")

	c2 := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	api.SetReadWriteRole(c2)
	assert.False(t, api.CanAdmin(c2), "ReadWrite role must NOT satisfy CanAdmin")
}

// TestCanAdminFalseWhenRoleAbsent verifies CanAdmin returns false when the
// role param is unset.
func TestCanAdminFalseWhenRoleAbsent(t *testing.T) {
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	assert.False(t, api.CanAdmin(c), "absent role must NOT satisfy CanAdmin")
}

// TestVerifyTokenWithUnknownRoleChars_DoesNotCrash mints a token claim with
// role-string "rwxq" via direct jwt.NewWithClaims (bypassing the production
// issuer), feeds it through Verify, and asserts no panic; CanRead returns true
// (substring "r"), CanWrite returns true (substring "w"), CanAdmin returns
// false (no "a"). Pre-validates the future "rwax" extension scheme survives
// unknown-role chars.
func TestVerifyTokenWithUnknownRoleChars_DoesNotCrash(t *testing.T) {
	signKey := []byte("TEST")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "future-system",
		"role":      "rwxq",
		"expiresAt": time.Now().UTC().Add(time.Hour).Unix(),
	})
	tkString, err := tok.SignedString(signKey)
	assert.NoError(t, err)

	// Run Verify via the same JwtManager.
	jm := newJwtManagerFromTestKey(t)
	data, err := jm.Verify(tkString)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, "rwxq", data.Role)

	// Construct a stdapi.Context with this role and run the predicates.
	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://text.com", nil))
	c.Set(structs.ConvoxRoleParam, data.Role)

	assert.True(t, api.CanRead(c), "rwxq must satisfy CanRead via substring \"r\"")
	assert.True(t, api.CanWrite(c), "rwxq must satisfy CanWrite via substring \"w\"")
	assert.False(t, api.CanAdmin(c), "rwxq must NOT satisfy CanAdmin (no \"a\")")
}

// TestAdminTokenContainsRWSubstrings is a 3-line const-locked guard that
// asserts the literal Admin role-string (structs.ConvoxRoleAdmin) contains
// both "r" and "w" substrings. Locks the rollback substring constraint into
// source — a future PR cannot drift the const without breaking this test.
func TestAdminTokenContainsRWSubstrings(t *testing.T) {
	assert.True(t, strings.Contains(structs.ConvoxRoleAdmin, "r"), "ConvoxRoleAdmin must contain \"r\"")
	assert.True(t, strings.Contains(structs.ConvoxRoleAdmin, "w"), "ConvoxRoleAdmin must contain \"w\"")
}

// newJwtManagerFromTestKey returns a JwtManager keyed with the test signing
// key "TEST" (matching the literal []byte("TEST") used in direct
// jwt.NewWithClaims mints elsewhere in this file).
func newJwtManagerFromTestKey(t *testing.T) *cjwt.JwtManager {
	t.Helper()
	return cjwt.NewJwtManager("TEST")
}
