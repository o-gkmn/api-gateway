package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestTrailingSlash(t *testing.T) {
	r := NewRouter()
	r.GET("/users", func(w http.ResponseWriter, req *http.Request, p *Params) {})

	// Slash ile gelen istek
	req := httptest.NewRequest("GET", "/users/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("trailing slash: got %d, want 200", w.Code)
	}
}

func TestCaseInsensitive(t *testing.T) {
	r := NewRouter()
	r.CaseInsensitive()
	r.GET("/Users", func(w http.ResponseWriter, req *http.Request, p *Params) {})

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("case insensitive: got %d, want 200", w.Code)
	}
}

func TestCaseSensitive_Default(t *testing.T) {
	r := NewRouter()
	// CaseInsensitive() çağrılmadı — default strict
	r.GET("/Users", func(w http.ResponseWriter, req *http.Request, p *Params) {})

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("case sensitive default: got %d, want 404", w.Code)
	}
}

func TestRootPath(t *testing.T) {
	r := NewRouter()
	r.GET("/", func(w http.ResponseWriter, req *http.Request, p *Params) {})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("root path: got %d, want 200", w.Code)
	}
}

func BenchmarkComparisonMine_Static(b *testing.B) {
	r := NewRouter()
	r.GET("/users", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonHttprouter_Static(b *testing.B) {
	r := httprouter.New()
	r.GET("/users", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonMine_SingleParam(b *testing.B) {
	r := NewRouter()
	r.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonHttprouter_SingleParam(b *testing.B) {
	r := httprouter.New()
	r.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonMine_MultiParam(b *testing.B) {
	r := NewRouter()
	r.GET("/users/:id/order/:orderId", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users/1/order/99", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonHttprouter_MultiParam(b *testing.B) {
	r := httprouter.New()
	r.GET("/users/:id/order/:orderId", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonMine_Wildcard(b *testing.B) {
	r := NewRouter()
	r.GET("/static/*filepath", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/static/css/dark.css", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonHttprouter_Wildcard(b *testing.B) {
	r := httprouter.New()
	r.GET("/static/*filepath", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/static/css/dark.css", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonMine_StaticDeep(b *testing.B) {
	r := NewRouter()
	r.GET("/api/v1/users/profile/settings", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/profile/settings", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkComparisonHttprouter_StaticDeep(b *testing.B) {
	r := NewRouter()
	r.GET("/api/v1/users/profile/settings", func(w http.ResponseWriter, r *http.Request, params *Params) {})

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/profile/settings", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
