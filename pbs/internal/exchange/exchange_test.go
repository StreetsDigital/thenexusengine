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
			Imp: []openrtb.Imp{
				{ID: "imp1"},
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
	}
	mock := &mockAdapter{
		bids: []*adapters.TypedBid{
			{Bid: mockBid, BidType: adapters.BidTypeBanner},
		},
	}

	registry.Register("test-bidder", mock, adapters.BidderInfo{
		Enabled: true,
	})

	// Create mock HTTP server for bidder endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid bid response
		resp := &openrtb.BidResponse{
			ID: "resp1",
			SeatBid: []openrtb.SeatBid{
				{
					Bid: []openrtb.Bid{*mockBid},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ex := New(registry, &Config{
		DefaultTimeout: 500 * time.Millisecond,
		IDREnabled:     false,
	})

	req := &AuctionRequest{
		BidRequest: &openrtb.BidRequest{
			ID: "test-req-2",
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
				{ID: "imp1"},
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
			User: &openrtb.User{
				ID: "user1",
				EIDs: []openrtb.EID{
					{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "lr123"}}},
					{Source: "blocked.com", UIDs: []openrtb.UID{{ID: "blk456"}}},
				},
			},
			Imp: []openrtb.Imp{{ID: "imp1"}},
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
			Imp:  []openrtb.Imp{{ID: "imp1"}},
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
			ID:  "test-debug",
			Imp: []openrtb.Imp{{ID: "imp1"}},
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
}
