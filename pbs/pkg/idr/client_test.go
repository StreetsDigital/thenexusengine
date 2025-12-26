package idr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:5050", 0, "test-api-key")
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.timeout != 150*time.Millisecond {
		t.Errorf("expected default 150ms timeout, got %v", client.timeout)
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("expected api key 'test-api-key', got %s", client.apiKey)
	}
}

func TestNewClientWithCircuitBreaker(t *testing.T) {
	cbConfig := &CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          time.Second,
	}

	client := NewClientWithCircuitBreaker("http://localhost:5050", 100*time.Millisecond, "", cbConfig)
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.timeout != 100*time.Millisecond {
		t.Errorf("expected 100ms timeout, got %v", client.timeout)
	}

	stats := client.CircuitBreakerStats()
	if stats.State != StateClosed {
		t.Errorf("expected closed state, got %s", stats.State)
	}
}

func TestSelectPartnersSuccess(t *testing.T) {
	// Create mock IDR server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/select" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify API key header
		if r.Header.Get("X-Internal-API-Key") != "test-key" {
			t.Errorf("expected X-Internal-API-Key header, got %s", r.Header.Get("X-Internal-API-Key"))
		}

		var req SelectPartnersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Return mock response
		resp := SelectPartnersResponse{
			SelectedBidders: []SelectedBidder{
				{BidderCode: "appnexus", Score: 0.95, Reason: "HIGH_SCORE"},
				{BidderCode: "rubicon", Score: 0.85, Reason: "DIVERSITY"},
			},
			ExcludedBidders: []ExcludedBidder{
				{BidderCode: "pubmatic", Score: 0.30, Reason: "LOW_SCORE"},
			},
			Mode:             "normal",
			ProcessingTimeMs: 5.2,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "test-key")

	req := json.RawMessage(`{"id":"test-1"}`)
	bidders := []string{"appnexus", "rubicon", "pubmatic"}

	resp, err := client.SelectPartners(context.Background(), req, bidders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if len(resp.SelectedBidders) != 2 {
		t.Errorf("expected 2 selected bidders, got %d", len(resp.SelectedBidders))
	}

	if resp.SelectedBidders[0].BidderCode != "appnexus" {
		t.Errorf("expected appnexus first, got %s", resp.SelectedBidders[0].BidderCode)
	}

	if len(resp.ExcludedBidders) != 1 {
		t.Errorf("expected 1 excluded bidder, got %d", len(resp.ExcludedBidders))
	}
}

func TestSelectPartnersServerError(t *testing.T) {
	// Create mock server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	req := json.RawMessage(`{"id":"test-1"}`)
	bidders := []string{"appnexus"}

	_, err := client.SelectPartners(context.Background(), req, bidders)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestSelectPartnersCircuitOpen(t *testing.T) {
	// Create server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClientWithCircuitBreaker(server.URL, 100*time.Millisecond, "", &CircuitBreakerConfig{
		FailureThreshold: 2, // Open after 2 failures
		SuccessThreshold: 1,
		Timeout:          time.Second,
	})

	req := json.RawMessage(`{"id":"test-1"}`)
	bidders := []string{"appnexus"}

	// Trigger failures to open circuit
	for i := 0; i < 2; i++ {
		client.SelectPartners(context.Background(), req, bidders)
	}

	// Circuit should be open now
	if !client.IsCircuitOpen() {
		t.Error("expected circuit to be open")
	}

	// Next call should return nil, nil (fail open)
	resp, err := client.SelectPartners(context.Background(), req, bidders)
	if err != nil {
		t.Errorf("expected nil error when circuit open, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response when circuit open, got %v", resp)
	}
}

func TestHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("unexpected health check error: %v", err)
	}
}

func TestHealthCheckFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy service")
	}
}

func TestGetConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config" {
			config := map[string]interface{}{
				"mode":         "normal",
				"max_partners": 10,
				"fpd": map[string]interface{}{
					"enabled":      true,
					"eids_enabled": true,
					"eid_sources":  "liveramp.com,uidapi.com",
				},
			}
			json.NewEncoder(w).Encode(config)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	config, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config["mode"] != "normal" {
		t.Errorf("expected normal mode, got %v", config["mode"])
	}
}

func TestGetFPDConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := map[string]interface{}{
			"fpd": map[string]interface{}{
				"enabled":       true,
				"site_enabled":  true,
				"user_enabled":  true,
				"eids_enabled":  true,
				"eid_sources":   "liveramp.com,id5-sync.com",
			},
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	fpdConfig, err := client.GetFPDConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fpdConfig.Enabled {
		t.Error("expected FPD to be enabled")
	}
	if !fpdConfig.EIDsEnabled {
		t.Error("expected EIDs to be enabled")
	}
	if fpdConfig.EIDSources != "liveramp.com,id5-sync.com" {
		t.Errorf("unexpected EID sources: %s", fpdConfig.EIDSources)
	}
}

func TestGetFPDConfigNoFPDSection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return config without FPD section
		config := map[string]interface{}{
			"mode": "normal",
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	fpdConfig, err := client.GetFPDConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return defaults
	if !fpdConfig.Enabled {
		t.Error("expected default FPD to be enabled")
	}
}

func TestSetBypassMode(t *testing.T) {
	var receivedEnabled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mode/bypass" && r.Method == "POST" {
			var body map[string]bool
			json.NewDecoder(r.Body).Decode(&body)
			receivedEnabled = body["enabled"]
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	err := client.SetBypassMode(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receivedEnabled {
		t.Error("expected bypass mode to be enabled")
	}
}

func TestSetShadowMode(t *testing.T) {
	var receivedEnabled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mode/shadow" && r.Method == "POST" {
			var body map[string]bool
			json.NewDecoder(r.Body).Decode(&body)
			receivedEnabled = body["enabled"]
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100*time.Millisecond, "")

	err := client.SetShadowMode(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receivedEnabled {
		t.Error("expected shadow mode to be enabled")
	}
}

func TestResetCircuitBreaker(t *testing.T) {
	client := NewClient("http://localhost:5050", 100*time.Millisecond, "")

	// Force open the circuit
	client.circuitBreaker.ForceOpen()

	if !client.IsCircuitOpen() {
		t.Error("expected circuit to be open")
	}

	// Reset
	client.ResetCircuitBreaker()

	if client.IsCircuitOpen() {
		t.Error("expected circuit to be closed after reset")
	}
}

func TestContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 1*time.Second, "")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := json.RawMessage(`{"id":"test-1"}`)
	bidders := []string{"appnexus"}

	_, err := client.SelectPartners(ctx, req, bidders)

	// Should get an error due to context timeout
	if err == nil {
		t.Error("expected error due to context timeout")
	}
}
