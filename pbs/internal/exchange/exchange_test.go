package exchange

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/fpd"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// mockAdapter implements adapters.Adapter for testing
type mockAdapter struct {
	bids     []*adapters.TypedBid
	makeErr  error
	bidsErr  error
	requests []*adapters.RequestData
}

func (m *mockAdapter) MakeRequests(request *openrtb.BidRequest, reqInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	if m.makeErr != nil {
		return nil, []error{m.makeErr}
	}
	if len(m.requests) > 0 {
		return m.requests, nil
	}
	// Return a minimal request
	return []*adapters.RequestData{
		{
			Method: "POST",
			URI:    "http://test.bidder.com/bid",
			Body:   []byte(`{}`),
		},
	}, nil
}

func (m *mockAdapter) MakeBids(internalRequest *openrtb.BidRequest, response *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if m.bidsErr != nil {
		return nil, []error{m.bidsErr}
	}
	return &adapters.BidderResponse{
		Bids: m.bids,
	}, nil
}

func TestExchangeNew(t *testing.T) {
	registry := adapters.NewRegistry()

	ex := New(registry, nil)
	if ex == nil {
		t.Fatal("expected non-nil exchange")
	}

	// Test with custom config
	config := &Config{
		DefaultTimeout:  500 * time.Millisecond,
		MaxBidders:      10,
		IDREnabled:      false,
		DefaultCurrency: "EUR",
	}

	ex = New(registry, config)
	if ex.config.DefaultTimeout != 500*time.Millisecond {
		t.Errorf("expected 500ms timeout, got %v", ex.config.DefaultTimeout)
	}
	if ex.config.DefaultCurrency != "EUR" {
		t.Errorf("expected EUR currency, got %s", ex.config.DefaultCurrency)
	}
}

func TestExchangeRunAuctionNoBidders(t *testing.T) {
	registry := adapters.NewRegistry()
	ex := New(registry, &Config{
		DefaultTimeout: 100 * time.Millisecond,
		IDREnabled:     false,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID: "test-req-1",
			Site: &openrtb.Site{ID: "site1"},
			Imp: []openrtb.Imp{
				{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}},
			},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.BidResponse == nil {
		t.Fatal("expected non-nil bid response")
	}

	if len(resp.BidResponse.SeatBid) != 0 {
		t.Errorf("expected 0 seat bids, got %d", len(resp.BidResponse.SeatBid))
	}
}

func TestExchangeRunAuctionWithBidders(t *testing.T) {
	registry := adapters.NewRegistry()

	// Register a mock adapter
	mockBid := &openrtb.Bid{
		ID:    "bid1",
		ImpID: "imp1",
		Price: 2.50,
		AdM:   "<div>test ad</div>",
	}

	// Create mock HTTP server for bidder endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid bid response
		resp := &openrtb.BidResponse{
			ID: "test-req-2",
			SeatBid: []openrtb.SeatBid{
				{
					Bid: []openrtb.Bid{*mockBid},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mock := &mockAdapter{
		bids: []*adapters.TypedBid{
			{Bid: mockBid, BidType: adapters.BidTypeBanner},
		},
		requests: []*adapters.RequestData{{Method: "POST", URI: server.URL, Body: []byte(`{}`)}},
	}

	registry.Register("test-bidder", mock, adapters.BidderInfo{
		Enabled: true,
	})

	ex := New(registry, &Config{
		DefaultTimeout: 500 * time.Millisecond,
		IDREnabled:     false,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID: "test-req-2",
			Site: &openrtb.Site{ID: "site1"},
			Imp: []openrtb.Imp{
				{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}},
			},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.BidderResults == nil {
		t.Fatal("expected non-nil bidder results")
	}

	// Check that test-bidder was called
	result, ok := resp.BidderResults["test-bidder"]
	if !ok {
		t.Error("expected test-bidder in results")
	} else {
		if result.BidderCode != "test-bidder" {
			t.Errorf("expected bidder code 'test-bidder', got %s", result.BidderCode)
		}
	}
}

func TestExchangeFPDProcessing(t *testing.T) {
	registry := adapters.NewRegistry()

	// Register a mock adapter
	mock := &mockAdapter{
		bids: []*adapters.TypedBid{},
	}
	registry.Register("bidder1", mock, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout: 100 * time.Millisecond,
		IDREnabled:     false,
		FPD: &fpd.Config{
			Enabled:     true,
			SiteEnabled: true,
			UserEnabled: true,
		},
	})

	// Create request with FPD
	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID: "test-fpd",
			Site: &openrtb.Site{
				ID:   "site1",
				Name: "Test Site",
				Ext:  json.RawMessage(`{"data":{"category":"news"}}`),
			},
			User: &openrtb.User{
				ID:  "user1",
				Ext: json.RawMessage(`{"data":{"interests":["sports","tech"]}}`),
			},
			Imp: []openrtb.Imp{
				{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}},
			},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FPD should be processed without errors
	if len(resp.DebugInfo.Errors["fpd"]) > 0 {
		t.Errorf("unexpected FPD errors: %v", resp.DebugInfo.Errors["fpd"])
	}
}

func TestExchangeEIDFiltering(t *testing.T) {
	registry := adapters.NewRegistry()

	mock := &mockAdapter{bids: []*adapters.TypedBid{}}
	registry.Register("bidder1", mock, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout: 100 * time.Millisecond,
		IDREnabled:     false,
		FPD: &fpd.Config{
			Enabled:     true,
			EIDsEnabled: true,
			EIDSources:  []string{"liveramp.com"},
		},
	})

	// Create request with multiple EIDs
	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID: "test-eid",
			Site: &openrtb.Site{ID: "site1"},
			User: &openrtb.User{
				ID: "user1",
				EIDs: []openrtb.EID{
					{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "lr123"}}},
					{Source: "blocked.com", UIDs: []openrtb.UID{{ID: "blk456"}}},
				},
			},
			Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
	}

	_, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After filtering, only liveramp.com should remain
	if len(req.BidRequest.User.EIDs) != 1 {
		t.Errorf("expected 1 EID after filtering, got %d", len(req.BidRequest.User.EIDs))
	}
	if req.BidRequest.User.EIDs[0].Source != "liveramp.com" {
		t.Errorf("expected liveramp.com EID, got %s", req.BidRequest.User.EIDs[0].Source)
	}
}

func TestExchangeTimeoutFromRequest(t *testing.T) {
	registry := adapters.NewRegistry()

	ex := New(registry, &Config{
		DefaultTimeout: 1 * time.Second,
		IDREnabled:     false,
	})

	// Request with TMax
	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID:   "test-timeout",
			TMax: 100, // 100ms
			Site: &openrtb.Site{ID: "site1"},
			Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
	}

	start := time.Now()
	_, err := ex.RunAuction(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete quickly since no bidders
	if elapsed > 200*time.Millisecond {
		t.Errorf("auction took too long: %v", elapsed)
	}
}

func TestExchangeUpdateFPDConfig(t *testing.T) {
	registry := adapters.NewRegistry()
	ex := New(registry, nil)

	// Initial config
	if ex.GetFPDConfig() == nil {
		t.Error("expected non-nil initial FPD config")
	}

	// Update config
	newConfig := &fpd.Config{
		Enabled:     false,
		EIDsEnabled: false,
	}
	ex.UpdateFPDConfig(newConfig)

	got := ex.GetFPDConfig()
	if got.Enabled {
		t.Error("expected FPD to be disabled after update")
	}
}

func TestExchangeDebugInfo(t *testing.T) {
	registry := adapters.NewRegistry()

	mock := &mockAdapter{bids: []*adapters.TypedBid{}}
	registry.Register("bidder1", mock, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout: 100 * time.Millisecond,
		IDREnabled:     false,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID:   "test-debug",
			Site: &openrtb.Site{ID: "site1"},
			Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
		Debug: true,
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.DebugInfo == nil {
		t.Fatal("expected non-nil debug info")
	}

	if resp.DebugInfo.RequestTime.IsZero() {
		t.Error("expected non-zero request time")
	}

	if resp.DebugInfo.TotalLatency == 0 {
		t.Error("expected non-zero total latency")
	}

	if len(resp.DebugInfo.SelectedBidders) != 1 {
		t.Errorf("expected 1 selected bidder, got %d", len(resp.DebugInfo.SelectedBidders))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DefaultTimeout == 0 {
		t.Error("expected non-zero default timeout")
	}
	if config.MaxBidders == 0 {
		t.Error("expected non-zero max bidders")
	}
	if config.DefaultCurrency == "" {
		t.Error("expected non-empty default currency")
	}
	if config.FPD == nil {
		t.Error("expected non-nil FPD config")
	}
	if config.AuctionType != FirstPriceAuction {
		t.Error("expected first-price auction as default")
	}
	if config.PriceIncrement != 0.01 {
		t.Errorf("expected 0.01 price increment, got %f", config.PriceIncrement)
	}
}

func TestBidValidation(t *testing.T) {
	registry := adapters.NewRegistry()
	ex := New(registry, &Config{
		DefaultTimeout:  100 * time.Millisecond,
		IDREnabled:      false,
		DefaultCurrency: "USD",
		MinBidPrice:     0.01,
	})

	impFloors := map[string]float64{
		"imp1": 1.00,
		"imp2": 0.50,
	}

	tests := []struct {
		name        string
		bid         *openrtb.Bid
		bidderCode  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil bid",
			bid:         nil,
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "nil bid",
		},
		{
			name:        "missing bid ID",
			bid:         &openrtb.Bid{ImpID: "imp1", Price: 1.50},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "missing required field: id",
		},
		{
			name:        "missing impID",
			bid:         &openrtb.Bid{ID: "bid1", Price: 1.50},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "missing required field: impid",
		},
		{
			name:        "invalid impID",
			bid:         &openrtb.Bid{ID: "bid1", ImpID: "invalid", Price: 1.50},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "not found in request",
		},
		{
			name:        "negative price",
			bid:         &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: -1.00},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "negative price",
		},
		{
			name:        "below minimum price",
			bid:         &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 0.005},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "below minimum",
		},
		{
			name:        "below floor price",
			bid:         &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 0.75},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "below floor",
		},
		{
			name:       "valid bid at floor",
			bid:        &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 1.00, AdM: "<div>ad</div>"},
			bidderCode: "test-bidder",
			wantErr:    false,
		},
		{
			name:       "valid bid above floor",
			bid:        &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 2.50, AdM: "<div>ad</div>"},
			bidderCode: "test-bidder",
			wantErr:    false,
		},
		{
			name:       "valid bid lower floor impression",
			bid:        &openrtb.Bid{ID: "bid2", ImpID: "imp2", Price: 0.50, AdM: "<div>ad</div>"},
			bidderCode: "test-bidder",
			wantErr:    false,
		},
		{
			name:        "missing adm and nurl",
			bid:         &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 2.00},
			bidderCode:  "test-bidder",
			wantErr:     true,
			errContains: "adm or nurl",
		},
		{
			name:       "valid bid with nurl only",
			bid:        &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 2.00, NURL: "http://example.com/win"},
			bidderCode: "test-bidder",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ex.validateBid(tt.bid, tt.bidderCode, impFloors)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBidDeduplication(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create mock HTTP server for bidder1
	bid1 := &openrtb.Bid{ID: "dup-bid", ImpID: "imp1", Price: 2.00, AdM: "<div>ad1</div>"}
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID: "test-request",
			SeatBid: []openrtb.SeatBid{
				{Bid: []openrtb.Bid{*bid1}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server1.Close()

	// Create mock HTTP server for bidder2
	bid2 := &openrtb.Bid{ID: "dup-bid", ImpID: "imp1", Price: 3.00, AdM: "<div>ad2</div>"}
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID: "test-request",
			SeatBid: []openrtb.SeatBid{
				{Bid: []openrtb.Bid{*bid2}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	// Create adapters that use the mock servers
	mock1 := &mockAdapter{
		bids: []*adapters.TypedBid{
			{Bid: bid1, BidType: adapters.BidTypeBanner},
		},
		requests: []*adapters.RequestData{
			{Method: "POST", URI: server1.URL, Body: []byte(`{}`)},
		},
	}
	mock2 := &mockAdapter{
		bids: []*adapters.TypedBid{
			{Bid: bid2, BidType: adapters.BidTypeBanner},
		},
		requests: []*adapters.RequestData{
			{Method: "POST", URI: server2.URL, Body: []byte(`{}`)},
		},
	}

	registry.Register("bidder1", mock1, adapters.BidderInfo{Enabled: true})
	registry.Register("bidder2", mock2, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout:  500 * time.Millisecond,
		IDREnabled:      false,
		DefaultCurrency: "USD",
		AuctionType:     FirstPriceAuction,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID:   "test-dedup",
			Site: &openrtb.Site{ID: "site1"},
			Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one bid should win (first one seen), the duplicate should be rejected
	totalBids := 0
	for _, sb := range resp.BidResponse.SeatBid {
		totalBids += len(sb.Bid)
	}

	if totalBids != 1 {
		t.Errorf("expected 1 bid after deduplication, got %d", totalBids)
	}

	// Check that a deduplication error was recorded
	foundDupError := false
	for _, errors := range resp.DebugInfo.Errors {
		for _, errMsg := range errors {
			if containsString(errMsg, "duplicate bid ID") {
				foundDupError = true
				break
			}
		}
	}
	if !foundDupError {
		t.Error("expected duplicate bid error in debug info")
	}
}

func TestSecondPriceAuction(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create mock HTTP servers and bidders with different prices
	bid1 := &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 5.00, AdM: "<div>ad1</div>"}
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID:      "test-second-price",
			SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{*bid1}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server1.Close()

	bid2 := &openrtb.Bid{ID: "bid2", ImpID: "imp1", Price: 3.00, AdM: "<div>ad2</div>"}
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID:      "test-second-price",
			SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{*bid2}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	bid3 := &openrtb.Bid{ID: "bid3", ImpID: "imp1", Price: 2.00, AdM: "<div>ad3</div>"}
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID:      "test-second-price",
			SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{*bid3}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server3.Close()

	mock1 := &mockAdapter{
		bids:     []*adapters.TypedBid{{Bid: bid1, BidType: adapters.BidTypeBanner}},
		requests: []*adapters.RequestData{{Method: "POST", URI: server1.URL, Body: []byte(`{}`)}},
	}
	mock2 := &mockAdapter{
		bids:     []*adapters.TypedBid{{Bid: bid2, BidType: adapters.BidTypeBanner}},
		requests: []*adapters.RequestData{{Method: "POST", URI: server2.URL, Body: []byte(`{}`)}},
	}
	mock3 := &mockAdapter{
		bids:     []*adapters.TypedBid{{Bid: bid3, BidType: adapters.BidTypeBanner}},
		requests: []*adapters.RequestData{{Method: "POST", URI: server3.URL, Body: []byte(`{}`)}},
	}

	registry.Register("bidder1", mock1, adapters.BidderInfo{Enabled: true})
	registry.Register("bidder2", mock2, adapters.BidderInfo{Enabled: true})
	registry.Register("bidder3", mock3, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout:  500 * time.Millisecond,
		IDREnabled:      false,
		DefaultCurrency: "USD",
		AuctionType:     SecondPriceAuction,
		PriceIncrement:  0.01,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID:   "test-second-price",
			Site: &openrtb.Site{ID: "site1"},
			Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the winning bid (highest)
	var winningPrice float64
	for _, sb := range resp.BidResponse.SeatBid {
		for _, bid := range sb.Bid {
			if bid.Price > winningPrice {
				winningPrice = bid.Price
			}
		}
	}

	// In second-price auction, winner should pay second-highest + increment
	// Second highest is 3.00, so winning price should be 3.01
	expectedPrice := 3.01
	if winningPrice != expectedPrice {
		t.Errorf("expected winning price %.2f (second price + increment), got %.2f", expectedPrice, winningPrice)
	}
}

func TestFirstPriceAuction(t *testing.T) {
	registry := adapters.NewRegistry()

	// Create mock HTTP servers
	bid1 := &openrtb.Bid{ID: "bid1", ImpID: "imp1", Price: 5.00, AdM: "<div>ad1</div>"}
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID:      "test-first-price",
			SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{*bid1}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server1.Close()

	bid2 := &openrtb.Bid{ID: "bid2", ImpID: "imp1", Price: 3.00, AdM: "<div>ad2</div>"}
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &openrtb.BidResponse{
			ID:      "test-first-price",
			SeatBid: []openrtb.SeatBid{{Bid: []openrtb.Bid{*bid2}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	mock1 := &mockAdapter{
		bids:     []*adapters.TypedBid{{Bid: bid1, BidType: adapters.BidTypeBanner}},
		requests: []*adapters.RequestData{{Method: "POST", URI: server1.URL, Body: []byte(`{}`)}},
	}
	mock2 := &mockAdapter{
		bids:     []*adapters.TypedBid{{Bid: bid2, BidType: adapters.BidTypeBanner}},
		requests: []*adapters.RequestData{{Method: "POST", URI: server2.URL, Body: []byte(`{}`)}},
	}

	registry.Register("bidder1", mock1, adapters.BidderInfo{Enabled: true})
	registry.Register("bidder2", mock2, adapters.BidderInfo{Enabled: true})

	ex := New(registry, &Config{
		DefaultTimeout:  500 * time.Millisecond,
		IDREnabled:      false,
		DefaultCurrency: "USD",
		AuctionType:     FirstPriceAuction,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID:   "test-first-price",
			Site: &openrtb.Site{ID: "site1"},
			Imp:  []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		},
	}

	resp, err := ex.RunAuction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the highest bid
	var highestPrice float64
	for _, sb := range resp.BidResponse.SeatBid {
		for _, bid := range sb.Bid {
			if bid.Price > highestPrice {
				highestPrice = bid.Price
			}
		}
	}

	// In first-price auction, winner pays their bid (5.00)
	if highestPrice != 5.00 {
		t.Errorf("expected winning price 5.00 (first price), got %.2f", highestPrice)
	}
}

func TestSortBidsByPrice(t *testing.T) {
	bids := []ValidatedBid{
		{Bid: &adapters.TypedBid{Bid: &openrtb.Bid{ID: "b1", Price: 1.00}}, BidderCode: "bidder1"},
		{Bid: &adapters.TypedBid{Bid: &openrtb.Bid{ID: "b2", Price: 5.00}}, BidderCode: "bidder2"},
		{Bid: &adapters.TypedBid{Bid: &openrtb.Bid{ID: "b3", Price: 3.00}}, BidderCode: "bidder3"},
		{Bid: &adapters.TypedBid{Bid: &openrtb.Bid{ID: "b4", Price: 2.50}}, BidderCode: "bidder4"},
	}

	sortBidsByPrice(bids)

	// Should be sorted descending
	expectedPrices := []float64{5.00, 3.00, 2.50, 1.00}
	for i, bid := range bids {
		if bid.Bid.Bid.Price != expectedPrices[i] {
			t.Errorf("position %d: expected price %.2f, got %.2f", i, expectedPrices[i], bid.Bid.Bid.Price)
		}
	}
}

func TestBuildImpFloorMap(t *testing.T) {
	req := &openrtb.BidRequest{
		Imp: []openrtb.Imp{
			{ID: "imp1", BidFloor: 1.50},
			{ID: "imp2", BidFloor: 0.75},
			{ID: "imp3", BidFloor: 0}, // No floor
		},
	}

	floors := buildImpFloorMap(req)

	if len(floors) != 3 {
		t.Errorf("expected 3 floor entries, got %d", len(floors))
	}
	if floors["imp1"] != 1.50 {
		t.Errorf("expected imp1 floor 1.50, got %f", floors["imp1"])
	}
	if floors["imp2"] != 0.75 {
		t.Errorf("expected imp2 floor 0.75, got %f", floors["imp2"])
	}
	if floors["imp3"] != 0 {
		t.Errorf("expected imp3 floor 0, got %f", floors["imp3"])
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
