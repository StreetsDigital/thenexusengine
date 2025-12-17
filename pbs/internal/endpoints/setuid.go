package endpoints

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

// SetUIDConfig holds configuration for the setuid endpoint
type SetUIDConfig struct {
	CookieDomain string
	CookieTTL    time.Duration
}

// DefaultSetUIDConfig returns default configuration
func DefaultSetUIDConfig() *SetUIDConfig {
	return &SetUIDConfig{
		CookieDomain: "",
		CookieTTL:    usersync.DefaultCookieTTL,
	}
}

// SetUIDHandler handles /setuid requests
type SetUIDHandler struct {
	config  *SetUIDConfig
	syncers *usersync.SyncerRegistry
}

// NewSetUIDHandler creates a new setuid handler
func NewSetUIDHandler(config *SetUIDConfig, syncers *usersync.SyncerRegistry) *SetUIDHandler {
	return &SetUIDHandler{
		config:  config,
		syncers: syncers,
	}
}

// ServeHTTP handles the setuid request
func (h *SetUIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	query := r.URL.Query()
	bidder := query.Get("bidder")
	uid := query.Get("uid")

	// Validate required parameters
	if bidder == "" {
		h.writeBadRequest(w, "bidder is required")
		return
	}

	// Parse the existing cookie
	cookie := usersync.ParseCookie(r)

	// Check if user has opted out
	if cookie.OptOut {
		h.writePixelResponse(w)
		return
	}

	// Validate bidder exists
	if _, ok := h.syncers.Get(bidder); !ok {
		h.writeBadRequest(w, "unknown bidder: "+bidder)
		return
	}

	// Handle GDPR consent if provided
	gdpr := query.Get("gdpr")
	gdprConsent := query.Get("gdpr_consent")

	// If GDPR applies and no consent, we shouldn't store the UID
	if gdpr == "1" && gdprConsent == "" {
		h.writePixelResponse(w)
		return
	}

	// Update the cookie
	if uid == "" {
		// Empty UID means delete the sync
		cookie.DeleteUID(bidder)
	} else {
		cookie.SetUID(bidder, uid)
	}

	// Write the updated cookie
	if err := cookie.WriteCookie(w, h.config.CookieDomain, h.config.CookieTTL); err != nil {
		h.writeError(w, "failed to write cookie", http.StatusInternalServerError)
		return
	}

	// Check format parameter for response type
	format := query.Get("f")
	switch format {
	case "b": // blank
		h.writeBlankResponse(w)
	case "i": // image/pixel
		h.writePixelResponse(w)
	case "j": // json
		h.writeJSONResponse(w, bidder, uid != "")
	default:
		// Default to pixel response
		h.writePixelResponse(w)
	}
}

// writeBadRequest writes a bad request error
func (h *SetUIDHandler) writeBadRequest(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// writeError writes an error response
func (h *SetUIDHandler) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// writePixelResponse writes a 1x1 transparent GIF
func (h *SetUIDHandler) writePixelResponse(w http.ResponseWriter) {
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
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	w.Write(pixel)
}

// writeBlankResponse writes an empty 200 response
func (h *SetUIDHandler) writeBlankResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
}

// writeJSONResponse writes a JSON response
func (h *SetUIDHandler) writeJSONResponse(w http.ResponseWriter, bidder string, success bool) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"bidder":  bidder,
		"success": success,
	}
	json.NewEncoder(w).Encode(response)
}
