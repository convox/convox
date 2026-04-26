package jwt

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/golang-jwt/jwt/v4"
)

type TokenData struct {
	User      string
	Role      string
	ExpiresAt time.Time
}

type JwtManager struct {
	signKey []byte
}

func NewJwtManager(signKey string) *JwtManager {
	return &JwtManager{
		signKey: []byte(signKey),
	}
}

func (j *JwtManager) ReadToken(duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-read",
		"role":      structs.ConvoxRoleRead,
		"expiresAt": time.Now().UTC().Add(duration).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(j.signKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (j *JwtManager) WriteToken(duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-write",
		"role":      structs.ConvoxRoleReadWrite,
		"expiresAt": time.Now().UTC().Add(duration).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(j.signKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (j *JwtManager) AdminToken(duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      "system-admin",
		"role":      structs.ConvoxRoleAdmin,
		"expiresAt": time.Now().UTC().Add(duration).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(j.signKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (j *JwtManager) Verify(token string) (*TokenData, error) {
	d := &TokenData{}
	tk, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return j.signKey, nil
	})
	if err != nil {
		return nil, err
	}

	if !tk.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := tk.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}

	// F-24 fix: explicit type-assertion guards on every claim. Without
	// these, a malformed claim (string where float expected, missing
	// claim, etc.) panics the api pod. Returning a plain error preserves
	// the existing failure semantics for legitimate calls and gives
	// downstream callers a single error path to handle.
	user, ok := claims["user"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid token: user claim missing or wrong type")
	}
	role, ok := claims["role"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid token: role claim missing or wrong type")
	}
	expiresAtRaw, ok := claims["expiresAt"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid token: expiresAt claim missing or wrong type")
	}
	d.User = user
	d.Role = role
	d.ExpiresAt = time.Unix(int64(expiresAtRaw), 0)
	if d.ExpiresAt.UTC().Before(time.Now().UTC()) {
		return nil, fmt.Errorf("token is expired")
	}
	return d, nil
}
