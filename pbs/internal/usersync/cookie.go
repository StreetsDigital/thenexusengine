// Package usersync provides user synchronization functionality for Prebid Server
package usersync

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const (
	// UIDsCookieName is the name of the cookie that stores user IDs
	UIDsCookieName = "uids"

	// DefaultCookieTTL is the default time-to-live for the cookie
	DefaultCookieTTL = 365 * 24 * time.Hour

	// SyncTypeRedirect indicates redirect-based sync
	SyncTypeRedirect = "redirect"

	// SyncTypeIframe indicates iframe-based sync
	SyncTypeIframe = "iframe"
)

// Cookie represents the PBS user sync cookie
type Cookie struct {
	UIDs     map[string]UIDEntry `json:"uids,omitempty"`
	OptOut   bool                `json:"optout,omitempty"`
	Birthday *time.Time          `json:"bday,omitempty"`
}

// UIDEntry represents a single user ID entry for a bidder
type UIDEntry struct {
	UID     string    `json:"uid"`
	Expires time.Time `json:"expires"`
}

// NewCookie creates a new empty cookie
func NewCookie() *Cookie {
	now := time.Now().UTC()
	return &Cookie{
		UIDs:     make(map[string]UIDEntry),
		Birthday: &now,
	}
}

// ParseCookie parses a cookie from an HTTP request
func ParseCookie(r *http.Request) *Cookie {
	cookie, err := r.Cookie(UIDsCookieName)
	if err != nil || cookie == nil {
		return NewCookie()
	}

	return DecodeCookie(cookie.Value)
}

// DecodeCookie decodes a base64-encoded cookie value
func DecodeCookie(value string) *Cookie {
	decoded, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return NewCookie()
	}

	var c Cookie
	if err := json.Unmarshal(decoded, &c); err != nil {
		return NewCookie()
	}

	if c.UIDs == nil {
		c.UIDs = make(map[string]UIDEntry)
	}

	// Clean up expired entries
	c.cleanExpired()

	return &c
}

// Encode encodes the cookie to a base64 string
func (c *Cookie) Encode() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// GetUID returns the user ID for a bidder
func (c *Cookie) GetUID(bidder string) (string, bool) {
	if c.OptOut {
		return "", false
	}

	entry, ok := c.UIDs[bidder]
	if !ok {
		return "", false
	}

	if time.Now().After(entry.Expires) {
		delete(c.UIDs, bidder)
		return "", false
	}

	return entry.UID, true
}

// SetUID sets the user ID for a bidder
func (c *Cookie) SetUID(bidder, uid string) {
	if c.OptOut {
		return
	}

	c.UIDs[bidder] = UIDEntry{
		UID:     uid,
		Expires: time.Now().Add(DefaultCookieTTL),
	}
}

// DeleteUID removes the user ID for a bidder
func (c *Cookie) DeleteUID(bidder string) {
	delete(c.UIDs, bidder)
}

// HasLiveSync returns true if the bidder has a non-expired sync
func (c *Cookie) HasLiveSync(bidder string) bool {
	_, ok := c.GetUID(bidder)
	return ok
}

// LiveSyncCount returns the number of live syncs
func (c *Cookie) LiveSyncCount() int {
	c.cleanExpired()
	return len(c.UIDs)
}

// SyncedBidders returns a list of bidders with live syncs
func (c *Cookie) SyncedBidders() []string {
	c.cleanExpired()
	bidders := make([]string, 0, len(c.UIDs))
	for bidder := range c.UIDs {
		bidders = append(bidders, bidder)
	}
	return bidders
}

// SetOptOut marks the user as opted out
func (c *Cookie) SetOptOut(optOut bool) {
	c.OptOut = optOut
	if optOut {
		c.UIDs = make(map[string]UIDEntry)
	}
}

// cleanExpired removes expired UID entries
func (c *Cookie) cleanExpired() {
	now := time.Now()
	for bidder, entry := range c.UIDs {
		if now.After(entry.Expires) {
			delete(c.UIDs, bidder)
		}
	}
}

// WriteCookie writes the cookie to the HTTP response
func (c *Cookie) WriteCookie(w http.ResponseWriter, domain string, ttl time.Duration) error {
	encoded, err := c.Encode()
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     UIDsCookieName,
		Value:    encoded,
		Expires:  time.Now().Add(ttl),
		Path:     "/",
		Domain:   domain,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

	return nil
}
