package auth

import (
	"api-gateway/pkg/paseto"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPasetoVerifier_Verify(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"
	subject := "user_1"
	roles := []string{"admin"}

	v, token, _ := setupPasetoVerifier(t, issuer, audience)

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}

	if claims.Iss != v.issuer {
		t.Errorf("expected issuer %s, got %s", v.issuer, claims.Iss)
	}

	if claims.Aud != v.audience {
		t.Errorf("expected audience %s, got %s", v.audience, claims.Aud)
	}

	if claims.Sub != subject {
		t.Errorf("expected subject %s, got %s", subject, claims.Sub)
	}

	if len(claims.Roles) != len(roles) || claims.Roles[0] != "admin" {
		t.Errorf("expected roles %s, got %s", roles, claims.Roles)
	}
}

func TestPasetoVerifier_TamperedPayload(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"
	v, token, _ := setupPasetoVerifier(t, issuer, audience)

	tokenParts := strings.Split(token, ".")
	body := tokenParts[2]

	rawBody, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}

	rawBody[0] ^= 0xFF
	tamperedBody := base64.RawURLEncoding.EncodeToString(rawBody)
	tamperedToken := strings.Join([]string{tokenParts[0], tokenParts[1], tamperedBody}, ".")

	_, err = v.Verify(context.Background(), tamperedToken)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_TamperedSignature(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"
	v, token, _ := setupPasetoVerifier(t, issuer, audience)

	tokenParts := strings.Split(token, ".")
	body := tokenParts[2]

	rawBody, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}

	rawBody[len(rawBody)-1] ^= 0xFF
	tamperedBody := base64.RawURLEncoding.EncodeToString(rawBody)
	tamperedToken := strings.Join([]string{tokenParts[0], tokenParts[1], tamperedBody}, ".")

	_, err = v.Verify(context.Background(), tamperedToken)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_WrongPublicKey(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	claims := map[string]any{
		"sub":   "user_1",
		"iss":   "https://auth.local.test",
		"aud":   "api-gateway",
		"roles": []string{"admin"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}

	footer := []byte{}
	implicit := []byte{}

	token := paseto.SignV4Public(privKey, payload, footer, implicit)

	v, _, _ := setupPasetoVerifier(t, claims["iss"].(string), claims["aud"].(string))

	_, err = v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_WrongHeader(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"

	v, token, _ := setupPasetoVerifier(t, issuer, audience)
	tokenParts := strings.Split(token, ".")
	tokenParts[0] = "v3"

	token = strings.Join(tokenParts, ".")

	_, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_ExpiredToken(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"

	v, token, clock := setupPasetoVerifier(t, issuer, audience)

	tokenExpireDuration := time.Hour * 24 * 365 * 4

	clock.Advance(tokenExpireDuration)

	_, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_InvalidIssuer(t *testing.T) {
	v, token, _ := setupPasetoVerifier(t, "issuer", "api-gateway")

	_, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_InvalidAudience(t *testing.T) {
	v, token, _ := setupPasetoVerifier(t, "https://auth.local.test", "audience")
	_, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestPasetoVerifier_MalformedToken(t *testing.T) {
	issuer := "https://auth.local.test"
	audience := "api-gateway"
	v, token, _ := setupPasetoVerifier(t, issuer, audience)

	tokenParts := strings.Split(token, ".")
	tokenParts[2] = tokenParts[2] + "d"
	token = strings.Join(tokenParts, ".")

	_, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error, got none")
	}
}
