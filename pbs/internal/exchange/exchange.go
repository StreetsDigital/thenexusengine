// Package exchange implements the auction exchange
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/ortb"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/fpd"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/idr"
)

// Exchange orchestrates the auction process
type Exchange struct {
	registry         *adapters.Registry
	dynamicRegistry  *ortb.DynamicRegistry
	httpClient       adapters.HTTPClient
	idrClient        *idr.Client
	eventRecorder    *idr.EventRecorder
	config           *Config
	fpdProcessor     *fpd.Processor
	eidFilter        *fpd.EIDFilter
}

// Config holds exchange configuration
type Config struct {
	DefaultTimeout     time.Duration
	MaxBidders         int
	IDREnabled         bool
	IDRServiceURL      string
	EventRecordEnabled bool
	EventBufferSize    int
	CurrencyConv       bool
	DefaultCurrency    string
	FPD                *fpd.Config
	// Dynamic bidder configuration
	DynamicBiddersEnabled bool
	DynamicRefreshPeriod  time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:        1000 * time.Millisecond,
		MaxBidders:            50,
		IDREnabled:            true,
		IDRServiceURL:         "http://localhost:5050",
		EventRecordEnabled:    true,
		EventBufferSize:       100,
		CurrencyConv:          false,
		DefaultCurrency:       "USD",
		FPD:                   fpd.DefaultConfig(),
		DynamicBiddersEnabled: true,
		DynamicRefreshPeriod:  30 * time.Second,
	}
}

// New creates a new exchange
func New(registry *adapters.Registry, config *Config) *Exchange {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize FPD config if not provided
	fpdConfig := config.FPD
	if fpdConfig == nil {
		fpdConfig = fpd.DefaultConfig()
	}

	ex := &Exchange{
		registry:     registry,
		httpClient:   adapters.NewHTTPClient(config.DefaultTimeout),
		config:       config,
		fpdProcessor: fpd.NewProcessor(fpdConfig),
		eidFilter:    fpd.NewEIDFilter(fpdConfig),
	}

	if config.IDREnabled && config.IDRServiceURL != "" {
		ex.idrClient = idr.NewClient(config.IDRServiceURL, 50*time.Millisecond)
	}

	if config.EventRecordEnabled && config.IDRServiceURL != "" {
		ex.eventRecorder = idr.NewEventRecorder(config.IDRServiceURL, config.EventBufferSize)
	}

	return ex
}

// SetDynamicRegistry sets the dynamic bidder registry
func (e *Exchange) SetDynamicRegistry(dr *ortb.DynamicRegistry) {
	e.dynamicRegistry = dr
}

// GetDynamicRegistry returns the dynamic registry
func (e *Exchange) GetDynamicRegistry() *ortb.DynamicRegistry {
	return e.dynamicRegistry
}

// Close shuts down the exchange and flushes pending events
func (e *Exchange) Close() error {
	if e.eventRecorder != nil {
		return e.eventRecorder.Close()
	}
	return nil
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
	errorsMu          sync.Mutex // Protects concurrent access to Errors map
}

// AddError safely adds errors to the Errors map with mutex protection
func (d *DebugInfo) AddError(key string, errors []string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.Errors[key] = errors
}

// AppendError safely appends an error to the Errors map with mutex protection
func (d *DebugInfo) AppendError(key string, errMsg string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.Errors[key] = append(d.Errors[key], errMsg)
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

	// Get available bidders from static registry
	availableBidders := e.registry.ListEnabledBidders()

	// Add dynamic bidders if enabled
	if e.config.DynamicBiddersEnabled && e.dynamicRegistry != nil {
		dynamicCodes := e.dynamicRegistry.ListEnabledBidderCodes()
		availableBidders = append(availableBidders, dynamicCodes...)
	}

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

	// Process FPD and filter EIDs
	var bidderFPD fpd.BidderFPD
	if e.fpdProcessor != nil {
		// Filter EIDs first
		if e.eidFilter != nil {
			e.eidFilter.ProcessRequestEIDs(req.BidRequest)
		}

		// Process FPD for each bidder
		var err error
		bidderFPD, err = e.fpdProcessor.ProcessRequest(req.BidRequest, selectedBidders)
		if err != nil {
			// Log error but continue - FPD is not critical
			response.DebugInfo.AddError("fpd", []string{err.Error()})
		}
	}

	// Call bidders in parallel
	results := e.callBiddersWithFPD(ctx, req.BidRequest, selectedBidders, timeout, bidderFPD)

	// Extract request context for event recording
	var country, deviceType, mediaType, adSize, publisherID string
	if req.BidRequest.Device != nil && req.BidRequest.Device.Geo != nil {
		country = req.BidRequest.Device.Geo.Country
	}
	if req.BidRequest.Device != nil {
		switch req.BidRequest.Device.DeviceType {
		case 1:
			deviceType = "mobile"
		case 2:
			deviceType = "desktop"
		case 3:
			deviceType = "ctv"
		default:
			deviceType = "unknown"
		}
	}
	if len(req.BidRequest.Imp) > 0 {
		imp := req.BidRequest.Imp[0]
		if imp.Banner != nil {
			mediaType = "banner"
			if imp.Banner.W > 0 && imp.Banner.H > 0 {
				adSize = fmt.Sprintf("%dx%d", imp.Banner.W, imp.Banner.H)
			}
		} else if imp.Video != nil {
			mediaType = "video"
		} else if imp.Native != nil {
			mediaType = "native"
		}
	}
	if req.BidRequest.Site != nil && req.BidRequest.Site.Publisher != nil {
		publisherID = req.BidRequest.Site.Publisher.ID
	}

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
			response.DebugInfo.AddError(bidderCode, errStrs)
		}

		// Record event to IDR
		if e.eventRecorder != nil {
			hadBid := len(result.Bids) > 0
			var bidCPM *float64
			if hadBid && len(result.Bids) > 0 {
				cpm := result.Bids[0].Bid.Price
				bidCPM = &cpm
			}
			hadError := len(result.Errors) > 0
			var errorMsg string
			if hadError {
				errorMsg = result.Errors[0].Error()
			}

			e.eventRecorder.RecordBidResponse(
				req.BidRequest.ID,
				bidderCode,
				float64(result.Latency.Milliseconds()),
				hadBid,
				bidCPM,
				nil, // floor price
				country,
				deviceType,
				mediaType,
				adSize,
				publisherID,
				false, // timedOut - would need to check context
				hadError,
				errorMsg,
			)
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

// callBidders calls all selected bidders in parallel (legacy, without FPD)
func (e *Exchange) callBidders(ctx context.Context, req *openrtb.BidRequest, bidders []string, timeout time.Duration) map[string]*BidderResult {
	return e.callBiddersWithFPD(ctx, req, bidders, timeout, nil)
}

// callBiddersWithFPD calls all selected bidders in parallel with FPD support
func (e *Exchange) callBiddersWithFPD(ctx context.Context, req *openrtb.BidRequest, bidders []string, timeout time.Duration, bidderFPD fpd.BidderFPD) map[string]*BidderResult {
	results := make(map[string]*BidderResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, bidderCode := range bidders {
		// Try static registry first
		adapterWithInfo, ok := e.registry.Get(bidderCode)
		if ok {
			wg.Add(1)
			go func(code string, awi adapters.AdapterWithInfo) {
				defer wg.Done()

				// Clone request and apply bidder-specific FPD
				bidderReq := e.cloneRequestWithFPD(req, code, bidderFPD)

				result := e.callBidder(ctx, bidderReq, code, awi.Adapter, timeout)

				mu.Lock()
				results[code] = result
				mu.Unlock()
			}(bidderCode, adapterWithInfo)
			continue
		}

		// Try dynamic registry
		if e.dynamicRegistry != nil {
			dynamicAdapter, found := e.dynamicRegistry.Get(bidderCode)
			if found {
				wg.Add(1)
				go func(code string, da *ortb.GenericAdapter) {
					defer wg.Done()

					// Clone request and apply bidder-specific FPD
					bidderReq := e.cloneRequestWithFPD(req, code, bidderFPD)

					// Use dynamic adapter's timeout if smaller
					bidderTimeout := timeout
					if da.GetTimeout() > 0 && da.GetTimeout() < timeout {
						bidderTimeout = da.GetTimeout()
					}

					result := e.callBidder(ctx, bidderReq, code, da, bidderTimeout)

					mu.Lock()
					results[code] = result
					mu.Unlock()
				}(bidderCode, dynamicAdapter)
			}
		}
	}

	wg.Wait()
	return results
}

// cloneRequestWithFPD creates a deep copy of the request with bidder-specific FPD applied
// and enforces USD currency for all bid requests
func (e *Exchange) cloneRequestWithFPD(req *openrtb.BidRequest, bidderCode string, bidderFPD fpd.BidderFPD) *openrtb.BidRequest {
	// Always clone to ensure thread safety and allow currency normalization
	clone := deepCloneRequest(req)

	// Enforce USD currency on all outgoing requests
	// This ensures all bidders compete in the same currency without needing forex conversion
	clone.Cur = []string{e.config.DefaultCurrency}

	// Normalize bid floor currencies to USD
	for i := range clone.Imp {
		if clone.Imp[i].BidFloorCur == "" || clone.Imp[i].BidFloorCur != e.config.DefaultCurrency {
			// If floor currency differs from USD and we had conversion, we'd convert here
			// For now, we just set the currency - publishers should specify floors in USD
			clone.Imp[i].BidFloorCur = e.config.DefaultCurrency
		}
	}

	// Apply FPD if available
	if bidderFPD != nil {
		if fpdData, ok := bidderFPD[bidderCode]; ok && fpdData != nil {
			if e.fpdProcessor != nil {
				e.fpdProcessor.ApplyFPDToRequest(clone, bidderCode, fpdData)
			}
		}
	}

	return clone
}

// deepCloneRequest creates a deep copy of the BidRequest to avoid race conditions
// when multiple bidders modify request data concurrently
func deepCloneRequest(req *openrtb.BidRequest) *openrtb.BidRequest {
	clone := *req

	// Deep copy Site
	if req.Site != nil {
		siteCopy := *req.Site
		if req.Site.Publisher != nil {
			pubCopy := *req.Site.Publisher
			siteCopy.Publisher = &pubCopy
		}
		if req.Site.Content != nil {
			contentCopy := *req.Site.Content
			siteCopy.Content = &contentCopy
		}
		clone.Site = &siteCopy
	}

	// Deep copy App
	if req.App != nil {
		appCopy := *req.App
		if req.App.Publisher != nil {
			pubCopy := *req.App.Publisher
			appCopy.Publisher = &pubCopy
		}
		if req.App.Content != nil {
			contentCopy := *req.App.Content
			appCopy.Content = &contentCopy
		}
		clone.App = &appCopy
	}

	// Deep copy User
	if req.User != nil {
		userCopy := *req.User
		if req.User.Geo != nil {
			geoCopy := *req.User.Geo
			userCopy.Geo = &geoCopy
		}
		// Deep copy EIDs slice
		if len(req.User.EIDs) > 0 {
			userCopy.EIDs = make([]openrtb.EID, len(req.User.EIDs))
			copy(userCopy.EIDs, req.User.EIDs)
		}
		// Deep copy Data slice
		if len(req.User.Data) > 0 {
			userCopy.Data = make([]openrtb.Data, len(req.User.Data))
			copy(userCopy.Data, req.User.Data)
		}
		clone.User = &userCopy
	}

	// Deep copy Device
	if req.Device != nil {
		deviceCopy := *req.Device
		if req.Device.Geo != nil {
			geoCopy := *req.Device.Geo
			deviceCopy.Geo = &geoCopy
		}
		clone.Device = &deviceCopy
	}

	// Deep copy Regs
	if req.Regs != nil {
		regsCopy := *req.Regs
		clone.Regs = &regsCopy
	}

	// Deep copy Source
	if req.Source != nil {
		sourceCopy := *req.Source
		if req.Source.SChain != nil {
			schainCopy := *req.Source.SChain
			if len(req.Source.SChain.Nodes) > 0 {
				schainCopy.Nodes = make([]openrtb.SupplyChainNode, len(req.Source.SChain.Nodes))
				copy(schainCopy.Nodes, req.Source.SChain.Nodes)
			}
			sourceCopy.SChain = &schainCopy
		}
		clone.Source = &sourceCopy
	}

	// Deep copy Imp slice
	if len(req.Imp) > 0 {
		clone.Imp = make([]openrtb.Imp, len(req.Imp))
		for i, imp := range req.Imp {
			impCopy := imp
			if imp.Banner != nil {
				bannerCopy := *imp.Banner
				impCopy.Banner = &bannerCopy
			}
			if imp.Video != nil {
				videoCopy := *imp.Video
				impCopy.Video = &videoCopy
			}
			if imp.Audio != nil {
				audioCopy := *imp.Audio
				impCopy.Audio = &audioCopy
			}
			if imp.Native != nil {
				nativeCopy := *imp.Native
				impCopy.Native = &nativeCopy
			}
			if imp.PMP != nil {
				pmpCopy := *imp.PMP
				if len(imp.PMP.Deals) > 0 {
					pmpCopy.Deals = make([]openrtb.Deal, len(imp.PMP.Deals))
					copy(pmpCopy.Deals, imp.PMP.Deals)
				}
				impCopy.PMP = &pmpCopy
			}
			clone.Imp[i] = impCopy
		}
	}

	return &clone
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
		// Check if context has expired before each request to avoid wasted work
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			result.Latency = time.Since(start)
			return result
		default:
			// Context still valid, proceed with request
		}

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

// UpdateFPDConfig updates the FPD configuration at runtime
func (e *Exchange) UpdateFPDConfig(config *fpd.Config) {
	if config == nil {
		return
	}

	e.config.FPD = config
	e.fpdProcessor = fpd.NewProcessor(config)
	e.eidFilter = fpd.NewEIDFilter(config)
}

// GetFPDConfig returns the current FPD configuration
func (e *Exchange) GetFPDConfig() *fpd.Config {
	if e.config == nil {
		return nil
	}
	return e.config.FPD
}

// GetIDRClient returns the IDR client (for metrics/admin)
func (e *Exchange) GetIDRClient() *idr.Client {
	return e.idrClient
}
