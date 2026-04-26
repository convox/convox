package jwt_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	golangjwt "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestJwtReadToken(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.ReadToken(time.Hour)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk)
	assert.NoError(t, err)
	assert.Equal(t, data.Role, structs.ConvoxRoleRead)
}

func TestJwtReadTokenExpired(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.ReadToken(time.Hour * -1)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestJwtReadTokenInvalid(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.ReadToken(time.Hour * -1)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk[:len(tk)-1])
	fmt.Println(err)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestJwtWriteToken(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.WriteToken(time.Hour)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk)
	assert.NoError(t, err)
	assert.Equal(t, data.Role, structs.ConvoxRoleReadWrite)
}

func TestJwtAdminToken(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.AdminToken(time.Hour)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk)
	assert.NoError(t, err)
	assert.Equal(t, data.Role, structs.ConvoxRoleAdmin)
	assert.Equal(t, data.User, "system-admin")
}

func TestJwtAdminTokenExpired(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.AdminToken(time.Hour * -1)
	assert.NoError(t, err, "no error")

	data, err := jm.Verify(tk)
	assert.Error(t, err)
	assert.Nil(t, data)
}

// TestAdminTokenContainsRWSubstrings_LiveMint actually mints an Admin token via
// JwtMngr.AdminToken and verifies the role-string contains both "r" and "w"
// substrings. Twin guard for catching a future PR that overrides the role
// mid-flight in AdminToken (e.g. accidentally hard-codes "a") without touching
// the const.
func TestAdminTokenContainsRWSubstrings_LiveMint(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	tk, err := jm.AdminToken(time.Hour)
	assert.NoError(t, err)

	data, err := jm.Verify(tk)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(data.Role, "r"), "Admin role must contain \"r\" for 3.24.5 rollback safety")
	assert.True(t, strings.Contains(data.Role, "w"), "Admin role must contain \"w\" for 3.24.5 rollback safety")
}


// TestVerify_ClaimsMissing_ReturnsErr — F-24 fix (catalog F-24).
// The Verify path used to do unguarded type assertions on jwt.MapClaims,
// which would panic the api pod if a malformed token (missing claim) hit
// the path. Now every assertion is `, ok := ...` and returns an error.
func TestVerify_ClaimsMissing_ReturnsErr(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	// Mint a token via the golang-jwt library directly with NO claims at
	// all so type assertions fail cleanly. (Cannot exercise this path
	// through ReadToken/WriteToken/AdminToken because they all populate
	// the user/role/expiresAt fields.)
	tok := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{})
	signed, err := tok.SignedString([]byte("TEST"))
	assert.NoError(t, err)

	data, verr := jm.Verify(signed)
	assert.Error(t, verr, "missing claims must surface error, not panic")
	assert.Nil(t, data, "data must be nil on error")
}

// TestVerify_ClaimsWrongType_ReturnsErr — F-24 fix.
// Tests the user/role-as-non-string path explicitly: the golang-jwt
// library accepts arbitrary value types in MapClaims, so a malformed
// token can present role=123 (int) where the code expects role=string.
// The unguarded path used to panic; the guarded path returns an error.
func TestVerify_ClaimsWrongType_ReturnsErr(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")

	// user is int (not string).
	tok := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
		"user":      123,
		"role":      "rw",
		"expiresAt": float64(time.Now().Add(time.Hour).Unix()),
	})
	signed, err := tok.SignedString([]byte("TEST"))
	assert.NoError(t, err)

	data, verr := jm.Verify(signed)
	assert.Error(t, verr, "wrong-type user claim must surface error")
	assert.Nil(t, data)

	// role is int (not string).
	tok2 := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
		"user":      "user@example.com",
		"role":      42,
		"expiresAt": float64(time.Now().Add(time.Hour).Unix()),
	})
	signed2, err := tok2.SignedString([]byte("TEST"))
	assert.NoError(t, err)

	data2, verr2 := jm.Verify(signed2)
	assert.Error(t, verr2, "wrong-type role claim must surface error")
	assert.Nil(t, data2)

	// expiresAt is string (not float64).
	tok3 := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
		"user":      "user@example.com",
		"role":      "rw",
		"expiresAt": "not-a-number",
	})
	signed3, err := tok3.SignedString([]byte("TEST"))
	assert.NoError(t, err)

	data3, verr3 := jm.Verify(signed3)
	assert.Error(t, verr3, "wrong-type expiresAt claim must surface error")
	assert.Nil(t, data3)
}
