package auth

import (
	"testing"
	"time"
)

func TestIntrospectionVerifier_Verify(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)
	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims.Subject != "sub123" {
		t.Errorf("expected subject to be sub123, got %v", claims.Subject)
	}
}

func TestIntrospectionVerifier_Verify_Fail(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)
	resp.Active = false

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_AuthServerError(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	oauthServer.SetFailMode(true)

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_AuthHeaderCheck(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	oauthServer.SetAuthCheck(true)

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims == nil {
		t.Fatalf("expected claims, got nil")
	}
}

func TestIntrospectionVerifier_IssAudMismatch(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := NewIntrospectionVerifier(oauthServer.URL(), resp.ClientID, "sec", "iss", "aud", clock.Now)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_ExpiredToken(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)
	resp.Exp = clock.Now().Add(-1 * time.Hour).Unix()

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_NoCachingFetchesEveryTime(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	for i := 0; i < 3; i++ {
		if _, err := verifier.Verify(t.Context(), "valid-token"); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}

	if got := oauthServer.Fetches(); got != 3 {
		t.Errorf("expected 3 fetches, got %d", got)
	}
}
