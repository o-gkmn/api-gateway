package router

import (
	"net/http"
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

func TestTree_Insert(t *testing.T) {
	cases := []struct {
		name         string
		insertMethod methodIndex
		insertPath   string
		matchMethod  methodIndex
		matchPath    string
		expectStatus int
		expectPanic  bool
	}{
		{
			name:         "static route",
			insertMethod: methodGET,
			insertPath:   "/users",
			matchMethod:  methodGET,
			matchPath:    "/users",
			expectStatus: http.StatusOK,
		},
		{
			name:         "single param",
			insertMethod: methodGET,
			insertPath:   "/users/:id",
			matchMethod:  methodGET,
			matchPath:    "/users/42",
			expectStatus: http.StatusOK,
		},
		{
			name:         "multiple params",
			insertMethod: methodGET,
			insertPath:   "/users/:id/orders/:orderId",
			matchMethod:  methodGET,
			matchPath:    "/users/1/orders/99",
			expectStatus: http.StatusOK,
		},
		{
			name:         "wildcard path",
			insertMethod: methodPOST,
			insertPath:   "/files/*filepath",
			matchMethod:  methodPOST,
			matchPath:    "/files/images/logo.png",
			expectStatus: http.StatusOK,
		},
		{
			name:         "shared prefix",
			insertMethod: methodPOST,
			insertPath:   "/upload",
			matchMethod:  methodPOST,
			matchPath:    "/upload",
			expectStatus: http.StatusOK,
		},
		{
			name:         "multiple methods same path",
			insertMethod: methodPOST,
			insertPath:   "/users",
			matchMethod:  methodPOST,
			matchPath:    "/users",
			expectStatus: http.StatusOK,
		},
		{
			name:         "wildcard not at end",
			insertMethod: methodPOST,
			insertPath:   "/users/*filepath/file",
			expectPanic:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := NewTree()
			handler := func(w http.ResponseWriter, r *http.Request) {}

			panicked := didPanic(func() {
				tree.Insert(tc.insertMethod, tc.insertPath, handler)
			})

			if tc.expectPanic {
				if !panicked {
					t.Errorf("expected a panic but did not get one")
				}
				return
			}

			if panicked {
				t.Fatalf("unexpected panic during Insert")
			}

			result := tree.Match(tc.matchMethod, tc.matchPath)

			if result.status != tc.expectStatus {
				t.Errorf("got status %d, want %d", result.status, tc.expectStatus)
			}

			if tc.expectStatus == http.StatusOK && result.handler == nil {
				t.Errorf("expected a handler but got nil")
			}
		})
	}
}

func TestTree_Insert_DuplicateRoute(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	tree.Insert(methodGET, "/users", handler)

	panicked := didPanic(func() {
		tree.Insert(methodGET, "/users", handler)
	})

	if !panicked {
		t.Errorf("expected a panic for duplicate route but did not get one")
	}
}

func TestTree_Insert_ConflictingParamName(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	tree.Insert(methodGET, "/users/:id", handler)

	panicked := didPanic(func() {
		tree.Insert(methodGET, "/users/:name", handler)
	})

	if !panicked {
		t.Errorf("expected a panic for conflicting param name but did not get one")
	}
}

func TestMatch_ParamCapture(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}

	tree.Insert(methodGET, "/users/:id/orders/:orderId", handler)

	result := tree.Match(methodGET, "/users/42/orders/99")

	if result.status != http.StatusOK {
		t.Errorf("got status %d, want %d", result.status, http.StatusOK)
	}

	if result.handler == nil {
		t.Errorf("expected a handler but got nil")
	}

	if result.params.count != 2 {
		t.Errorf("got params count %d, want 2", result.params.count)
	}

	if result.params.Get("id") != "42" {
		t.Errorf("got param id %s, want 42", result.params.Get("id"))
	}

	if result.params.Get("orderId") != "99" {
		t.Errorf("got order id %s, want %s", result.params.Get("orderId"), "99")
	}
}

func TestMatch_SameParamNameCapture(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}

	panicked := didPanic(func() {
		tree.Insert(methodGET, "/users/:id/orders/:id", handler)
	})

	if !panicked {
		t.Errorf("expected a panic for same param name but did not get one")
	}
}

func TestMatch_Backtracking(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}

	tree.Insert(methodGET, "/users/profile", handler)
	tree.Insert(methodGET, "/users/:id/posts", handler)

	result := tree.Match(methodGET, "/users/profile/posts")

	if result.status != http.StatusOK {
		t.Errorf("got status %d, want %d", result.status, http.StatusOK)
	}

	if result.handler == nil {
		t.Errorf("expected a handler but got nil")
	}

	if result.params.count != 1 {
		t.Errorf("got params count %d, want 1", result.params.count)
	}

	if result.params.Get("id") != "profile" {
		t.Errorf("got param id %s, want profile", result.params.Get("id"))
	}
}

func TestMatch_404(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}

	tree.Insert(methodGET, "/users/posts", handler)

	result := tree.Match(methodGET, "/unknwon")

	if result.status != http.StatusNotFound {
		t.Errorf("got status %d, want %d", result.status, http.StatusNotFound)
	}
}

func TestMatch_405(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	tree.Insert(methodGET, "/users/posts", handler)

	result := tree.Match(methodPOST, "/users/posts")
	if result.status != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", result.status, http.StatusMethodNotAllowed)
	}

	if len(result.allowedMethods) == 0 {
		t.Fatalf("allowedMethods is empty")
	}
	if result.allowedMethods[0] != methodNames[methodGET] {
		t.Errorf("got allowedMethods[0] = %s, want GET", result.allowedMethods[0])
	}
}

func TestInsert_StaticAndParamCoexist(t *testing.T) {
	tree := NewTree()
	handler := func(w http.ResponseWriter, r *http.Request) {}

	tree.Insert(methodGET, "/users/profile", handler)
	tree.Insert(methodGET, "/users/:id", handler)

	r1 := tree.Match(methodGET, "/users/profile")
	if r1.status != http.StatusOK {
		t.Errorf("static path not matched")
	}

	r2 := tree.Match(methodGET, "/users/42")
	if r2.status != http.StatusOK {
		t.Errorf("param path not matched")
	}
	if r2.params.entries[0].value != "42" {
		t.Errorf("param value = %q, want 42", r2.params.entries[0].value)
	}
}
