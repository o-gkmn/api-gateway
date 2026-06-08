package mw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeVerifier struct {
	claims *reqctx.Claims
	err    error
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	return f.claims, f.err
}

func TestAuth_MissingHeader(t *testing.T) {
	called := false
	h := Auth(&fakeVerifier{})(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is missing")
	}

	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header should not be empty")
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	called := false
	h := Auth(&fakeVerifier{})(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "invalid")
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is malformed")
	}

	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header should not be empty")
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	called := false
	verifier := &fakeVerifier{err: errors.New("invalid token")}
	h := Auth(verifier)(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is invalid")
	}
}

func TestAuth_ValidToken_PutsClaimsInContext(t *testing.T) {
	want := &reqctx.Claims{Subject: "test", Role: []string{"admin"}}
	var got *reqctx.Claims
	called := false

	verifier := &fakeVerifier{claims: want}
	h := Auth(verifier)(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		got, _ = reqctx.GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer test")
	h(w, r, nil)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want %d", w.Code, http.StatusOK)
	}

	if !called {
		t.Error("handler should not be called when Authorization header is valid")
	}

	if got.Subject != want.Subject {
		t.Errorf("got %s, want %s", got.Subject, want.Subject)
	}
}

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

	if claims.Subject != "user_1" || len(claims.Role) != 1 || claims.Role[0] != "admin" {
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
