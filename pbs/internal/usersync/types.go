// Package usersync provides user synchronization functionality for Prebid Server
package usersync

import (
	"encoding/json"
	"time"
)

// SyncType represents the type of user sync
type SyncType string

const (
	// SyncTypeIFrame indicates an iframe-based sync
	SyncTypeIFrame SyncType = "iframe"
	// SyncTypeRedirect indicates a redirect/pixel-based sync
	SyncTypeRedirect SyncType = "redirect"
)

// SyncURLTemplate contains a URL template with macro support
type SyncURLTemplate struct {
	// URL is the sync URL with macros like {{gdpr}}, {{gdpr_consent}}, {{us_privacy}}, {{redirect_url}}
	URL string `json:"url"`
	// UserMacro is the macro the bidder uses for user ID injection (e.g., $UID, ${UID}, [UID])
	UserMacro string `json:"user_macro,omitempty"`
}

// SyncerConfig contains the full sync configuration for a bidder
type SyncerConfig struct {
	// Key is the unique syncer key (may differ from bidder code, e.g., "adnxs" for appnexus)
	Key string `json:"key"`
	// Supports lists supported sync types
	Supports []SyncType `json:"supports"`
	// Default is the preferred sync type
	Default SyncType `json:"default"`
	// IFrame contains the iframe sync URL template
	IFrame *SyncURLTemplate `json:"iframe,omitempty"`
	// Redirect contains the redirect/pixel sync URL template
	Redirect *SyncURLTemplate `json:"redirect,omitempty"`
	// SupportCORS indicates if the syncer endpoint supports CORS
	SupportCORS bool `json:"support_cors"`
}

// CookieSyncRequest represents the POST body for /cookie_sync
type CookieSyncRequest struct {
	// Bidders is the list of bidders to sync (empty = use account config or all enabled)
	Bidders []string `json:"bidders,omitempty"`
	// Limit is the max number of syncs to return
	Limit int `json:"limit,omitempty"`
	// GDPR indicates if GDPR applies (1 = yes, 0 = no)
	GDPR *int `json:"gdpr,omitempty"`
	// GDPRConsent is the TCF consent string
	GDPRConsent string `json:"gdpr_consent,omitempty"`
	// USPrivacy is the CCPA/US Privacy string
	USPrivacy string `json:"us_privacy,omitempty"`
	// GPP is the Global Privacy Platform consent string
	GPP string `json:"gpp,omitempty"`
	// GPPSID contains GPP section IDs that apply
	GPPSID []int `json:"gpp_sid,omitempty"`
	// CooperativeSync enables cooperative syncing with other bidders
	CooperativeSync *bool `json:"coopSync,omitempty"`
	// FilterSettings controls which sync types/bidders to include or exclude
	FilterSettings *FilterSettings `json:"filterSettings,omitempty"`
	// Account is the publisher account ID
	Account string `json:"account,omitempty"`
	// Debug enables debug info in response
	Debug bool `json:"debug,omitempty"`
}

// FilterSettings controls which syncs to include/exclude
type FilterSettings struct {
	// IFrame controls iframe sync filtering
	IFrame *SyncFilter `json:"iframe,omitempty"`
	// Redirect controls redirect/image sync filtering (Prebid.js calls this "image")
	Redirect *SyncFilter `json:"image,omitempty"`
}

// SyncFilter specifies bidder filter for a sync type
type SyncFilter struct {
	// Bidders is the list of bidders to filter
	Bidders []string `json:"bidders,omitempty"`
	// Mode is "include" or "exclude"
	Mode FilterMode `json:"filter,omitempty"`
}

// FilterMode represents the filter mode
type FilterMode string

const (
	// FilterModeInclude only includes specified bidders
	FilterModeInclude FilterMode = "include"
	// FilterModeExclude excludes specified bidders
	FilterModeExclude FilterMode = "exclude"
)

// CookieSyncResponse is the response from /cookie_sync
type CookieSyncResponse struct {
	// Status is "ok", "no_cookie", or "error"
	Status string `json:"status"`
	// BidderStatus contains sync info for each bidder
	BidderStatus []BidderSyncStatus `json:"bidder_status"`
	// Debug contains debug information if requested
	Debug *DebugInfo `json:"debug,omitempty"`
}

// SyncStatus represents the status of a cookie sync response
type SyncStatus string

const (
	// SyncStatusOK indicates successful sync info returned
	SyncStatusOK SyncStatus = "ok"
	// SyncStatusNoCookie indicates no existing cookie was found
	SyncStatusNoCookie SyncStatus = "no_cookie"
	// SyncStatusError indicates an error occurred
	SyncStatusError SyncStatus = "error"
)

// BidderSyncStatus contains sync info for a single bidder
type BidderSyncStatus struct {
	// Bidder is the bidder code
	Bidder string `json:"bidder"`
	// NoCookie indicates the bidder needs a sync (no existing UID)
	NoCookie bool `json:"no_cookie,omitempty"`
	// UserSync contains the sync URL details if a sync is needed
	UserSync *UserSyncInfo `json:"usersync,omitempty"`
	// Error contains any error message for this bidder
	Error string `json:"error,omitempty"`
}

// UserSyncInfo contains the sync URL details
type UserSyncInfo struct {
	// URL is the sync URL to load
	URL string `json:"url"`
	// Type is "iframe" or "redirect"
	Type SyncType `json:"type"`
	// SupportCORS indicates if the endpoint supports CORS
	SupportCORS bool `json:"supportCORS,omitempty"`
}

// DebugInfo contains debug information for the response
type DebugInfo struct {
	// RequestedBidders lists all bidders that were requested
	RequestedBidders []string `json:"requested_bidders,omitempty"`
	// RejectedBidders maps rejected bidders to rejection reasons
	RejectedBidders map[string]string `json:"rejected_bidders,omitempty"`
}

// SetUIDRequest represents query parameters for /setuid
type SetUIDRequest struct {
	// Bidder is the bidder code
	Bidder string `json:"bidder"`
	// UID is the user ID from the bidder
	UID string `json:"uid"`
	// GDPR indicates if GDPR applies
	GDPR *int `json:"gdpr,omitempty"`
	// GDPRConsent is the TCF consent string
	GDPRConsent string `json:"gdpr_consent,omitempty"`
	// Format is the response format ("b" for blank, "i" for image)
	Format string `json:"f,omitempty"`
}

// PrivacyInfo contains privacy/consent information for sync URL building
type PrivacyInfo struct {
	// GDPRApplies indicates if GDPR applies
	GDPRApplies bool
	// GDPRConsent is the TCF consent string
	GDPRConsent string
	// USPrivacy is the CCPA/US Privacy string
	USPrivacy string
	// GPP is the Global Privacy Platform string
	GPP string
	// GPPSID contains GPP section IDs
	GPPSID []int
}

// GDPRSignal returns the GDPR signal as a string ("0" or "1")
func (p *PrivacyInfo) GDPRSignal() string {
	if p.GDPRApplies {
		return "1"
	}
	return "0"
}

// GPPSIDString returns the GPP SID as a comma-separated string
func (p *PrivacyInfo) GPPSIDString() string {
	if len(p.GPPSID) == 0 {
		return ""
	}
	result := ""
	for i, sid := range p.GPPSID {
		if i > 0 {
			result += ","
		}
		result += string(rune('0' + sid%10))
		if sid >= 10 {
			result = string(rune('0'+sid/10)) + result
		}
	}
	return result
}

// PBSCookie represents the UID cookie storing all bidder user IDs
type PBSCookie struct {
	// UIDs maps bidder syncer keys to their user ID entries
	UIDs map[string]UIDEntry `json:"uids,omitempty"`
	// OptOut indicates if the user has opted out of tracking
	OptOut bool `json:"optout,omitempty"`
	// Birthday is when this cookie was first created
	Birthday *time.Time `json:"bday,omitempty"`
}

// UIDEntry represents a single bidder's user ID
type UIDEntry struct {
	// UID is the user ID value
	UID string `json:"uid"`
	// Expires is when this UID expires
	Expires *time.Time `json:"expires,omitempty"`
}

// NewPBSCookie creates a new empty PBSCookie
func NewPBSCookie() *PBSCookie {
	now := time.Now().UTC()
	return &PBSCookie{
		UIDs:     make(map[string]UIDEntry),
		OptOut:   false,
		Birthday: &now,
	}
}

// HasUID checks if a UID exists for the given syncer key
func (c *PBSCookie) HasUID(syncerKey string) bool {
	if c == nil || c.UIDs == nil {
		return false
	}
	entry, exists := c.UIDs[syncerKey]
	if !exists {
		return false
	}
	// Check if expired
	if entry.Expires != nil && entry.Expires.Before(time.Now().UTC()) {
		return false
	}
	return entry.UID != ""
}

// GetUID returns the UID for the given syncer key
func (c *PBSCookie) GetUID(syncerKey string) (string, bool) {
	if c == nil || c.UIDs == nil {
		return "", false
	}
	entry, exists := c.UIDs[syncerKey]
	if !exists {
		return "", false
	}
	// Check if expired
	if entry.Expires != nil && entry.Expires.Before(time.Now().UTC()) {
		return "", false
	}
	return entry.UID, entry.UID != ""
}

// SetUID sets or updates the UID for the given syncer key
func (c *PBSCookie) SetUID(syncerKey string, uid string, expires *time.Time) {
	if c.UIDs == nil {
		c.UIDs = make(map[string]UIDEntry)
	}
	c.UIDs[syncerKey] = UIDEntry{
		UID:     uid,
		Expires: expires,
	}
}

// DeleteUID removes the UID for the given syncer key
func (c *PBSCookie) DeleteUID(syncerKey string) {
	if c.UIDs != nil {
		delete(c.UIDs, syncerKey)
	}
}

// SyncersNeedingSync returns syncer keys that don't have a valid UID
func (c *PBSCookie) SyncersNeedingSync(syncerKeys []string) []string {
	result := make([]string, 0, len(syncerKeys))
	for _, key := range syncerKeys {
		if !c.HasUID(key) {
			result = append(result, key)
		}
	}
	return result
}

// Marshal encodes the cookie to JSON
func (c *PBSCookie) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalPBSCookie decodes a PBSCookie from JSON
func UnmarshalPBSCookie(data []byte) (*PBSCookie, error) {
	if len(data) == 0 {
		return NewPBSCookie(), nil
	}
	cookie := &PBSCookie{}
	if err := json.Unmarshal(data, cookie); err != nil {
		return NewPBSCookie(), err
	}
	if cookie.UIDs == nil {
		cookie.UIDs = make(map[string]UIDEntry)
	}
	return cookie, nil
}

// CookieSyncConfig holds configuration for the cookie sync endpoint
type CookieSyncConfig struct {
	// DefaultLimit is the default max syncs to return (default: 8)
	DefaultLimit int `json:"default_limit"`
	// MaxLimit is the maximum allowed limit (default: 32)
	MaxLimit int `json:"max_limit"`
	// CookieName is the name of the UID cookie (default: "uids")
	CookieName string `json:"cookie_name"`
	// CookieDomain is the domain for the cookie
	CookieDomain string `json:"cookie_domain"`
	// CookieMaxAge is the cookie expiry duration (default: 90 days)
	CookieMaxAge time.Duration `json:"cookie_max_age"`
	// ExternalURL is the external URL for redirect_url macro
	ExternalURL string `json:"external_url"`
	// DefaultCooperativeSync enables cooperative sync by default
	DefaultCooperativeSync bool `json:"default_coop_sync"`
	// GDPREnforcement enables GDPR enforcement for syncing
	GDPREnforcement bool `json:"gdpr_enforcement"`
}

// DefaultCookieSyncConfig returns the default configuration
func DefaultCookieSyncConfig() CookieSyncConfig {
	return CookieSyncConfig{
		DefaultLimit:           8,
		MaxLimit:               32,
		CookieName:             "uids",
		CookieDomain:           "",
		CookieMaxAge:           90 * 24 * time.Hour, // 90 days
		ExternalURL:            "",
		DefaultCooperativeSync: true,
		GDPREnforcement:        true,
	}
}
