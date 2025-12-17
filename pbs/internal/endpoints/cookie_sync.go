package endpoints

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

// CookieSyncRequest represents a cookie sync request
type CookieSyncRequest struct {
	Bidders       []string `json:"bidders,omitempty"`
	GDPR          *int     `json:"gdpr,omitempty"`
	GDPRConsent   string   `json:"gdpr_consent,omitempty"`
	USPrivacy     string   `json:"us_privacy,omitempty"`
	GPP           string   `json:"gpp,omitempty"`
	GPPSID        string   `json:"gpp_sid,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	CoopSync      *bool    `json:"coopSync,omitempty"`
	FilterSettings *FilterSettings `json:"filterSettings,omitempty"`
	Account       string   `json:"account,omitempty"`
}

// FilterSettings controls which bidders are included in cookie sync
type FilterSettings struct {
	IFrame   *BidderFilter `json:"iframe,omitempty"`
	Redirect *BidderFilter `json:"image,omitempty"`
}

// BidderFilter specifies bidder filtering rules
type BidderFilter struct {
	Bidders []string `json:"bidders,omitempty"`
	Mode    string   `json:"filter,omitempty"` // "include" or "exclude"
}

// CookieSyncResponse represents a cookie sync response
type CookieSyncResponse struct {
	Status       string            `json:"status"`
	BidderStatus []BidderSyncInfo  `json:"bidder_status,omitempty"`
}

// BidderSyncInfo contains sync information for a single bidder
type BidderSyncInfo struct {
	Bidder    string `json:"bidder"`
	NoCookie  bool   `json:"no_cookie,omitempty"`
	UserSync  *UserSyncInfo `json:"usersync,omitempty"`
	Error     string `json:"error,omitempty"`
}

// UserSyncInfo contains the sync URL and type
type UserSyncInfo struct {
	URL         string `json:"url"`
	Type        string `json:"type"`
	SupportCORS bool   `json:"supportCORS,omitempty"`
}

// CookieSyncConfig holds configuration for the cookie sync endpoint
type CookieSyncConfig struct {
	ExternalURL     string
	DefaultLimit    int
	MaxLimit        int
	DefaultCoopSync bool
	CookieDomain    string
}

// DefaultCookieSyncConfig returns default configuration
func DefaultCookieSyncConfig() *CookieSyncConfig {
	return &CookieSyncConfig{
		ExternalURL:     "https://pbs.nexusengine.io",
		DefaultLimit:    0, // 0 means no limit
		MaxLimit:        100,
		DefaultCoopSync: true,
		CookieDomain:    "",
	}
}

// CookieSyncHandler handles /cookie_sync requests
type CookieSyncHandler struct {
	config   *CookieSyncConfig
	syncers  *usersync.SyncerRegistry
	bidders  []string
}

// NewCookieSyncHandler creates a new cookie sync handler
func NewCookieSyncHandler(config *CookieSyncConfig, syncers *usersync.SyncerRegistry, bidders []string) *CookieSyncHandler {
	return &CookieSyncHandler{
		config:  config,
		syncers: syncers,
		bidders: bidders,
	}
}

// ServeHTTP handles the cookie sync request
func (h *CookieSyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the existing cookie
	cookie := usersync.ParseCookie(r)

	// Check if user has opted out
	if cookie.OptOut {
		h.writeResponse(w, &CookieSyncResponse{
			Status: "ok",
		})
		return
	}

	// Parse request body
	var req CookieSyncRequest
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	// Build privacy info
	privacy := usersync.Privacy{
		GDPRConsent: req.GDPRConsent,
		USPrivacy:   req.USPrivacy,
		GPP:         req.GPP,
		GPPSID:      req.GPPSID,
	}
	if req.GDPR != nil && *req.GDPR == 1 {
		privacy.GDPR = true
	}

	// Determine which bidders to sync
	biddersToSync := h.getBiddersToSync(req, cookie)

	// Apply limit
	limit := h.getLimit(req.Limit)
	if limit > 0 && len(biddersToSync) > limit {
		biddersToSync = biddersToSync[:limit]
	}

	// Get preferred sync types based on filter settings
	syncTypes := h.getSyncTypes(req.FilterSettings)

	// Build sync info for each bidder
	bidderStatus := make([]BidderSyncInfo, 0, len(biddersToSync))
	for _, bidder := range biddersToSync {
		status := h.buildBidderStatus(bidder, cookie, syncTypes, privacy)
		bidderStatus = append(bidderStatus, status)
	}

	// Build response
	response := &CookieSyncResponse{
		Status:       "ok",
		BidderStatus: bidderStatus,
	}

	h.writeResponse(w, response)
}

// getBiddersToSync returns the list of bidders that need syncing
func (h *CookieSyncHandler) getBiddersToSync(req CookieSyncRequest, cookie *usersync.Cookie) []string {
	var candidateBidders []string

	// If specific bidders requested, use those
	if len(req.Bidders) > 0 {
		candidateBidders = req.Bidders
	} else {
		// Use coop sync if enabled
		coopSync := h.config.DefaultCoopSync
		if req.CoopSync != nil {
			coopSync = *req.CoopSync
		}
		if coopSync {
			candidateBidders = h.bidders
		}
	}

	// Filter out bidders that already have synced cookies
	needsSync := make([]string, 0, len(candidateBidders))
	for _, bidder := range candidateBidders {
		if !cookie.HasLiveSync(bidder) {
			needsSync = append(needsSync, bidder)
		}
	}

	// Sort for consistent ordering
	sort.Strings(needsSync)

	return needsSync
}

// getLimit returns the effective limit
func (h *CookieSyncHandler) getLimit(requestLimit int) int {
	if requestLimit <= 0 {
		return h.config.DefaultLimit
	}
	if h.config.MaxLimit > 0 && requestLimit > h.config.MaxLimit {
		return h.config.MaxLimit
	}
	return requestLimit
}

// getSyncTypes returns the preferred sync types based on filter settings
func (h *CookieSyncHandler) getSyncTypes(settings *FilterSettings) []usersync.SyncType {
	// Default: prefer redirect over iframe
	types := []usersync.SyncType{usersync.SyncTypePixel, usersync.SyncTypeIFrame}

	if settings == nil {
		return types
	}

	// If only one type is configured, use that
	if settings.IFrame != nil && settings.Redirect == nil {
		return []usersync.SyncType{usersync.SyncTypeIFrame}
	}
	if settings.Redirect != nil && settings.IFrame == nil {
		return []usersync.SyncType{usersync.SyncTypePixel}
	}

	return types
}

// buildBidderStatus builds the sync status for a single bidder
func (h *CookieSyncHandler) buildBidderStatus(bidder string, cookie *usersync.Cookie, syncTypes []usersync.SyncType, privacy usersync.Privacy) BidderSyncInfo {
	status := BidderSyncInfo{
		Bidder: bidder,
	}

	// Check if bidder already has a sync
	if cookie.HasLiveSync(bidder) {
		status.NoCookie = false
		return status
	}

	status.NoCookie = true

	// Get syncer for this bidder
	syncer, ok := h.syncers.Get(bidder)
	if !ok {
		status.Error = "no sync configuration"
		return status
	}

	// Get sync URL
	sync, err := syncer.GetSync(syncTypes, privacy)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	status.UserSync = &UserSyncInfo{
		URL:         sync.URL,
		Type:        string(sync.Type),
		SupportCORS: sync.SupportCORS,
	}

	return status
}

// writeResponse writes the JSON response
func (h *CookieSyncHandler) writeResponse(w http.ResponseWriter, response *CookieSyncResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
