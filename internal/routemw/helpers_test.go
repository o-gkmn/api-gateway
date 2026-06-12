package routemw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"context"
	"net/http"
	"net/http/httptest"
)

type fakeVerifier struct {
	claims *reqctx.Claims
	err    error
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	return f.claims, f.err
}

type nopWriter struct{ h http.Header }

func (n *nopWriter) Header() http.Header {
	if n.h == nil {
		n.h = make(http.Header)
	}
	return n.h
}
func (n *nopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopWriter) WriteHeader(int)             {}

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

type testPayload struct {
	Email   string `json:"email"`
	Pass    string `json:"pass"`
	IsValid bool   `json:"is_valid"`
}

func (t testPayload) Validate() ValidationErrors {
	if !t.IsValid {
		return ValidationErrors{
			ValidationError{
				Field:   "email",
				Message: "email is required",
			},
		}
	}

	return nil
}

type infiniteZeroReader struct{}

func (infiniteZeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
