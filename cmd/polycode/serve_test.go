package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware(inner)

	// Test preflight OPTIONS from localhost origin
	req := httptest.NewRequest(http.MethodOptions, "/prompt", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS expected 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("expected localhost origin, got %q", got)
	}

	// Test normal GET from localhost gets CORS headers
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "http://127.0.0.1:9876")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:9876" {
		t.Errorf("expected 127.0.0.1 origin, got %q", got)
	}

	// Test that non-localhost origin is rejected
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for non-localhost, got %q", got)
	}

	// Test that spoofed localhost subdomain is rejected
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "http://localhost.evil.com")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for localhost.evil.com, got %q", got)
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"hello": "world"})

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Error("expected JSON content type")
	}
	if !strings.Contains(rr.Body.String(), `"hello":"world"`) {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}
