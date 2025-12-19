package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	cors := NewCORS(&CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "X-API-Key"},
		MaxAge:         86400,
	})

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight request")
	}))

	req := httptest.NewRequest("OPTIONS", "/openrtb2/auction", nil)
	req.Header.Set("Origin", "https://publisher.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should return 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rr.Code)
	}

	// Check CORS headers
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://publisher.example.com" {
		t.Errorf("Expected Allow-Origin header, got %q", got)
	}

	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Expected Allow-Methods header")
	}

	if got := rr.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Expected Max-Age 86400, got %q", got)
	}
}

func TestCORSMiddleware_ActualRequest(t *testing.T) {
	cors := NewCORS(&CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST"},
		ExposedHeaders: []string{"X-Request-ID"},
	})

	handlerCalled := false
	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/openrtb2/auction", nil)
	req.Header.Set("Origin", "https://publisher.example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should be called for actual request")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check CORS headers on actual request
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://publisher.example.com" {
		t.Errorf("Expected Allow-Origin header, got %q", got)
	}

	if got := rr.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
		t.Errorf("Expected Expose-Headers, got %q", got)
	}
}

func TestCORSMiddleware_Disabled(t *testing.T) {
	cors := NewCORS(&CORSConfig{
		Enabled: false,
	})

	handlerCalled := false
	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/openrtb2/auction", nil)
	req.Header.Set("Origin", "https://publisher.example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Handler should be called when CORS is disabled
	if !handlerCalled {
		t.Error("Handler should be called when CORS is disabled")
	}

	// No CORS headers when disabled
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Expected no CORS headers when disabled, got %q", got)
	}
}

func TestCORSMiddleware_OriginRestriction(t *testing.T) {
	cors := NewCORS(&CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://allowed.com", "*.trusted.com"},
		AllowedMethods: []string{"POST"},
	})

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		origin         string
		expectAllowed  bool
	}{
		{"exact match", "https://allowed.com", true},
		{"wildcard subdomain", "https://sub.trusted.com", true},
		{"not allowed", "https://evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/openrtb2/auction", nil)
			req.Header.Set("Origin", tt.origin)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			got := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.expectAllowed && got != tt.origin {
				t.Errorf("Expected origin %q to be allowed", tt.origin)
			}
			if !tt.expectAllowed && got == tt.origin {
				t.Errorf("Expected origin %q to be blocked", tt.origin)
			}
		})
	}
}

func TestCORSMiddleware_Credentials(t *testing.T) {
	cors := NewCORS(&CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{},
		AllowCredentials: true,
	})

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/openrtb2/auction", nil)
	req.Header.Set("Origin", "https://publisher.example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Expected credentials header, got %q", got)
	}
}
