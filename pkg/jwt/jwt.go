package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JwtManager struct {
	signKey string
}

func New(signKey string) *JwtManager {
	return &JwtManager{
		signKey: signKey,
	}
}

func (j *JwtManager) ReadToken(duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"foo":       "bar",
		"expiresAt": time.Now(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(hmacSampleSecret)
}
