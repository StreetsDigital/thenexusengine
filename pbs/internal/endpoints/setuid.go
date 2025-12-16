package endpoints

import (
	"encoding/json"
	"net/http"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

// SetUIDHandler handles /setuid requests
type SetUIDHandler struct {
	syncer        *usersync.Syncer
	cookieManager *usersync.CookieManager
	gdprEnforcer  usersync.GDPREnforcer
	config        usersync.CookieSyncConfig
}

// NewSetUIDHandler creates a new setuid handler
func NewSetUIDHandler(syncer *usersync.Syncer, cookieManager *usersync.CookieManager, config usersync.CookieSyncConfig) *SetUIDHandler {
	gdprEnforcer := usersync.NewDefaultGDPREnforcer(usersync.DefaultVendorIDs(), config.GDPREnforcement)

	return &SetUIDHandler{
		syncer:        syncer,
		cookieManager: cookieManager,
		gdprEnforcer:  gdprEnforcer,
		config:        config,
	}
}

// ServeHTTP handles the setuid request
func (h *SetUIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Allow GET and POST
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set CORS headers
	h.setCORSHeaders(w, r)

	// Parse query parameters
	query := r.URL.Query()
	bidder := query.Get("bidder")
	uid := query.Get("uid")
	gdprParam := query.Get("gdpr")
	gdprConsent := query.Get("gdpr_consent")
	format := query.Get("f") // Response format: "b" for blank, "i" for image

	// Validate bidder parameter
	if bidder == "" {
		h.writeError(w, "bidder parameter required", http.StatusBadRequest, format)
		return
	}

	// Get syncer key for the bidder
	syncerKey := h.syncer.GetSyncerKey(bidder)
	if syncerKey == "" {
		syncerKey = bidder // Use bidder code if no mapping
	}

	// Check GDPR consent if applicable
	if gdprParam == "1" && h.config.GDPREnforcement {
		if !h.gdprEnforcer.CanSync(bidder, gdprConsent) {
			h.writeError(w, "GDPR consent denied", http.StatusUnavailableForLegalReasons, format)
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
		h.writeSuccess(w, format)
		return
	}

	// Update the UID
	err = h.cookieManager.SetUID(w, r, syncerKey, uid)
	if err != nil {
		h.writeError(w, "Failed to set cookie", http.StatusInternalServerError, format)
		return
	}

	// Write success response
	h.writeSuccess(w, format)
}

// writeSuccess writes a success response in the requested format
func (h *SetUIDHandler) writeSuccess(w http.ResponseWriter, format string) {
	switch format {
	case "i":
		// Return 1x1 transparent pixel
		h.writePixel(w)
	case "b":
		// Return blank response
		w.WriteHeader(http.StatusOK)
	default:
		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// writeError writes an error response in the requested format
func (h *SetUIDHandler) writeError(w http.ResponseWriter, message string, status int, format string) {
	switch format {
	case "i":
		// Still return pixel for image format, but with error status
		w.WriteHeader(status)
		h.writePixel(w)
	case "b":
		// Return blank with status
		w.WriteHeader(status)
	default:
		// Return JSON error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  message,
		})
	}
}

// writePixel writes a 1x1 transparent GIF
func (h *SetUIDHandler) writePixel(w http.ResponseWriter) {
	// 1x1 transparent GIF
	pixel := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
		0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
		0x01, 0x00, 0x3b,
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Content-Length", "43")
	w.Write(pixel)
}

// setCORSHeaders sets CORS headers for the response
func (h *SetUIDHandler) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	}
}

// OptOutHandler handles /optout requests for user opt-out
type OptOutHandler struct {
	cookieManager *usersync.CookieManager
}

// NewOptOutHandler creates a new opt-out handler
func NewOptOutHandler(cookieManager *usersync.CookieManager) *OptOutHandler {
	return &OptOutHandler{cookieManager: cookieManager}
}

// ServeHTTP handles the opt-out request
func (h *OptOutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.cookieManager.OptOut(w, r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Failed to set opt-out cookie",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
