package tools

import (
	"testing"

	"github.com/golang-jwt/jwt"
)

func TestJWTRequiresConfiguredSecret(t *testing.T) {
	t.Setenv("GOFLY_JWT_SECRET", "")
	if _, err := MakeToken(map[string]interface{}{"kefu_name": "agent"}); err == nil {
		t.Fatal("MakeToken should reject an empty secret")
	}
	t.Setenv("GOFLY_JWT_SECRET", "too-short")
	if _, err := MakeToken(map[string]interface{}{"kefu_name": "agent"}); err == nil {
		t.Fatal("MakeToken should reject a short secret")
	}
}

func TestJWTRoundTripAndAlgorithmRestriction(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("GOFLY_JWT_SECRET", secret)
	signed, err := MakeToken(map[string]interface{}{"kefu_name": "agent"})
	if err != nil {
		t.Fatal(err)
	}
	claims := ParseToken(signed)
	if claims == nil || claims["kefu_name"] != "agent" {
		t.Fatal("valid HS256 token was not accepted")
	}

	otherMethod := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{"kefu_name": "agent"})
	otherSigned, err := otherMethod.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if ParseToken(otherSigned) != nil {
		t.Fatal("non-HS256 token should be rejected")
	}

	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"kefu_name": "agent"})
	noneSigned, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if ParseToken(noneSigned) != nil {
		t.Fatal("unsigned token should be rejected")
	}
}
