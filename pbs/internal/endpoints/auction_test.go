package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/exchange"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// mockExchange implements a minimal Exchange for testing
type mockExchange struct {
	response *exchange.AuctionResponse
	err      error
}

func (m *mockExchange) RunAuction(ctx context.Context, req *exchange.AuctionRequest) (*exchange.AuctionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func newMockExchange(response *exchange.AuctionResponse, err error) *exchange.Exchange {
	// We can't easily mock exchange.Exchange since it's a struct, not interface
	// For endpoint tests, we'll focus on request validation paths
	return nil
}

func TestStatusHandler(t *testing.T) {
	handler := NewStatusHandler()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Expected timestamp in response")
	}
}

func TestValidateBidRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *openrtb.BidRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			request: &openrtb.BidRequest{
				ID: "test-123",
				Imp: []openrtb.Imp{
					{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			request: &openrtb.BidRequest{
				Imp: []openrtb.Imp{
					{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
				},
			},
			wantErr: true,
			errMsg:  "id: required",
		},
		{
			name: "no impressions",
			request: &openrtb.BidRequest{
				ID:  "test-123",
				Imp: []openrtb.Imp{},
			},
			wantErr: true,
			errMsg:  "imp: at least one impression required",
		},
		{
			name: "impression missing ID",
			request: &openrtb.BidRequest{
				ID: "test-123",
				Imp: []openrtb.Imp{
					{Banner: &openrtb.Banner{W: 300, H: 250}},
				},
			},
			wantErr: true,
			errMsg:  "imp[].id: required",
		},
		{
			name: "impression missing media type",
			request: &openrtb.BidRequest{
				ID: "test-123",
				Imp: []openrtb.Imp{
					{ID: "imp-1"},
				},
			},
			wantErr: true,
			errMsg:  "imp[].banner|video|native|audio: at least one media type required",
		},
		{
			name: "valid with video",
			request: &openrtb.BidRequest{
				ID: "test-123",
				Imp: []openrtb.Imp{
					{ID: "imp-1", Video: &openrtb.Video{W: 640, H: 480}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with multiple impressions",
			request: &openrtb.BidRequest{
				ID: "test-123",
				Imp: []openrtb.Imp{
					{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
					{ID: "imp-2", Video: &openrtb.Video{W: 640, H: 480}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBidRequest(tt.request)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "simple field error",
			err:      &ValidationError{Field: "id", Message: "required", Index: -1},
			expected: "id: required",
		},
		{
			name:     "indexed field error",
			err:      &ValidationError{Field: "imp[].id", Message: "required", Index: 0},
			expected: "imp[].id: required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBuildResponseExt(t *testing.T) {
	result := &exchange.AuctionResponse{
		BidResponse: &openrtb.BidResponse{ID: "test-1"},
		DebugInfo: &exchange.DebugInfo{
			BidderLatencies: map[string]time.Duration{
				"appnexus": 50 * time.Millisecond,
				"rubicon":  75 * time.Millisecond,
			},
			Errors: map[string][]string{
				"pubmatic": {"timeout", "bad response"},
			},
			TotalLatency: 100 * time.Millisecond,
		},
	}

	ext := buildResponseExt(result)

	if ext.TMMaxRequest != 100 {
		t.Errorf("Expected TMMaxRequest 100, got %d", ext.TMMaxRequest)
	}

	if ext.ResponseTimeMillis["appnexus"] != 50 {
		t.Errorf("Expected appnexus latency 50, got %d", ext.ResponseTimeMillis["appnexus"])
	}

	if ext.ResponseTimeMillis["rubicon"] != 75 {
		t.Errorf("Expected rubicon latency 75, got %d", ext.ResponseTimeMillis["rubicon"])
	}

	if len(ext.Errors["pubmatic"]) != 2 {
		t.Errorf("Expected 2 pubmatic errors, got %d", len(ext.Errors["pubmatic"]))
	}
}

func TestBuildResponseExt_NilDebugInfo(t *testing.T) {
	result := &exchange.AuctionResponse{
		BidResponse: &openrtb.BidResponse{ID: "test-1"},
		DebugInfo:   nil,
	}

	ext := buildResponseExt(result)

	if ext.TMMaxRequest != 0 {
		t.Errorf("Expected TMMaxRequest 0, got %d", ext.TMMaxRequest)
	}

	if len(ext.ResponseTimeMillis) != 0 {
		t.Errorf("Expected empty ResponseTimeMillis, got %d entries", len(ext.ResponseTimeMillis))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, "test error message", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "test error message" {
		t.Errorf("Expected error 'test error message', got '%s'", response["error"])
	}
}

// mockBidderLister implements BidderLister for testing
type mockBidderLister struct {
	bidders []string
}

func (m *mockBidderLister) ListBidders() []string {
	return m.bidders
}

// mockDynamicLister implements DynamicBidderLister for testing
type mockDynamicLister struct {
	bidders []string
}

func (m *mockDynamicLister) ListBidderCodes() []string {
	return m.bidders
}

func TestInfoBiddersHandler(t *testing.T) {
	staticRegistry := &mockBidderLister{bidders: []string{"appnexus", "rubicon"}}
	dynamicRegistry := &mockDynamicLister{bidders: []string{"custom1", "appnexus"}} // appnexus overlaps

	handler := NewDynamicInfoBiddersHandler(staticRegistry, dynamicRegistry)

	req := httptest.NewRequest(http.MethodGet, "/info/bidders", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var bidders []string
	if err := json.Unmarshal(w.Body.Bytes(), &bidders); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have 3 unique bidders (appnexus appears in both but deduplicated)
	if len(bidders) != 3 {
		t.Errorf("Expected 3 unique bidders, got %d: %v", len(bidders), bidders)
	}

	// Verify all expected bidders are present
	bidderSet := make(map[string]bool)
	for _, b := range bidders {
		bidderSet[b] = true
	}

	expected := []string{"appnexus", "rubicon", "custom1"}
	for _, e := range expected {
		if !bidderSet[e] {
			t.Errorf("Expected bidder '%s' not found in response", e)
		}
	}
}

func TestInfoBiddersHandler_StaticOnly(t *testing.T) {
	staticRegistry := &mockBidderLister{bidders: []string{"appnexus", "rubicon"}}

	handler := NewDynamicInfoBiddersHandler(staticRegistry, nil)

	req := httptest.NewRequest(http.MethodGet, "/info/bidders", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var bidders []string
	if err := json.Unmarshal(w.Body.Bytes(), &bidders); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(bidders) != 2 {
		t.Errorf("Expected 2 bidders, got %d", len(bidders))
	}
}

func TestInfoBiddersHandler_NoRegistries(t *testing.T) {
	handler := NewDynamicInfoBiddersHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/info/bidders", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var bidders []string
	if err := json.Unmarshal(w.Body.Bytes(), &bidders); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(bidders) != 0 {
		t.Errorf("Expected 0 bidders, got %d", len(bidders))
	}
}

func TestAuctionHandler_MethodNotAllowed(t *testing.T) {
	handler := NewAuctionHandler(nil)

	// Test non-POST methods
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/openrtb2/auction", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

func TestAuctionHandler_InvalidJSON(t *testing.T) {
	handler := NewAuctionHandler(nil)

	body := bytes.NewBufferString("not valid json")
	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", body)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "Invalid JSON in request body" {
		t.Errorf("Expected 'Invalid JSON in request body', got '%s'", response["error"])
	}
}

func TestAuctionHandler_ValidationError(t *testing.T) {
	handler := NewAuctionHandler(nil)

	// Missing required fields
	invalidRequest := openrtb.BidRequest{
		ID: "", // Missing ID
	}
	body, _ := json.Marshal(invalidRequest)

	req := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestNewAuctionHandler(t *testing.T) {
	handler := NewAuctionHandler(nil)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestNewStatusHandler(t *testing.T) {
	handler := NewStatusHandler()

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestNewInfoBiddersHandler_Deprecated(t *testing.T) {
	// Test the deprecated constructor
	handler := NewInfoBiddersHandler([]string{"a", "b"})

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}
