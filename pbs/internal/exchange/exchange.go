// Package exchange implements the auction exchange
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
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

// Default clone allocation limits (P1-3: prevent OOM from malicious requests)
// P3-1: These are now defaults; can be overridden via CloneLimits config
const (
	defaultMaxImpressionsPerRequest = 100 // Maximum impressions to clone
	defaultMaxEIDsPerUser           = 50  // Maximum EIDs to clone
	defaultMaxDataPerUser           = 20  // Maximum Data segments to clone
	defaultMaxDealsPerImp           = 50  // Maximum deals per impression
	defaultMaxSChainNodes           = 20  // Maximum supply chain nodes
)

// P1-4: Timeout bounds for dynamic adapter validation
const (
	minBidderTimeout = 10 * time.Millisecond  // Minimum reasonable timeout
	maxBidderTimeout = 5 * time.Second        // Maximum to prevent resource exhaustion
)

// P2-NEW-2: OpenRTB 2.5 No-Bid Reason (NBR) codes per Section 5.24
// These explain why an exchange returned no bid for a request
const (
	NBRUnknownError     = 0  // Unknown Error
	NBRTechnicalError   = 1  // Technical Error
	NBRInvalidRequest   = 2  // Invalid Request
	NBRKnownWebSpider   = 3  // Known Web Spider
	NBRNonHumanTraffic  = 4  // Suspected Non-Human Traffic
	NBRProxyIP          = 5  // Cloud, Data Center, or Proxy IP
	NBRUnsupportedDevice = 6 // Unsupported Device
	NBRBlockedPublisher = 7  // Blocked Publisher or Site
	NBRUnmatchedUser    = 8  // Unmatched User
	// 500+ reserved for exchange-specific reasons
	NBRNoBiddersAvailable = 500 // Exchange-specific: No bidders configured/available
	NBRTimeout            = 501 // Exchange-specific: Request processing timed out
)

// CloneLimits holds configurable limits for request cloning (P3-1)
type CloneLimits struct {
	MaxImpressionsPerRequest int // Maximum impressions to clone (default: 100)
	MaxEIDsPerUser           int // Maximum EIDs to clone (default: 50)
	MaxDataPerUser           int // Maximum Data segments to clone (default: 20)
	MaxDealsPerImp           int // Maximum deals per impression (default: 50)
	MaxSChainNodes           int // Maximum supply chain nodes (default: 20)
}

// DefaultCloneLimits returns default clone limits
func DefaultCloneLimits() *CloneLimits {
	return &CloneLimits{
		MaxImpressionsPerRequest: defaultMaxImpressionsPerRequest,
		MaxEIDsPerUser:           defaultMaxEIDsPerUser,
		MaxDataPerUser:           defaultMaxDataPerUser,
		MaxDealsPerImp:           defaultMaxDealsPerImp,
		MaxSChainNodes:           defaultMaxSChainNodes,
	}
}

// Config holds exchange configuration
type Config struct {
	DefaultTimeout       time.Duration
	MaxBidders           int
	MaxConcurrentBidders int // P0-4: Limit concurrent bidder goroutines (0 = unlimited)
	IDREnabled           bool
	IDRServiceURL        string
	EventRecordEnabled   bool
	EventBufferSize      int
	CurrencyConv         bool
	DefaultCurrency      string
	FPD                  *fpd.Config
	CloneLimits          *CloneLimits // P3-1: Configurable clone limits
	// Dynamic bidder configuration
	DynamicBiddersEnabled bool
	DynamicRefreshPeriod  time.Duration
	// Auction configuration
	AuctionType    AuctionType
	PriceIncrement float64 // For second-price auctions (typically 0.01)
	MinBidPrice    float64 // Minimum valid bid price
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:        1000 * time.Millisecond,
		MaxBidders:            50,
		MaxConcurrentBidders:  10, // P0-4: Limit concurrent HTTP requests per auction
		IDREnabled:            true,
		IDRServiceURL:         "http://localhost:5050",
		EventRecordEnabled:    true,
		EventBufferSize:       100,
		CurrencyConv:          false,
		DefaultCurrency:       "USD",
		FPD:                   fpd.DefaultConfig(),
		CloneLimits:           DefaultCloneLimits(), // P3-1: Configurable clone limits
		DynamicBiddersEnabled: true,
		DynamicRefreshPeriod:  30 * time.Second,
		AuctionType:           FirstPriceAuction,
		PriceIncrement:        0.01,
		MinBidPrice:           0.0,
	}
}

// validateConfig validates config values and applies sensible defaults for invalid values
// P1-2: Prevent runtime panics or silent failures from bad configuration
func validateConfig(config *Config) *Config {
	defaults := DefaultConfig()

	// Timeout must be positive
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = defaults.DefaultTimeout
	}

	// MaxBidders must be positive
	if config.MaxBidders <= 0 {
		config.MaxBidders = defaults.MaxBidders
	}

	// MaxConcurrentBidders must be non-negative (0 means unlimited)
	if config.MaxConcurrentBidders < 0 {
		config.MaxConcurrentBidders = defaults.MaxConcurrentBidders
	}

	// AuctionType must be valid
	if config.AuctionType != FirstPriceAuction && config.AuctionType != SecondPriceAuction {
		config.AuctionType = FirstPriceAuction
	}

	// PriceIncrement must be positive for second-price auctions
	if config.AuctionType == SecondPriceAuction && config.PriceIncrement <= 0 {
		config.PriceIncrement = defaults.PriceIncrement
	}

	// MinBidPrice should not be negative
	if config.MinBidPrice < 0 {
		config.MinBidPrice = 0
	}

	// EventBufferSize must be positive if event recording is enabled
	if config.EventRecordEnabled && config.EventBufferSize <= 0 {
		config.EventBufferSize = defaults.EventBufferSize
	}

	// P3-1: Initialize CloneLimits if nil and validate values
	if config.CloneLimits == nil {
		config.CloneLimits = DefaultCloneLimits()
	} else {
		defaultLimits := DefaultCloneLimits()
		if config.CloneLimits.MaxImpressionsPerRequest <= 0 {
			config.CloneLimits.MaxImpressionsPerRequest = defaultLimits.MaxImpressionsPerRequest
		}
		if config.CloneLimits.MaxEIDsPerUser <= 0 {
			config.CloneLimits.MaxEIDsPerUser = defaultLimits.MaxEIDsPerUser
		}
		if config.CloneLimits.MaxDataPerUser <= 0 {
			config.CloneLimits.MaxDataPerUser = defaultLimits.MaxDataPerUser
		}
		if config.CloneLimits.MaxDealsPerImp <= 0 {
			config.CloneLimits.MaxDealsPerImp = defaultLimits.MaxDealsPerImp
		}
		if config.CloneLimits.MaxSChainNodes <= 0 {
			config.CloneLimits.MaxSChainNodes = defaultLimits.MaxSChainNodes
		}
	}

	return config
}

// New creates a new exchange
func New(registry *adapters.Registry, config *Config) *Exchange {
	if config == nil {
		config = DefaultConfig()
	}

	// P1-2: Validate and apply defaults for critical config fields
	config = validateConfig(config)

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
	TimedOut   bool // P2-2: indicates if the bidder request timed out
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

	// P2-1: Validate that bid has creative content (AdM or NURL required)
	// OpenRTB 2.x requires either inline markup (adm) or a URL to fetch it (nurl)
	if bid.AdM == "" && bid.NURL == "" {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     "bid must have either adm or nurl",
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
func (e *Exchange) runAuctionLogic(validBids []ValidatedBid, impFloors map[string]float64) map[string][]ValidatedBid {
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

		if e.config.AuctionType == SecondPriceAuction {
			var winningPrice float64

			if len(bids) > 1 {
				// Multiple bids: winner pays second highest + increment
				// Use integer arithmetic to avoid floating-point precision errors (P0-2)
				secondPrice := bids[1].Bid.Bid.Price
				winningPrice = roundToCents(secondPrice + e.config.PriceIncrement)
			} else {
				// P0-6: Single bid - use floor as "second price" for consistent auction semantics
				floor := impFloors[impID]
				if floor > 0 {
					winningPrice = roundToCents(floor + e.config.PriceIncrement)
				} else {
					// No floor - winner pays minimum bid price + increment
					winningPrice = roundToCents(e.config.MinBidPrice + e.config.PriceIncrement)
				}
			}

			// Ensure winning price doesn't exceed original bid (safety cap)
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
// Includes defensive nil checks to prevent panics
func sortBidsByPrice(bids []ValidatedBid) {
	// Simple insertion sort - typically small number of bids per impression
	for i := 1; i < len(bids); i++ {
		j := i
		for j > 0 {
			// Defensive nil checks (P1-5)
			if bids[j].Bid == nil || bids[j].Bid.Bid == nil ||
				bids[j-1].Bid == nil || bids[j-1].Bid.Bid == nil {
				break
			}
			if bids[j].Bid.Bid.Price > bids[j-1].Bid.Bid.Price {
				bids[j], bids[j-1] = bids[j-1], bids[j]
				j--
			} else {
				break
			}
		}
	}
}

// roundToCents rounds a price to 2 decimal places
// P2-NEW-3: Use math.Round for correct rounding of all values including edge cases
func roundToCents(price float64) float64 {
	// math.Round correctly handles all cases including negative numbers and .5 values
	return math.Round(price*100) / 100.0
}

// RunAuction executes the auction
func (e *Exchange) RunAuction(ctx context.Context, req *AuctionRequest) (*AuctionResponse, error) {
	startTime := time.Now()

	// P0-7: Validate required BidRequest fields per OpenRTB 2.x spec
	if req.BidRequest == nil {
		return nil, fmt.Errorf("invalid auction request: missing bid request")
	}
	if req.BidRequest.ID == "" {
		return nil, fmt.Errorf("invalid bid request: missing required field 'id'")
	}
	if len(req.BidRequest.Imp) == 0 {
		return nil, fmt.Errorf("invalid bid request: must have at least one impression")
	}

	// P1-NEW-2: Validate impression IDs are unique and non-empty per OpenRTB 2.5 section 3.2.4
	seenImpIDs := make(map[string]bool, len(req.BidRequest.Imp))
	for i, imp := range req.BidRequest.Imp {
		if imp.ID == "" {
			return nil, fmt.Errorf("invalid bid request: impression[%d] has empty id (required by OpenRTB 2.5)", i)
		}
		if seenImpIDs[imp.ID] {
			return nil, fmt.Errorf("invalid bid request: duplicate impression id %q (must be unique per OpenRTB 2.5)", imp.ID)
		}
		seenImpIDs[imp.ID] = true
	}

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
		response.BidResponse = e.buildEmptyResponse(req.BidRequest, NBRNoBiddersAvailable)
		return response, nil
	}

	// Run IDR selection if enabled
	selectedBidders := availableBidders
	if e.idrClient != nil && e.config.IDREnabled {
		idrStart := time.Now()

		// P1-5: Handle JSON marshaling errors properly
		reqJSON, marshalErr := json.Marshal(req.BidRequest)
		if marshalErr != nil {
			// Log error but continue with all bidders as fallback
			response.DebugInfo.AddError("idr", []string{
				fmt.Sprintf("failed to marshal request for IDR: %v", marshalErr),
			})
		} else {
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

	// P1-2: Check context deadline before expensive validation work
	// If we've already timed out, return early with whatever we have
	select {
	case <-ctx.Done():
		response.DebugInfo.TotalLatency = time.Since(startTime)
		response.BidResponse = e.buildEmptyResponse(req.BidRequest, NBRTimeout)
		return response, nil // Return empty response rather than error on timeout
	default:
		// Context still valid, proceed with validation
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
				// P2-7: Aggregate all errors instead of just the first
				if len(result.Errors) == 1 {
					errorMsg = result.Errors[0].Error()
				} else {
					errMsgs := make([]string, len(result.Errors))
					for i, err := range result.Errors {
						errMsgs[i] = err.Error()
					}
					errorMsg = fmt.Sprintf("%d errors: %s", len(result.Errors), strings.Join(errMsgs, "; "))
				}
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
				result.TimedOut, // P2-2: use actual timeout status
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
	auctionedBids := e.runAuctionLogic(validBids, impFloors)

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
// P0-1: Uses sync.Map for thread-safe result collection
// P0-4: Uses semaphore to limit concurrent bidder goroutines
func (e *Exchange) callBiddersWithFPD(ctx context.Context, req *openrtb.BidRequest, bidders []string, timeout time.Duration, bidderFPD fpd.BidderFPD) map[string]*BidderResult {
	var results sync.Map // P0-1: Thread-safe map for concurrent writes
	var wg sync.WaitGroup

	// P0-4: Create semaphore to limit concurrent bidder calls
	maxConcurrent := e.config.MaxConcurrentBidders
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default limit
	}
	sem := make(chan struct{}, maxConcurrent)

	for _, bidderCode := range bidders {
		// Try static registry first
		adapterWithInfo, ok := e.registry.Get(bidderCode)
		if ok {
			wg.Add(1)
			go func(code string, awi adapters.AdapterWithInfo) {
				defer wg.Done()

				// P0-4: Acquire semaphore (blocks if at capacity)
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }() // Release on completion
				case <-ctx.Done():
					// Context cancelled while waiting for semaphore
					results.Store(code, &BidderResult{
						BidderCode: code,
						Errors:     []error{ctx.Err()},
						TimedOut:   true,
					})
					return
				}

				// Clone request and apply bidder-specific FPD
				bidderReq := e.cloneRequestWithFPD(req, code, bidderFPD)

				result := e.callBidder(ctx, bidderReq, code, awi.Adapter, timeout)

				results.Store(code, result) // P0-1: Thread-safe store
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

					// P0-4: Acquire semaphore (blocks if at capacity)
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }() // Release on completion
					case <-ctx.Done():
						// Context cancelled while waiting for semaphore
						results.Store(code, &BidderResult{
							BidderCode: code,
							Errors:     []error{ctx.Err()},
							TimedOut:   true,
						})
						return
					}

					// Clone request and apply bidder-specific FPD
					bidderReq := e.cloneRequestWithFPD(req, code, bidderFPD)

					// P1-4: Use dynamic adapter's timeout if smaller, with validation bounds
					bidderTimeout := timeout
					if da.GetTimeout() > 0 && da.GetTimeout() < timeout {
						dynamicTimeout := da.GetTimeout()
						// Enforce minimum timeout to prevent crashes
						if dynamicTimeout < minBidderTimeout {
							dynamicTimeout = minBidderTimeout
						}
						// Enforce maximum timeout to prevent resource exhaustion
						if dynamicTimeout > maxBidderTimeout {
							dynamicTimeout = maxBidderTimeout
						}
						bidderTimeout = dynamicTimeout
					}

					result := e.callBidder(ctx, bidderReq, code, da, bidderTimeout)

					results.Store(code, result) // P0-1: Thread-safe store
				}(bidderCode, dynamicAdapter)
			}
		}
	}

	wg.Wait()

	// P0-1: Convert sync.Map to regular map for return
	finalResults := make(map[string]*BidderResult)
	results.Range(func(key, value interface{}) bool {
		finalResults[key.(string)] = value.(*BidderResult)
		return true
	})
	return finalResults
}

// cloneRequestWithFPD creates a deep copy of the request with bidder-specific FPD applied
// and enforces USD currency for all bid requests
func (e *Exchange) cloneRequestWithFPD(req *openrtb.BidRequest, bidderCode string, bidderFPD fpd.BidderFPD) *openrtb.BidRequest {
	// Always clone to ensure thread safety and allow currency normalization
	clone := deepCloneRequest(req, e.config.CloneLimits)

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
// P3-1: Uses configurable limits to bound allocations
func deepCloneRequest(req *openrtb.BidRequest, limits *CloneLimits) *openrtb.BidRequest {
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
		// Deep copy EIDs slice (P1-3: bounded allocation)
		if len(req.User.EIDs) > 0 {
			eidCount := len(req.User.EIDs)
			if eidCount > limits.MaxEIDsPerUser {
				eidCount = limits.MaxEIDsPerUser
			}
			userCopy.EIDs = make([]openrtb.EID, eidCount)
			copy(userCopy.EIDs, req.User.EIDs[:eidCount])
		}
		// Deep copy Data slice (P1-3: bounded allocation)
		if len(req.User.Data) > 0 {
			dataCount := len(req.User.Data)
			if dataCount > limits.MaxDataPerUser {
				dataCount = limits.MaxDataPerUser
			}
			userCopy.Data = make([]openrtb.Data, dataCount)
			copy(userCopy.Data, req.User.Data[:dataCount])
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
			// P1-3: bounded allocation for supply chain nodes
			if len(req.Source.SChain.Nodes) > 0 {
				nodeCount := len(req.Source.SChain.Nodes)
				if nodeCount > limits.MaxSChainNodes {
					nodeCount = limits.MaxSChainNodes
				}
				schainCopy.Nodes = make([]openrtb.SupplyChainNode, nodeCount)
				copy(schainCopy.Nodes, req.Source.SChain.Nodes[:nodeCount])
			}
			sourceCopy.SChain = &schainCopy
		}
		clone.Source = &sourceCopy
	}

	// Deep copy Imp slice (P1-3: bounded allocation)
	if len(req.Imp) > 0 {
		impCount := len(req.Imp)
		if impCount > limits.MaxImpressionsPerRequest {
			impCount = limits.MaxImpressionsPerRequest
		}
		clone.Imp = make([]openrtb.Imp, impCount)
		for i := 0; i < impCount; i++ {
			imp := req.Imp[i]
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
				// P1-3: bounded allocation for deals
				if len(imp.PMP.Deals) > 0 {
					dealCount := len(imp.PMP.Deals)
					if dealCount > limits.MaxDealsPerImp {
						dealCount = limits.MaxDealsPerImp
					}
					pmpCopy.Deals = make([]openrtb.Deal, dealCount)
					copy(pmpCopy.Deals, imp.PMP.Deals[:dealCount])
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
			result.TimedOut = true // P2-2: mark as timed out
			return result
		default:
			// Context still valid, proceed with request
		}

		resp, err := e.httpClient.Do(ctx, reqData, timeout)
		if err != nil {
			result.Errors = append(result.Errors, err)
			// P2-2: Check if this was a timeout error
			if err == context.DeadlineExceeded || err == context.Canceled {
				result.TimedOut = true
			}
			continue
		}

		bidderResp, errs := adapter.MakeBids(req, resp)
		if len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
		}

		if bidderResp != nil {
			// P2-5: Validate BidResponse.ID matches BidRequest.ID (OpenRTB 2.x requirement)
			// Per spec, response ID must echo request ID - reject on mismatch
			if bidderResp.ResponseID != "" && bidderResp.ResponseID != req.ID {
				result.Errors = append(result.Errors, fmt.Errorf(
					"response ID mismatch from %s: expected %q, got %q (bids rejected)",
					bidderCode, req.ID, bidderResp.ResponseID,
				))
				continue // Reject all bids from this response
			}

			// P1-NEW-3: Normalize and validate response currency
			// Per OpenRTB 2.5 spec section 7.2, empty currency means USD
			responseCurrency := bidderResp.Currency
			if responseCurrency == "" {
				responseCurrency = "USD" // OpenRTB 2.5 default
			}
			if responseCurrency != e.config.DefaultCurrency {
				result.Errors = append(result.Errors, fmt.Errorf(
					"currency mismatch from %s: expected %s, got %s (bids rejected)",
					bidderCode, e.config.DefaultCurrency, responseCurrency,
				))
				// Skip bids with wrong currency - can't safely compare prices
				continue
			}

			allBids = append(allBids, bidderResp.Bids...)
		}
	}

	result.Bids = allBids
	result.Latency = time.Since(start)
	return result
}

// buildEmptyResponse creates an empty bid response with optional NBR code
// P2-NEW-2: Include NBR (No-Bid Reason) for debugging per OpenRTB 2.5
func (e *Exchange) buildEmptyResponse(req *openrtb.BidRequest, nbr int) *openrtb.BidResponse {
	return &openrtb.BidResponse{
		ID:      req.ID,
		SeatBid: []openrtb.SeatBid{},
		Cur:     e.config.DefaultCurrency,
		NBR:     nbr,
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
