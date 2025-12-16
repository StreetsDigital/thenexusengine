package endpoints

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

// CookieSyncHandler handles /cookie_sync requests
type CookieSyncHandler struct {
	syncer        *usersync.Syncer
	cookieManager *usersync.CookieManager
	gdprEnforcer  usersync.GDPREnforcer
	config        usersync.CookieSyncConfig
}

// NewCookieSyncHandler creates a new cookie sync handler
func NewCookieSyncHandler(syncer *usersync.Syncer, cookieManager *usersync.CookieManager, config usersync.CookieSyncConfig) *CookieSyncHandler {
	// Create GDPR enforcer with default vendor IDs
	gdprEnforcer := usersync.NewDefaultGDPREnforcer(usersync.DefaultVendorIDs(), config.GDPREnforcement)

	return &CookieSyncHandler{
		syncer:        syncer,
		cookieManager: cookieManager,
		gdprEnforcer:  gdprEnforcer,
		config:        config,
	}
}

// ServeHTTP handles the cookie sync request
func (h *CookieSyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set CORS headers
	h.setCORSHeaders(w, r)

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorResponse(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request (allow empty body for GET-style requests)
	var req usersync.CookieSyncRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			h.writeErrorResponse(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Read existing cookie
	pbsCookie, err := h.cookieManager.Read(r)
	if err != nil {
		pbsCookie = usersync.NewPBSCookie()
	}

	// Check for opt-out
	if pbsCookie.OptOut || h.cookieManager.ParseOptOutCookie(r) {
		h.writeOptOutResponse(w)
		return
	}

	// Build privacy info
	privacy := h.buildPrivacyInfo(&req)

	// Determine cooperative sync setting
	coopSync := h.config.DefaultCooperativeSync
	if req.CooperativeSync != nil {
		coopSync = *req.CooperativeSync
	}

	// Choose bidders to sync
	chooserReq := &usersync.ChooseBiddersRequest{
		RequestedBidders: req.Bidders,
		Limit:            req.Limit,
		Cookie:           pbsCookie,
		Privacy:          privacy,
		FilterSettings:   req.FilterSettings,
		CooperativeSync:  coopSync,
		GDPREnforcer:     h.gdprEnforcer,
	}

	chooseResult := h.syncer.ChooseBidders(chooserReq)

	// Build sync URLs for chosen bidders
	syncResults := h.syncer.BuildSyncs(chooseResult.BiddersToSync, "", privacy)

	// Build response
	response := h.buildResponse(pbsCookie, syncResults, chooseResult, req.Debug)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// buildPrivacyInfo builds privacy info from the request
func (h *CookieSyncHandler) buildPrivacyInfo(req *usersync.CookieSyncRequest) *usersync.PrivacyInfo {
	privacy := &usersync.PrivacyInfo{
		GDPRConsent: req.GDPRConsent,
		USPrivacy:   req.USPrivacy,
		GPP:         req.GPP,
		GPPSID:      req.GPPSID,
	}

	// Parse GDPR applies flag
	if req.GDPR != nil {
		privacy.GDPRApplies = *req.GDPR == 1
	}

	return privacy
}

// buildResponse builds the cookie sync response
func (h *CookieSyncHandler) buildResponse(cookie *usersync.PBSCookie, syncResults []*usersync.SyncResult, chooseResult *usersync.ChooseBiddersResult, debug bool) *usersync.CookieSyncResponse {
	// Determine status
	status := string(usersync.SyncStatusOK)
	if cookie.CountUIDs() == 0 {
		status = string(usersync.SyncStatusNoCookie)
	}

	response := &usersync.CookieSyncResponse{
		Status:       status,
		BidderStatus: make([]usersync.BidderSyncStatus, 0, len(syncResults)),
	}

	// Add bidder sync statuses
	for _, result := range syncResults {
		bidderStatus := usersync.BidderSyncStatus{
			Bidder: result.BidderCode,
		}

		if result.Error != nil {
			bidderStatus.Error = result.Error.Error()
		} else if result.URL != "" {
			bidderStatus.NoCookie = true
			bidderStatus.UserSync = &usersync.UserSyncInfo{
				URL:         result.URL,
				Type:        result.Type,
				SupportCORS: result.SupportCORS,
			}
		}

		response.BidderStatus = append(response.BidderStatus, bidderStatus)
	}

	// Add debug info if requested
	if debug {
		response.Debug = &usersync.DebugInfo{
			RequestedBidders: chooseResult.BiddersToSync,
			RejectedBidders:  chooseResult.RejectedBidders,
		}
	}

	return response
}

// writeErrorResponse writes an error response
func (h *CookieSyncHandler) writeErrorResponse(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&usersync.CookieSyncResponse{
		Status:       string(usersync.SyncStatusError),
		BidderStatus: []usersync.BidderSyncStatus{},
	})
}

// writeOptOutResponse writes a response for opted-out users
func (h *CookieSyncHandler) writeOptOutResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&usersync.CookieSyncResponse{
		Status:       string(usersync.SyncStatusOK),
		BidderStatus: []usersync.BidderSyncStatus{},
	})
}

// setCORSHeaders sets CORS headers for the response
func (h *CookieSyncHandler) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	}
}
