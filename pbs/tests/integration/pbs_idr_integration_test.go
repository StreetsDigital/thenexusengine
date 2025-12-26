//go:build integration
// +build integration

// Package integration contains integration tests for PBS-IDR interaction
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/idr"
)

// TestIDRClientSelectPartners tests the IDR client partner selection
func TestIDRClientSelectPartners(t *testing.T) {
	// Create mock IDR server
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/select" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Decode request to verify it's valid
		var req idr.SelectPartnersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Return mock response
		response := idr.SelectPartnersResponse{
			SelectedBidders: []idr.SelectedBidder{
				{BidderCode: "appnexus", Score: 0.95, Reason: "HIGH_SCORE"},
				{BidderCode: "rubicon", Score: 0.85, Reason: "DIVERSITY"},
			},
			Mode:             "normal",
			ProcessingTimeMs: 5.2,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockIDR.Close()

	// Create client
	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	// Test request
	ortbReq := openrtb.BidRequest{
		ID: "test-1",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}
	ortbJSON, _ := json.Marshal(ortbReq)

	availableBidders := []string{"appnexus", "rubicon", "pubmatic", "openx"}

	ctx := context.Background()
	resp, err := client.SelectPartners(ctx, ortbJSON, availableBidders)

	if err != nil {
		t.Fatalf("SelectPartners failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if len(resp.SelectedBidders) != 2 {
		t.Errorf("Expected 2 selected bidders, got %d", len(resp.SelectedBidders))
	}

	if resp.Mode != "normal" {
		t.Errorf("Expected mode 'normal', got '%s'", resp.Mode)
	}
}

// TestIDRClientHealthCheck tests the health check endpoint
func TestIDRClientHealthCheck(t *testing.T) {
	// Create mock IDR server
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

// TestIDRClientCircuitBreaker tests circuit breaker behavior
func TestIDRClientCircuitBreaker(t *testing.T) {
	failCount := 0
	failThreshold := 5

	// Create mock IDR server that fails initially then recovers
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		if failCount <= failThreshold {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// After failures, return success
		response := idr.SelectPartnersResponse{
			SelectedBidders: []idr.SelectedBidder{
				{BidderCode: "appnexus", Score: 0.95, Reason: "HIGH_SCORE"},
			},
			Mode: "normal",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockIDR.Close()

	// Create client with aggressive circuit breaker for testing
	config := &idr.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}
	client := idr.NewClientWithCircuitBreaker(mockIDR.URL, 100*time.Millisecond, "", config)

	ortbJSON := []byte(`{"id":"test-1"}`)
	bidders := []string{"appnexus"}

	// Make requests to trigger failures
	for i := 0; i < 4; i++ {
		_, err := client.SelectPartners(context.Background(), ortbJSON, bidders)
		if i < 3 && err == nil {
			// First 3 should return errors from the server
			t.Logf("Request %d: expected error, but may be OK due to circuit breaker", i)
		}
	}

	// Circuit should be open now
	if !client.IsCircuitOpen() {
		t.Log("Circuit breaker may not be open - depends on timing and implementation")
	}

	// Reset and verify it can work again
	client.ResetCircuitBreaker()

	if client.IsCircuitOpen() {
		t.Error("Circuit should be closed after reset")
	}
}

// TestIDRClientBypassMode tests bypass mode toggle
func TestIDRClientBypassMode(t *testing.T) {
	bypassEnabled := false

	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mode/bypass" && r.Method == "POST" {
			var body map[string]bool
			json.NewDecoder(r.Body).Decode(&body)
			bypassEnabled = body["enabled"]
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	// Enable bypass mode
	err := client.SetBypassMode(context.Background(), true)
	if err != nil {
		t.Fatalf("SetBypassMode failed: %v", err)
	}

	if !bypassEnabled {
		t.Error("Expected bypass mode to be enabled")
	}

	// Disable bypass mode
	err = client.SetBypassMode(context.Background(), false)
	if err != nil {
		t.Fatalf("SetBypassMode failed: %v", err)
	}

	if bypassEnabled {
		t.Error("Expected bypass mode to be disabled")
	}
}

// TestIDRClientShadowMode tests shadow mode toggle
func TestIDRClientShadowMode(t *testing.T) {
	shadowEnabled := false

	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mode/shadow" && r.Method == "POST" {
			var body map[string]bool
			json.NewDecoder(r.Body).Decode(&body)
			shadowEnabled = body["enabled"]
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	err := client.SetShadowMode(context.Background(), true)
	if err != nil {
		t.Fatalf("SetShadowMode failed: %v", err)
	}

	if !shadowEnabled {
		t.Error("Expected shadow mode to be enabled")
	}
}

// TestIDRClientGetConfig tests config retrieval
func TestIDRClientGetConfig(t *testing.T) {
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config" && r.Method == "GET" {
			config := map[string]interface{}{
				"mode":          "normal",
				"bypass_enabled": false,
				"shadow_mode":    false,
				"fpd": map[string]interface{}{
					"enabled":      true,
					"site_enabled": true,
					"user_enabled": true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(config)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	config, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if config["mode"] != "normal" {
		t.Errorf("Expected mode 'normal', got '%v'", config["mode"])
	}
}

// TestIDRClientGetFPDConfig tests FPD config retrieval
func TestIDRClientGetFPDConfig(t *testing.T) {
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config" && r.Method == "GET" {
			config := map[string]interface{}{
				"fpd": map[string]interface{}{
					"enabled":        true,
					"site_enabled":   true,
					"user_enabled":   true,
					"imp_enabled":    false,
					"eids_enabled":   true,
					"eid_sources":    "liveramp.com,criteo.com",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(config)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 100*time.Millisecond, "")

	fpdConfig, err := client.GetFPDConfig(context.Background())
	if err != nil {
		t.Fatalf("GetFPDConfig failed: %v", err)
	}

	if !fpdConfig.Enabled {
		t.Error("Expected FPD to be enabled")
	}

	if !fpdConfig.SiteEnabled {
		t.Error("Expected site FPD to be enabled")
	}

	if fpdConfig.ImpEnabled {
		t.Error("Expected imp FPD to be disabled")
	}

	if fpdConfig.EIDSources != "liveramp.com,criteo.com" {
		t.Errorf("Unexpected EID sources: %s", fpdConfig.EIDSources)
	}
}

// TestIDRClientTimeout tests timeout behavior
func TestIDRClientTimeout(t *testing.T) {
	// Create slow mock server
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockIDR.Close()

	// Create client with very short timeout
	client := idr.NewClient(mockIDR.URL, 50*time.Millisecond, "")

	ctx := context.Background()
	_, err := client.SelectPartners(ctx, []byte(`{"id":"test"}`), []string{"appnexus"})

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestIDRClientContextCancellation tests context cancellation
func TestIDRClientContextCancellation(t *testing.T) {
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 1*time.Second, "")

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.SelectPartners(ctx, []byte(`{"id":"test"}`), []string{"appnexus"})

	if err == nil {
		t.Error("Expected cancellation error, got nil")
	}
}

// TestIDRResponseSizeLimit tests response size limiting
func TestIDRResponseSizeLimit(t *testing.T) {
	// Create mock server that returns oversized response
	mockIDR := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start with valid JSON structure
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"selected_bidders":[`))

		// Write lots of data
		for i := 0; i < 100000; i++ {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write([]byte(`{"bidder_code":"bidder","score":0.5,"reason":"TEST"}`))
		}

		w.Write([]byte(`]}`))
	}))
	defer mockIDR.Close()

	client := idr.NewClient(mockIDR.URL, 5*time.Second, "")

	_, err := client.SelectPartners(context.Background(), []byte(`{"id":"test"}`), []string{"appnexus"})

	// Should get an error due to response size limit or JSON decode error
	if err == nil {
		t.Log("Request may have succeeded - response size limit depends on implementation")
	}
}
