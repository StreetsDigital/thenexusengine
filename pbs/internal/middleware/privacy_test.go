package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

func TestPrivacyMiddleware_NoGDPR(t *testing.T) {
	// Request without GDPR signal should pass through
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-1",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		// No Regs field - GDPR doesn't apply
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when GDPR doesn't apply")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_GDPRWithValidConsent(t *testing.T) {
	// Request with GDPR=1 and valid consent should pass through
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	// This is a real TCF v2 consent string (base64url encoded)
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	req := &openrtb.BidRequest{
		ID:  "test-2",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: validConsent,
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called with valid consent")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_GDPRNoConsent(t *testing.T) {
	// Request with GDPR=1 but no consent should be blocked
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-3",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		// No User.Consent - GDPR violation
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called without consent when GDPR applies")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	// Check error response
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "GDPR" {
		t.Errorf("Expected regulation=GDPR, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_GDPRInvalidConsent(t *testing.T) {
	// Request with GDPR=1 but invalid consent string should be blocked
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-4",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: "invalid-not-base64", // Invalid consent string
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called with invalid consent")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_COPPA(t *testing.T) {
	// COPPA requests should be blocked by default
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-5",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			COPPA: 1, // Child-directed content
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called for COPPA requests")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "COPPA" {
		t.Errorf("Expected regulation=COPPA, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_GETRequest(t *testing.T) {
	// GET requests should pass through without privacy checks
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	httpReq := httptest.NewRequest(http.MethodGet, "/openrtb2/auction", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called for GET requests")
	}
}

func TestPrivacyMiddleware_DisabledGDPR(t *testing.T) {
	// When GDPR enforcement is disabled, requests should pass through
	config := DefaultPrivacyConfig()
	config.EnforceGDPR = false
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-6",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		// No consent - would normally be blocked
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when GDPR enforcement is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestIsValidTCFv2String(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	tests := []struct {
		name    string
		consent string
		valid   bool
	}{
		{"empty", "", false},
		{"too short", "abc", false},
		{"invalid base64", "not-valid-base64-!!!", false},
		{"valid TCF v2", "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA", true},
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.isValidTCFv2String(tt.consent)
			if result != tt.valid {
				t.Errorf("isValidTCFv2String(%q) = %v, want %v", tt.consent, result, tt.valid)
			}
		})
	}
}
