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

// AuctionType defines the type of auction to run
type AuctionType int

const (
	// FirstPriceAuction - winner pays their bid price
	FirstPriceAuction AuctionType = 1
	// SecondPriceAuction - winner pays second highest bid + increment
	SecondPriceAuction AuctionType = 2
)

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
	// Auction configuration
	AuctionType      AuctionType
	PriceIncrement   float64 // For second-price auctions (typically 0.01)
	MinBidPrice      float64 // Minimum valid bid price
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
		AuctionType:           FirstPriceAuction,
		PriceIncrement:        0.01,
		MinBidPrice:           0.0,
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

// RequestValidationError represents a bid request validation failure
type RequestValidationError struct {
	Field  string
	Reason string
}

func (e *RequestValidationError) Error() string {
	return fmt.Sprintf("invalid request: %s - %s", e.Field, e.Reason)
}

// ValidateRequest performs OpenRTB 2.x request validation
// Returns nil if valid, or a RequestValidationError describing the issue
func ValidateRequest(req *openrtb.BidRequest) *RequestValidationError {
	if req == nil {
		return &RequestValidationError{Field: "request", Reason: "nil request"}
	}

	// Validate required field: ID
	if req.ID == "" {
		return &RequestValidationError{Field: "id", Reason: "missing required field"}
	}

	// Validate required field: at least one impression
	if len(req.Imp) == 0 {
		return &RequestValidationError{Field: "imp", Reason: "at least one impression is required"}
	}

	// Validate impression IDs are unique and non-empty
	impIDs := make(map[string]struct{}, len(req.Imp))
	for i, imp := range req.Imp {
		if imp.ID == "" {
			return &RequestValidationError{
				Field:  fmt.Sprintf("imp[%d].id", i),
				Reason: "impression ID is required",
			}
		}
		if _, exists := impIDs[imp.ID]; exists {
			return &RequestValidationError{
				Field:  fmt.Sprintf("imp[%d].id", i),
				Reason: fmt.Sprintf("duplicate impression ID: %s", imp.ID),
			}
		}
		impIDs[imp.ID] = struct{}{}
	}

	// Validate Site XOR App (exactly one must be present, not both, not neither)
	hasSite := req.Site != nil
	hasApp := req.App != nil
	if hasSite && hasApp {
		return &RequestValidationError{
			Field:  "site/app",
			Reason: "request cannot contain both site and app objects",
		}
	}
	if !hasSite && !hasApp {
		return &RequestValidationError{
			Field:  "site/app",
			Reason: "request must contain either site or app object",
		}
	}

	// Validate TMax if present (reasonable bounds: 0 means no limit, otherwise 10ms-30000ms)
	if req.TMax < 0 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax cannot be negative: %d", req.TMax),
		}
	}
	if req.TMax > 0 && req.TMax < 10 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax too small (minimum 10ms): %d", req.TMax),
		}
	}
	if req.TMax > 30000 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax too large (maximum 30000ms): %d", req.TMax),
		}
	}

	return nil
}

// BidValidationError represents a bid validation failure
type BidValidationError struct {
	BidID   string
	ImpID   string
	Reason  string
	BidderCode string
}

func (e *BidValidationError) Error() string {
	return fmt.Sprintf("invalid bid from %s (bid=%s, imp=%s): %s", e.BidderCode, e.BidID, e.ImpID, e.Reason)
}

// validateBid checks if a bid meets OpenRTB requirements and exchange rules
func (e *Exchange) validateBid(bid *openrtb.Bid, bidderCode string, impIDs map[string]float64) *BidValidationError {
	if bid == nil {
		return &BidValidationError{BidderCode: bidderCode, Reason: "nil bid"}
	}

	// Check required field: Bid.ID
	if bid.ID == "" {
		return &BidValidationError{
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     "missing required field: id",
		}
	}

	// Check required field: Bid.ImpID
	if bid.ImpID == "" {
		return &BidValidationError{
			BidID:      bid.ID,
			BidderCode: bidderCode,
			Reason:     "missing required field: impid",
		}
	}

	// Validate ImpID exists in request
	floor, validImp := impIDs[bid.ImpID]
	if !validImp {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("impid %q not found in request", bid.ImpID),
		}
	}

	// Check price is non-negative
	if bid.Price < 0 {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("negative price: %.4f", bid.Price),
		}
	}

	// Check price meets minimum
	if bid.Price < e.config.MinBidPrice {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("price %.4f below minimum %.4f", bid.Price, e.config.MinBidPrice),
		}
	}

	// Check price meets floor
	if floor > 0 && bid.Price < floor {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("price %.4f below floor %.4f", bid.Price, floor),
		}
	}

	return nil
}

// buildImpFloorMap creates a map of impression IDs to their floor prices
func buildImpFloorMap(req *openrtb.BidRequest) map[string]float64 {
	impFloors := make(map[string]float64, len(req.Imp))
	for _, imp := range req.Imp {
		impFloors[imp.ID] = imp.BidFloor
	}
	return impFloors
}

// ValidatedBid wraps a bid with validation status
type ValidatedBid struct {
	Bid        *adapters.TypedBid
	BidderCode string
}

// runAuctionLogic applies auction rules (first-price or second-price) to validated bids
// Returns bids grouped by impression with prices adjusted according to auction type
func (e *Exchange) runAuctionLogic(validBids []ValidatedBid) map[string][]ValidatedBid {
	// Group bids by impression
	bidsByImp := make(map[string][]ValidatedBid)
	for _, vb := range validBids {
		impID := vb.Bid.Bid.ImpID
		bidsByImp[impID] = append(bidsByImp[impID], vb)
	}

	// Apply auction logic per impression
	for impID, bids := range bidsByImp {
		if len(bids) == 0 {
			continue
		}

		// Sort by price descending
		sortBidsByPrice(bids)

		if e.config.AuctionType == SecondPriceAuction && len(bids) > 1 {
			// Second-price: winner pays second highest + increment
			secondPrice := bids[1].Bid.Bid.Price
			winningPrice := secondPrice + e.config.PriceIncrement
			// Ensure winning price doesn't exceed original bid
			if winningPrice > bids[0].Bid.Bid.Price {
				winningPrice = bids[0].Bid.Bid.Price
			}
			bids[0].Bid.Bid.Price = winningPrice
		}
		// First-price: winner pays their bid (no adjustment needed)

		bidsByImp[impID] = bids
	}

	return bidsByImp
}

// sortBidsByPrice sorts bids in descending order by price (highest first)
func sortBidsByPrice(bids []ValidatedBid) {
	// Simple insertion sort - typically small number of bids per impression
	for i := 1; i < len(bids); i++ {
		j := i
		for j > 0 && bids[j].Bid.Bid.Price > bids[j-1].Bid.Bid.Price {
			bids[j], bids[j-1] = bids[j-1], bids[j]
			j--
		}
	}
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

	// Validate the bid request per OpenRTB 2.x specification
	if validationErr := ValidateRequest(req.BidRequest); validationErr != nil {
		response.DebugInfo.TotalLatency = time.Since(startTime)
		return response, validationErr
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

	// Build impression floor map for bid validation
	impFloors := buildImpFloorMap(req.BidRequest)

	// Track seen bid IDs for deduplication
	seenBidIDs := make(map[string]struct{})

	// Collect and validate all bids
	var validBids []ValidatedBid
	var validationErrors []error

	// Collect results
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

		// Validate and deduplicate bids
		for _, tb := range result.Bids {
			// Skip nil bids
			if tb == nil || tb.Bid == nil {
				continue
			}

			// Validate bid
			if validErr := e.validateBid(tb.Bid, bidderCode, impFloors); validErr != nil {
				validationErrors = append(validationErrors, validErr)
				response.DebugInfo.AppendError(bidderCode, validErr.Error())
				continue
			}

			// Check for duplicate bid IDs
			if _, seen := seenBidIDs[tb.Bid.ID]; seen {
				dupErr := &BidValidationError{
					BidID:      tb.Bid.ID,
					ImpID:      tb.Bid.ImpID,
					BidderCode: bidderCode,
					Reason:     "duplicate bid ID",
				}
				validationErrors = append(validationErrors, dupErr)
				response.DebugInfo.AppendError(bidderCode, dupErr.Error())
				continue
			}
			seenBidIDs[tb.Bid.ID] = struct{}{}

			// Add to valid bids
			validBids = append(validBids, ValidatedBid{
				Bid:        tb,
				BidderCode: bidderCode,
			})
		}
	}

	// Apply auction logic (first-price or second-price)
	auctionedBids := e.runAuctionLogic(validBids)

	// Build seat bids from auctioned results
	seatBidMap := make(map[string]*openrtb.SeatBid)
	for _, impBids := range auctionedBids {
		for _, vb := range impBids {
			sb, ok := seatBidMap[vb.BidderCode]
			if !ok {
				sb = &openrtb.SeatBid{
					Seat: vb.BidderCode,
					Bid:  []openrtb.Bid{},
				}
				seatBidMap[vb.BidderCode] = sb
			}
			sb.Bid = append(sb.Bid, *vb.Bid.Bid)
		}
	}

	// Convert seat bid map to slice
	allBids := make([]openrtb.SeatBid, 0, len(seatBidMap))
	for _, sb := range seatBidMap {
		allBids = append(allBids, *sb)
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
			// Validate BidResponse.ID matches BidRequest.ID (OpenRTB 2.x requirement)
			if bidderResp.ResponseID != "" && bidderResp.ResponseID != req.ID {
				result.Errors = append(result.Errors, fmt.Errorf(
					"response ID mismatch from %s: expected %q, got %q",
					bidderCode, req.ID, bidderResp.ResponseID,
				))
				// Still collect bids but log the mismatch as a warning
			}
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
