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
	// Note: Using StrictMode=false to allow valid format without purpose consent check
	config := DefaultPrivacyConfig()
	config.StrictMode = false // Don't require specific purpose consents
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

func TestPrivacyMiddleware_CCPAOptOut(t *testing.T) {
	// When CCPA enforcement is enabled and user opts out, request should be blocked
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-1",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YYN", // User has opted out (Y in position 3)
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called when user opts out under CCPA")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "CCPA" {
		t.Errorf("Expected regulation=CCPA, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_CCPANoOptOut(t *testing.T) {
	// When CCPA enforcement is enabled but user doesn't opt out, request should pass
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-2",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YNN", // User has NOT opted out (N in position 3)
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when user doesn't opt out")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_CCPADisabled(t *testing.T) {
	// When CCPA enforcement is disabled, opt-out should be ignored
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = false
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-3",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YYN", // User has opted out, but enforcement is disabled
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when CCPA enforcement is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestParseTCFv2String(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	// Valid TCF v2 consent string (base64url encoded)
	// This is a minimal valid consent string
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	data, err := m.parseTCFv2String(validConsent)
	if err != nil {
		t.Fatalf("parseTCFv2String failed: %v", err)
	}

	// Check that version is 2
	if data.Version != 2 {
		t.Errorf("Expected version 2, got %d", data.Version)
	}

	// Check that purpose consents slice is populated
	if len(data.PurposeConsents) == 0 {
		t.Error("Expected purpose consents to be populated")
	}
}

func TestCheckPurposeConsents(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	// Create test TCF data with some purposes granted
	data := &TCFv2Data{
		Version:         2,
		PurposeConsents: make([]bool, 24),
	}

	// Grant purposes 1, 2, 7 (required for programmatic ads)
	data.PurposeConsents[0] = true // Purpose 1
	data.PurposeConsents[1] = true // Purpose 2
	data.PurposeConsents[6] = true // Purpose 7

	// Check required purposes
	missing := m.checkPurposeConsents(data, RequiredPurposes)
	if len(missing) != 0 {
		t.Errorf("Expected no missing purposes, got %v", missing)
	}

	// Remove purpose 2 consent
	data.PurposeConsents[1] = false
	missing = m.checkPurposeConsents(data, RequiredPurposes)
	if len(missing) != 1 || missing[0] != 2 {
		t.Errorf("Expected missing purpose 2, got %v", missing)
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		defaultVal bool
		expected   bool
	}{
		{"empty returns default true", "", true, true},
		{"empty returns default false", "", false, false},
		{"true string", "true", false, true},
		{"TRUE string", "TRUE", false, true},
		{"1 string", "1", false, true},
		{"false string", "false", true, false},
		{"0 string", "0", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			key := "TEST_ENV_BOOL_" + tt.name
			if tt.envValue != "" {
				t.Setenv(key, tt.envValue)
			}

			result := getEnvBool(key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}
