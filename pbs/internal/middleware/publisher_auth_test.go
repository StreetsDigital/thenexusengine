package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublisherAuth_Disabled(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled: false,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called when auth is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPublisherAuth_NonAuctionEndpoint(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled: true,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Non-auction endpoint should pass through
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called for non-auction endpoints")
	}
}

func TestPublisherAuth_MissingPublisher(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request without publisher
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if called {
		t.Error("Handler should NOT have been called without publisher when AllowUnregistered=false")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestPublisherAuth_AllowUnregistered(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: true,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request without publisher
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called when AllowUnregistered=true")
	}
}

func TestPublisherAuth_RegisteredPublisher(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
		RegisteredPubs:    map[string]string{"pub123": "example.com"},
		ValidateDomain:    false, // Don't validate domain for this test
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request with registered publisher
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
		"site": map[string]interface{}{
			"domain": "example.com",
			"publisher": map[string]interface{}{
				"id": "pub123",
			},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called for registered publisher")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPublisherAuth_UnregisteredPublisher(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
		RegisteredPubs:    map[string]string{"pub123": "example.com"},
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request with unregistered publisher
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
		"site": map[string]interface{}{
			"domain": "example.com",
			"publisher": map[string]interface{}{
				"id": "unknown_pub", // Not in RegisteredPubs
			},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if called {
		t.Error("Handler should NOT have been called for unregistered publisher")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestPublisherAuth_DomainValidation(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
		RegisteredPubs:    map[string]string{"pub123": "allowed.com"},
		ValidateDomain:    true,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request with wrong domain
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
		"site": map[string]interface{}{
			"domain": "wrong.com", // Not in allowed domains
			"publisher": map[string]interface{}{
				"id": "pub123",
			},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if called {
		t.Error("Handler should NOT have been called for wrong domain")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestPublisherAuth_DomainValidation_Allowed(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
		RegisteredPubs:    map[string]string{"pub123": "allowed.com"},
		ValidateDomain:    true,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request with correct domain
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
		"site": map[string]interface{}{
			"domain": "allowed.com",
			"publisher": map[string]interface{}{
				"id": "pub123",
			},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called for allowed domain")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPublisherAuth_AppPublisher(t *testing.T) {
	config := &PublisherAuthConfig{
		Enabled:           true,
		AllowUnregistered: false,
		RegisteredPubs:    map[string]string{"app_pub": ""},
		ValidateDomain:    false,
	}
	auth := NewPublisherAuth(config)

	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request with app publisher
	bidReq := map[string]interface{}{
		"id": "test-1",
		"imp": []map[string]interface{}{
			{"id": "imp1", "banner": map[string]interface{}{}},
		},
		"app": map[string]interface{}{
			"bundle": "com.example.app",
			"publisher": map[string]interface{}{
				"id": "app_pub",
			},
		},
	}
	body, _ := json.Marshal(bidReq)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should have been called for registered app publisher")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestParsePublishers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "single with domain",
			input:    "pub1:example.com",
			expected: map[string]string{"pub1": "example.com"},
		},
		{
			name:     "single without domain",
			input:    "pub1",
			expected: map[string]string{"pub1": ""},
		},
		{
			name:  "multiple",
			input: "pub1:example.com,pub2:other.com",
			expected: map[string]string{
				"pub1": "example.com",
				"pub2": "other.com",
			},
		},
		{
			name:  "mixed with and without domains",
			input: "pub1:example.com,pub2,pub3:test.com",
			expected: map[string]string{
				"pub1": "example.com",
				"pub2": "",
				"pub3": "test.com",
			},
		},
		{
			name:  "with spaces",
			input: " pub1 : example.com , pub2 : other.com ",
			expected: map[string]string{
				"pub1": "example.com",
				"pub2": "other.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePublishers(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parsePublishers(%q) returned %d items, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("parsePublishers(%q)[%s] = %q, want %q", tt.input, k, result[k], v)
				}
			}
		})
	}
}
