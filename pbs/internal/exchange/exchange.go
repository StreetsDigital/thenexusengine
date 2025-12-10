// Package exchange implements the auction exchange
package exchange

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/idr"
)

// Exchange orchestrates the auction process
type Exchange struct {
	registry   *adapters.Registry
	httpClient adapters.HTTPClient
	idrClient  *idr.Client
	config     *Config
}

// Config holds exchange configuration
type Config struct {
	DefaultTimeout    time.Duration
	MaxBidders        int
	IDREnabled        bool
	IDRServiceURL     string
	CurrencyConv      bool
	DefaultCurrency   string
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:  1000 * time.Millisecond,
		MaxBidders:      50,
		IDREnabled:      true,
		IDRServiceURL:   "http://localhost:5050",
		CurrencyConv:    false,
		DefaultCurrency: "USD",
	}
}

// New creates a new exchange
func New(registry *adapters.Registry, config *Config) *Exchange {
	if config == nil {
		config = DefaultConfig()
	}

	ex := &Exchange{
		registry:   registry,
		httpClient: adapters.NewHTTPClient(config.DefaultTimeout),
		config:     config,
	}

	if config.IDREnabled && config.IDRServiceURL != "" {
		ex.idrClient = idr.NewClient(config.IDRServiceURL, 50*time.Millisecond)
	}

	return ex
}

// AuctionRequest contains auction parameters
type AuctionRequest struct {
	BidRequest *openrtb.BidRequest
	Timeout    time.Duration
	Account    string
	Debug      bool
}

// AuctionResponse contains auction results
type AuctionResponse struct {
	BidResponse   *openrtb.BidResponse
	BidderResults map[string]*BidderResult
	IDRResult     *idr.SelectPartnersResponse
	DebugInfo     *DebugInfo
}

// BidderResult contains results from a single bidder
type BidderResult struct {
	BidderCode string
	Bids       []*adapters.TypedBid
	Errors     []error
	Latency    time.Duration
	Selected   bool
	Score      float64
}

// DebugInfo contains debug information
type DebugInfo struct {
	RequestTime       time.Time
	TotalLatency      time.Duration
	IDRLatency        time.Duration
	BidderLatencies   map[string]time.Duration
	SelectedBidders   []string
	ExcludedBidders   []string
	Errors            map[string][]string
}

// RunAuction executes the auction
func (e *Exchange) RunAuction(ctx context.Context, req *AuctionRequest) (*AuctionResponse, error) {
	startTime := time.Now()

	response := &AuctionResponse{
		BidderResults: make(map[string]*BidderResult),
		DebugInfo: &DebugInfo{
			RequestTime:     startTime,
			BidderLatencies: make(map[string]time.Duration),
			Errors:          make(map[string][]string),
		},
	}

	// Get timeout from request or config
	timeout := req.Timeout
	if timeout == 0 && req.BidRequest.TMax > 0 {
		timeout = time.Duration(req.BidRequest.TMax) * time.Millisecond
	}
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get available bidders
	availableBidders := e.registry.ListEnabledBidders()
	if len(availableBidders) == 0 {
		response.BidResponse = e.buildEmptyResponse(req.BidRequest)
		return response, nil
	}

	// Run IDR selection if enabled
	selectedBidders := availableBidders
	if e.idrClient != nil && e.config.IDREnabled {
		idrStart := time.Now()

		reqJSON, _ := json.Marshal(req.BidRequest)
		idrResult, err := e.idrClient.SelectPartners(ctx, reqJSON, availableBidders)

		response.DebugInfo.IDRLatency = time.Since(idrStart)

		if err == nil && idrResult != nil {
			response.IDRResult = idrResult
			selectedBidders = make([]string, 0, len(idrResult.SelectedBidders))
			for _, sb := range idrResult.SelectedBidders {
				selectedBidders = append(selectedBidders, sb.BidderCode)
			}

			for _, eb := range idrResult.ExcludedBidders {
				response.DebugInfo.ExcludedBidders = append(response.DebugInfo.ExcludedBidders, eb.BidderCode)
			}
		}
		// If IDR fails, fall back to all bidders
	}

	response.DebugInfo.SelectedBidders = selectedBidders

	// Call bidders in parallel
	results := e.callBidders(ctx, req.BidRequest, selectedBidders, timeout)

	// Collect results
	allBids := make([]openrtb.SeatBid, 0)
	for bidderCode, result := range results {
		response.BidderResults[bidderCode] = result
		response.DebugInfo.BidderLatencies[bidderCode] = result.Latency

		if len(result.Errors) > 0 {
			errStrs := make([]string, len(result.Errors))
			for i, err := range result.Errors {
				errStrs[i] = err.Error()
			}
			response.DebugInfo.Errors[bidderCode] = errStrs
		}

		if len(result.Bids) > 0 {
			seatBid := openrtb.SeatBid{
				Seat: bidderCode,
				Bid:  make([]openrtb.Bid, len(result.Bids)),
			}
			for i, tb := range result.Bids {
				seatBid.Bid[i] = *tb.Bid
			}
			allBids = append(allBids, seatBid)
		}
	}

	// Build response
	response.BidResponse = &openrtb.BidResponse{
		ID:      req.BidRequest.ID,
		SeatBid: allBids,
		Cur:     e.config.DefaultCurrency,
	}

	response.DebugInfo.TotalLatency = time.Since(startTime)

	return response, nil
}

// callBidders calls all selected bidders in parallel
func (e *Exchange) callBidders(ctx context.Context, req *openrtb.BidRequest, bidders []string, timeout time.Duration) map[string]*BidderResult {
	results := make(map[string]*BidderResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, bidderCode := range bidders {
		adapterWithInfo, ok := e.registry.Get(bidderCode)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(code string, awi adapters.AdapterWithInfo) {
			defer wg.Done()

			result := e.callBidder(ctx, req, code, awi.Adapter, timeout)

			mu.Lock()
			results[code] = result
			mu.Unlock()
		}(bidderCode, adapterWithInfo)
	}

	wg.Wait()
	return results
}

// callBidder calls a single bidder
func (e *Exchange) callBidder(ctx context.Context, req *openrtb.BidRequest, bidderCode string, adapter adapters.Adapter, timeout time.Duration) *BidderResult {
	start := time.Now()
	result := &BidderResult{
		BidderCode: bidderCode,
		Selected:   true,
	}

	// Build requests
	extraInfo := &adapters.ExtraRequestInfo{
		BidderCoreName: bidderCode,
	}

	requests, errs := adapter.MakeRequests(req, extraInfo)
	if len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	if len(requests) == 0 {
		result.Latency = time.Since(start)
		return result
	}

	// Execute requests (could parallelize for multi-request adapters)
	allBids := make([]*adapters.TypedBid, 0)
	for _, reqData := range requests {
		resp, err := e.httpClient.Do(ctx, reqData, timeout)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		bidderResp, errs := adapter.MakeBids(req, resp)
		if len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
		}

		if bidderResp != nil {
			allBids = append(allBids, bidderResp.Bids...)
		}
	}

	result.Bids = allBids
	result.Latency = time.Since(start)
	return result
}

// buildEmptyResponse creates an empty bid response
func (e *Exchange) buildEmptyResponse(req *openrtb.BidRequest) *openrtb.BidResponse {
	return &openrtb.BidResponse{
		ID:      req.ID,
		SeatBid: []openrtb.SeatBid{},
		Cur:     e.config.DefaultCurrency,
	}
}
