package routemw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidate_Success(t *testing.T) {
	payload := testPayload{
		Email:   "ozgurgokmen",
		Pass:    "123456",
		IsValid: true,
	}

	called := false
	handler := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		got, ok := reqctx.GetPayload[testPayload](r.Context())
		if !ok {
			t.Error("payload not found in context")
			return
		}
		if got.Email != "ozgurgokmen" {
			t.Errorf("got email %q, want %q", got.Email, "ozgurgokmen")
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}
	h := Validate[testPayload]()(handler)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/test", &buf)
	w := httptest.NewRecorder()

	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	h(w, r, nil)

	if !called {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusOK)
	}
}

func TestValidate_Fail(t *testing.T) {
	payload := testPayload{
		Email:   "ozgurgokmen",
		Pass:    "123456",
		IsValid: false,
	}

	called := false
	handler := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	h := Validate[testPayload]()(handler)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/test", &buf)
	w := httptest.NewRecorder()

	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	h(w, r, nil)

	if called {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusBadRequest)
	}
}

func TestValidate_Invalid(t *testing.T) {
	called := false
	handler := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	h := Validate[testPayload]()(handler)

	r := httptest.NewRequest("POST", "/test", strings.NewReader("{"))
	w := httptest.NewRecorder()

	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	h(w, r, nil)

	if called {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusBadRequest)
	}
}

func TestValidate_WrongContentType(t *testing.T) {
	payload := testPayload{
		Email:   "ozgurgokmen",
		Pass:    "123456",
		IsValid: true,
	}

	called := false
	handler := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	h := Validate[testPayload]()(handler)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/test", &buf)
	w := httptest.NewRecorder()

	r.Header.Set("Content-Type", "text/plain; charset=utf-8")
	h(w, r, nil)

	if called {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusUnsupportedMediaType)
	}
}

func TestValidate_BodyTooLarge(t *testing.T) {
	called := false
	handler := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	h := Validate[testPayload]()(handler)

	jsonPart := `{"email":"ozgurgokmen","pass":"123456","isValid":true}`

	oneMegabyte := int64(1024 * 1024)
	largeGarbage := io.LimitReader(infiniteZeroReader{}, oneMegabyte+100)

	bodyStream := io.MultiReader(strings.NewReader(jsonPart), largeGarbage)

	r := httptest.NewRequest("POST", "/test", bodyStream)
	w := httptest.NewRecorder()

	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	h(w, r, nil)

	if called {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusRequestEntityTooLarge)
	}
}
