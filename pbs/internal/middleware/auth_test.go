package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareDisabled(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled: false,
	})

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when auth disabled, got %d", rec.Code)
	}
}

func TestAuthMiddlewareMissingKey(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:    true,
		APIKeys:    map[string]string{"valid-key": "pub1"},
		HeaderName: "X-API-Key",
	})

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing key, got %d", rec.Code)
	}
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:    true,
		APIKeys:    map[string]string{"valid-key": "pub1"},
		HeaderName: "X-API-Key",
	})

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for invalid key, got %d", rec.Code)
	}
}

func TestAuthMiddlewareValidKey(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:    true,
		APIKeys:    map[string]string{"valid-key": "pub1"},
		HeaderName: "X-API-Key",
	})

	var gotPublisherID string
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPublisherID = r.Header.Get("X-Publisher-ID")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid key, got %d", rec.Code)
	}

	if gotPublisherID != "pub1" {
		t.Errorf("expected publisher ID 'pub1', got '%s'", gotPublisherID)
	}
}

func TestAuthMiddlewareBearerToken(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:    true,
		APIKeys:    map[string]string{"bearer-token": "pub2"},
		HeaderName: "X-API-Key",
	})

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bearer-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for Bearer token, got %d", rec.Code)
	}
}

func TestAuthMiddlewareBypassPaths(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:     true,
		APIKeys:     map[string]string{"key": "pub"},
		HeaderName:  "X-API-Key",
		BypassPaths: []string{"/health", "/metrics"},
	})

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		path     string
		wantCode int
	}{
		{"/health", http.StatusOK},
		{"/health/live", http.StatusOK},
		{"/metrics", http.StatusOK},
		{"/api/test", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tt.wantCode {
			t.Errorf("path %s: expected %d, got %d", tt.path, tt.wantCode, rec.Code)
		}
	}
}

func TestAuthAddRemoveAPIKey(t *testing.T) {
	auth := NewAuth(&AuthConfig{
		Enabled:    true,
		APIKeys:    map[string]string{},
		HeaderName: "X-API-Key",
	})

	// Initially no keys
	_, valid := auth.GetPublisherID("new-key")
	if valid {
		t.Error("expected key to be invalid before adding")
	}

	// Add key
	auth.AddAPIKey("new-key", "pub-new")

	pubID, valid := auth.GetPublisherID("new-key")
	if !valid {
		t.Error("expected key to be valid after adding")
	}
	if pubID != "pub-new" {
		t.Errorf("expected pub-new, got %s", pubID)
	}

	// Remove key
	auth.RemoveAPIKey("new-key")

	_, valid = auth.GetPublisherID("new-key")
	if valid {
		t.Error("expected key to be invalid after removing")
	}
}

func TestParseAPIKeys(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{"", map[string]string{}},
		{"key1:pub1", map[string]string{"key1": "pub1"}},
		{"key1:pub1,key2:pub2", map[string]string{"key1": "pub1", "key2": "pub2"}},
		{"key1", map[string]string{"key1": "default"}},
		{" key1 : pub1 , key2 : pub2 ", map[string]string{"key1": "pub1", "key2": "pub2"}},
	}

	for _, tt := range tests {
		result := parseAPIKeys(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("input %q: expected %d keys, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for k, v := range tt.expected {
			if result[k] != v {
				t.Errorf("input %q: expected %s=%s, got %s", tt.input, k, v, result[k])
			}
		}
	}
}
