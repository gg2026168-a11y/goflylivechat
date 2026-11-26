package tools

import (
	"errors"
	"os"
	"strings"

	"github.com/golang-jwt/jwt"
)

func jwtSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("GOFLY_JWT_SECRET"))
	if len(secret) < 32 {
		return nil, errors.New("GOFLY_JWT_SECRET must contain at least 32 characters")
	}
	return []byte(secret), nil
}

func MakeToken(obj map[string]interface{}) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(obj))
	return token.SignedString(secret)
}

func ParseToken(tokenStr string) map[string]interface{} {
	secret, err := jwtSecret()
	if err != nil {
		return nil
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}
	return claims
}
