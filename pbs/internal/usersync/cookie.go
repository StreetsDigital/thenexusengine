package usersync

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

// CookieReader handles reading the UID cookie from HTTP requests
type CookieReader struct {
	config CookieSyncConfig
}

// NewCookieReader creates a new CookieReader
func NewCookieReader(config CookieSyncConfig) *CookieReader {
	return &CookieReader{config: config}
}

// Read extracts and decodes the PBSCookie from an HTTP request
func (cr *CookieReader) Read(r *http.Request) (*PBSCookie, error) {
	cookie, err := r.Cookie(cr.config.CookieName)
	if err != nil {
		// No cookie found, return a new empty cookie
		return NewPBSCookie(), nil
	}

	// Decode from base64
	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		// Invalid encoding, return new cookie
		return NewPBSCookie(), nil
	}

	// Unmarshal JSON
	pbsCookie, err := UnmarshalPBSCookie(decoded)
	if err != nil {
		// Invalid JSON, return new cookie
		return NewPBSCookie(), nil
	}

	return pbsCookie, nil
}

// CookieWriter handles writing the UID cookie to HTTP responses
type CookieWriter struct {
	config CookieSyncConfig
}

// NewCookieWriter creates a new CookieWriter
func NewCookieWriter(config CookieSyncConfig) *CookieWriter {
	return &CookieWriter{config: config}
}

// Write encodes and sets the PBSCookie on an HTTP response
func (cw *CookieWriter) Write(w http.ResponseWriter, pbsCookie *PBSCookie) error {
	// Marshal to JSON
	data, err := pbsCookie.Marshal()
	if err != nil {
		return err
	}

	// Encode to base64
	encoded := base64.URLEncoding.EncodeToString(data)

	// Create HTTP cookie
	cookie := &http.Cookie{
		Name:     cw.config.CookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   int(cw.config.CookieMaxAge.Seconds()),
		Expires:  time.Now().UTC().Add(cw.config.CookieMaxAge),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}

	if cw.config.CookieDomain != "" {
		cookie.Domain = cw.config.CookieDomain
	}

	http.SetCookie(w, cookie)
	return nil
}

// CookieManager combines reading and writing functionality
type CookieManager struct {
	reader *CookieReader
	writer *CookieWriter
	config CookieSyncConfig
}

// NewCookieManager creates a new CookieManager
func NewCookieManager(config CookieSyncConfig) *CookieManager {
	return &CookieManager{
		reader: NewCookieReader(config),
		writer: NewCookieWriter(config),
		config: config,
	}
}

// Read extracts and decodes the PBSCookie from an HTTP request
func (cm *CookieManager) Read(r *http.Request) (*PBSCookie, error) {
	return cm.reader.Read(r)
}

// Write encodes and sets the PBSCookie on an HTTP response
func (cm *CookieManager) Write(w http.ResponseWriter, cookie *PBSCookie) error {
	return cm.writer.Write(w, cookie)
}

// SetUID updates a single UID in the cookie and writes it to the response
func (cm *CookieManager) SetUID(w http.ResponseWriter, r *http.Request, syncerKey string, uid string) error {
	// Read existing cookie
	pbsCookie, err := cm.Read(r)
	if err != nil {
		pbsCookie = NewPBSCookie()
	}

	// Check for opt-out
	if pbsCookie.OptOut {
		return nil // Don't update if opted out
	}

	// Calculate expiry
	expires := time.Now().UTC().Add(cm.config.CookieMaxAge)

	// Update UID
	if uid == "" || uid == "0" {
		// Empty or "0" UID means delete
		pbsCookie.DeleteUID(syncerKey)
	} else {
		pbsCookie.SetUID(syncerKey, uid, &expires)
	}

	// Write updated cookie
	return cm.Write(w, pbsCookie)
}

// OptOut marks the cookie as opted-out and clears all UIDs
func (cm *CookieManager) OptOut(w http.ResponseWriter, r *http.Request) error {
	pbsCookie := &PBSCookie{
		UIDs:   make(map[string]UIDEntry),
		OptOut: true,
	}
	return cm.Write(w, pbsCookie)
}

// OptIn removes the opt-out flag
func (cm *CookieManager) OptIn(w http.ResponseWriter, r *http.Request) error {
	pbsCookie, _ := cm.Read(r)
	pbsCookie.OptOut = false
	return cm.Write(w, pbsCookie)
}

// IsOptedOut checks if the user has opted out
func (cm *CookieManager) IsOptedOut(r *http.Request) bool {
	pbsCookie, _ := cm.Read(r)
	return pbsCookie.OptOut
}

// ParseOptOutCookie reads the legacy opt-out cookie if present
func (cm *CookieManager) ParseOptOutCookie(r *http.Request) bool {
	// Check for legacy optout cookie
	optoutCookie, err := r.Cookie("uids_optout")
	if err == nil && optoutCookie.Value == "true" {
		return true
	}
	return false
}

// ToJSON returns the cookie as a JSON string for debugging
func (c *PBSCookie) ToJSON() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

// CountUIDs returns the number of valid (non-expired) UIDs
func (c *PBSCookie) CountUIDs() int {
	if c == nil || c.UIDs == nil {
		return 0
	}
	count := 0
	now := time.Now().UTC()
	for _, entry := range c.UIDs {
		if entry.UID != "" && (entry.Expires == nil || entry.Expires.After(now)) {
			count++
		}
	}
	return count
}

// GetAllUIDs returns all valid UIDs as a map
func (c *PBSCookie) GetAllUIDs() map[string]string {
	result := make(map[string]string)
	if c == nil || c.UIDs == nil {
		return result
	}
	now := time.Now().UTC()
	for key, entry := range c.UIDs {
		if entry.UID != "" && (entry.Expires == nil || entry.Expires.After(now)) {
			result[key] = entry.UID
		}
	}
	return result
}
