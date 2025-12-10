// Package endpoints provides HTTP endpoint handlers
package endpoints

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/exchange"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// AuctionHandler handles /openrtb2/auction requests
type AuctionHandler struct {
	exchange *exchange.Exchange
}

// NewAuctionHandler creates a new auction handler
func NewAuctionHandler(ex *exchange.Exchange) *AuctionHandler {
	return &AuctionHandler{exchange: ex}
}

// ServeHTTP handles the auction request
func (h *AuctionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse OpenRTB request
	var bidRequest openrtb.BidRequest
	if err := json.Unmarshal(body, &bidRequest); err != nil {
		writeError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateBidRequest(&bidRequest); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build auction request
	auctionReq := &exchange.AuctionRequest{
		BidRequest: &bidRequest,
		Debug:      r.URL.Query().Get("debug") == "1",
	}

	// Run auction
	ctx := r.Context()
	result, err := h.exchange.RunAuction(ctx, auctionReq)
	if err != nil {
		writeError(w, "Auction failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response with extensions
	response := result.BidResponse
	if auctionReq.Debug && result.DebugInfo != nil {
		// Add debug info to extension
		ext := buildResponseExt(result)
		if extBytes, err := json.Marshal(ext); err == nil {
			response.Ext = extBytes
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateBidRequest validates the bid request
func validateBidRequest(req *openrtb.BidRequest) error {
	if req.ID == "" {
		return &ValidationError{Field: "id", Message: "required"}
	}
	if len(req.Imp) == 0 {
		return &ValidationError{Field: "imp", Message: "at least one impression required"}
	}
	for i, imp := range req.Imp {
		if imp.ID == "" {
			return &ValidationError{Field: "imp[].id", Message: "required", Index: i}
		}
		if imp.Banner == nil && imp.Video == nil && imp.Native == nil && imp.Audio == nil {
			return &ValidationError{Field: "imp[].banner|video|native|audio", Message: "at least one media type required", Index: i}
		}
	}
	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Index   int
}

func (e *ValidationError) Error() string {
	if e.Index >= 0 {
		return e.Field + ": " + e.Message
	}
	return e.Field + ": " + e.Message
}

// buildResponseExt builds response extensions with debug info
func buildResponseExt(result *exchange.AuctionResponse) *openrtb.BidResponseExt {
	ext := &openrtb.BidResponseExt{
		ResponseTimeMillis: make(map[string]int),
		Errors:             make(map[string][]openrtb.ExtBidderMessage),
	}

	if result.DebugInfo != nil {
		for bidder, latency := range result.DebugInfo.BidderLatencies {
			ext.ResponseTimeMillis[bidder] = int(latency.Milliseconds())
		}

		for bidder, errs := range result.DebugInfo.Errors {
			messages := make([]openrtb.ExtBidderMessage, len(errs))
			for i, e := range errs {
				messages[i] = openrtb.ExtBidderMessage{Code: 1, Message: e}
			}
			ext.Errors[bidder] = messages
		}

		ext.TMMaxRequest = int(result.DebugInfo.TotalLatency.Milliseconds())
	}

	return ext
}

// writeError writes an error response
func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// StatusHandler handles /status requests
type StatusHandler struct{}

// NewStatusHandler creates a new status handler
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{}
}

// ServeHTTP handles status requests
func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// InfoBiddersHandler handles /info/bidders requests
type InfoBiddersHandler struct {
	bidders []string
}

// NewInfoBiddersHandler creates a new bidders info handler
func NewInfoBiddersHandler(bidders []string) *InfoBiddersHandler {
	return &InfoBiddersHandler{bidders: bidders}
}

// ServeHTTP handles info/bidders requests
func (h *InfoBiddersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.bidders)
}
