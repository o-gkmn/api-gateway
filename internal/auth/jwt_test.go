package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signRS256(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return s
}

func TestJWTVerifier_Verify(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}
	v := NewJWTVerifier(&priv.PublicKey, "iss-x", "aud-x")

	good := signRS256(t, priv, jwt.MapClaims{
		"sub":   "user_1",
		"iss":   "iss-x",
		"aud":   "aud-x",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": []string{"admin"},
	})
	claims, err := v.Verify(context.Background(), good)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Subject != "user_1" || len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}

	expired := signRS256(t, priv, jwt.MapClaims{
		"sub": "user_1", "iss": "iss-x", "aud": "aud-x",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	if _, err := v.Verify(context.Background(), expired); err == nil {
		t.Error("süresi geçmiş token reddedilmeliydi")
	}
}
