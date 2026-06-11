package routemw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"net/http"
	"net/http/httptest"
	"testing"
)

func didPanic(f func()) bool {
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		f()
	}()
	return panicked
}

func setupTest(roles []string, requiredRoles ...string) (called *bool, w *httptest.ResponseRecorder, r *http.Request, h router.Handler) {
	var c bool
	handler := func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		c = true
		w.WriteHeader(http.StatusOK)
	}
	verifier := &fakeVerifier{claims: &reqctx.Claims{Roles: roles}}
	h = Auth(verifier)(RequireAnyRole(requiredRoles...)(handler))
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer token")
	return &c, w, r, h
}

func TestRBAC_Match(t *testing.T) {
	called, w, r, h := setupTest([]string{"admin"}, "admin")

	h(w, r, nil)

	if !*called {
		t.Error("auth fail")
	}

	if w.Code != http.StatusOK {
		t.Error("unexpected status code:", w.Code)
	}
}

func TestRBAC_RoleMismatch(t *testing.T) {
	called, w, r, h := setupTest([]string{"user"}, "admin")

	h(w, r, nil)

	if *called {
		t.Error("handler should not be called when roles do not match")
	}

	if w.Code != http.StatusForbidden {
		t.Error("got status code", w.Code, "want", http.StatusForbidden)
	}
}

func TestRBAC_MultiRole(t *testing.T) {
	called, w, r, h := setupTest([]string{"user"}, "admin", "user")

	h(w, r, nil)
	if !*called {
		t.Error("auth fail")
	}

	if w.Code != http.StatusOK {
		t.Error("unexpected status code:", w.Code)
	}
}

func TestRBAC_EmptyRole(t *testing.T) {
	called, w, r, h := setupTest(nil, "admin")

	h(w, r, nil)

	if *called {
		t.Error("handler should not be called when role is empty")
	}

	if w.Code != http.StatusForbidden {
		t.Error("got status code", w.Code, "want", http.StatusForbidden)
	}
}

func TestRBAC_NoAuth(t *testing.T) {
	c := false
	handler := func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		c = true
		w.WriteHeader(http.StatusOK)
	}
	h := RequireAnyRole("admin")(handler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer token")
	h(w, r, nil)

	if c {
		t.Error("handler should not be called when claims missing from context")
	}

	if w.Code != http.StatusForbidden {
		t.Error("got status code", w.Code, "want", http.StatusForbidden)
	}
}

func TestRBAC_EmptyCall(t *testing.T) {
	panicked := didPanic(func() {
		_ = RequireAnyRole()
	})

	if !panicked {
		t.Error("RequireAnyRole should panic if no roles are provided")
	}
}
