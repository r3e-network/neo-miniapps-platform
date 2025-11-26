package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithMethod(t *testing.T) {
	handler := withMethod(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Correct method
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET, got %d", rec.Code)
	}

	// Wrong method
	req = httptest.NewRequest(http.MethodPost, "/test", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("expected Allow header to be GET")
	}
}

func TestWithMethodDifferentMethods(t *testing.T) {
	tests := []struct {
		allowed   string
		request   string
		expectOK  bool
	}{
		{http.MethodGet, http.MethodGet, true},
		{http.MethodPost, http.MethodPost, true},
		{http.MethodPut, http.MethodPut, true},
		{http.MethodDelete, http.MethodDelete, true},
		{http.MethodPatch, http.MethodPatch, true},
		{http.MethodGet, http.MethodPost, false},
		{http.MethodPost, http.MethodGet, false},
		{http.MethodPut, http.MethodPatch, false},
	}

	for _, tt := range tests {
		handler := withMethod(tt.allowed, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(tt.request, "/test", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if tt.expectOK && rec.Code != http.StatusOK {
			t.Errorf("method %s with request %s: expected 200, got %d", tt.allowed, tt.request, rec.Code)
		}
		if !tt.expectOK && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s with request %s: expected 405, got %d", tt.allowed, tt.request, rec.Code)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	// No methods specified
	rec := httptest.NewRecorder()
	methodNotAllowed(rec)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != "" {
		t.Fatalf("expected no Allow header")
	}

	// Single method
	rec = httptest.NewRecorder()
	methodNotAllowed(rec, http.MethodGet)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != "GET" {
		t.Fatalf("expected Allow: GET, got %s", rec.Header().Get("Allow"))
	}

	// Multiple methods
	rec = httptest.NewRecorder()
	methodNotAllowed(rec, http.MethodGet, http.MethodPost, http.MethodPut)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != "GET, POST, PUT" {
		t.Fatalf("expected Allow: GET, POST, PUT, got %s", rec.Header().Get("Allow"))
	}
}

func TestWithMethodChaining(t *testing.T) {
	// Test that handler is properly chained
	callCount := 0
	handler := withMethod(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusCreated)
	})

	// Should call handler
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if callCount != 1 {
		t.Fatalf("handler should be called once, called %d times", callCount)
	}

	// Should not call handler for wrong method
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if callCount != 1 {
		t.Fatalf("handler should not be called for wrong method, called %d times", callCount)
	}
}
