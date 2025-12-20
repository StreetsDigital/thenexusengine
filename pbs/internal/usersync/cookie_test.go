package usersync

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewCookie(t *testing.T) {
	c := NewCookie()
	if c.UIDs == nil {
		t.Error("UIDs should not be nil")
	}
	if c.OptOut {
		t.Error("OptOut should be false by default")
	}
	if c.Created.IsZero() {
		t.Error("Created should be set")
	}
}

func TestCookieSetGetUID(t *testing.T) {
	c := NewCookie()

	// Set UID
	c.SetUID("appnexus", "test-uid-123")

	// Get UID
	uid := c.GetUID("appnexus")
	if uid != "test-uid-123" {
		t.Errorf("Expected test-uid-123, got %s", uid)
	}

	// Check HasUID
	if !c.HasUID("appnexus") {
		t.Error("HasUID should return true for appnexus")
	}

	// Unknown bidder
	if c.HasUID("unknown") {
		t.Error("HasUID should return false for unknown bidder")
	}
}

func TestCookieDeleteUID(t *testing.T) {
	c := NewCookie()
	c.SetUID("appnexus", "test-uid-123")

	// Verify it exists
	if !c.HasUID("appnexus") {
		t.Error("UID should exist before delete")
	}

	// Delete
	c.DeleteUID("appnexus")

	// Verify it's gone
	if c.HasUID("appnexus") {
		t.Error("UID should not exist after delete")
	}
}

func TestCookieSyncCount(t *testing.T) {
	c := NewCookie()
	c.SetUID("appnexus", "uid1")
	c.SetUID("rubicon", "uid2")
	c.SetUID("pubmatic", "uid3")

	if c.SyncCount() != 3 {
		t.Errorf("Expected sync count 3, got %d", c.SyncCount())
	}
}

func TestCookieOptOut(t *testing.T) {
	c := NewCookie()
	c.SetUID("appnexus", "test-uid")

	// Set opt out
	c.SetOptOut(true)

	if !c.IsOptOut() {
		t.Error("Should be opted out")
	}

	// UIDs should be cleared
	if c.SyncCount() != 0 {
		t.Error("UIDs should be cleared on opt out")
	}
}

func TestCookieToHTTPCookie(t *testing.T) {
	c := NewCookie()
	c.SetUID("appnexus", "test-uid-123")

	httpCookie, err := c.ToHTTPCookie("example.com")
	if err != nil {
		t.Fatalf("Failed to create HTTP cookie: %v", err)
	}

	if httpCookie.Name != CookieName {
		t.Errorf("Expected cookie name %s, got %s", CookieName, httpCookie.Name)
	}
	if httpCookie.Domain != "example.com" {
		t.Errorf("Expected domain example.com, got %s", httpCookie.Domain)
	}
	if !httpCookie.Secure {
		t.Error("Cookie should be secure")
	}
	if !httpCookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}
	if httpCookie.SameSite != http.SameSiteNoneMode {
		t.Error("Cookie should have SameSite=None")
	}
}

func TestParseCookie(t *testing.T) {
	// Create a cookie
	c := NewCookie()
	c.SetUID("appnexus", "parsed-uid")

	httpCookie, _ := c.ToHTTPCookie("example.com")

	// Create a request with the cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(httpCookie)

	// Parse it
	parsed := ParseCookie(req)

	if parsed.GetUID("appnexus") != "parsed-uid" {
		t.Errorf("Expected parsed-uid, got %s", parsed.GetUID("appnexus"))
	}
}

func TestParseCookie_Empty(t *testing.T) {
	// Request without cookie
	req := httptest.NewRequest("GET", "/", nil)

	parsed := ParseCookie(req)

	if parsed == nil {
		t.Error("Should return a valid cookie even if none exists")
	}
	if parsed.SyncCount() != 0 {
		t.Error("New cookie should have no UIDs")
	}
}

func TestParseCookie_Invalid(t *testing.T) {
	// Request with invalid cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  CookieName,
		Value: "not-valid-base64!!!",
	})

	parsed := ParseCookie(req)

	// Should return new empty cookie
	if parsed == nil {
		t.Error("Should return a valid cookie for invalid input")
	}
	if parsed.SyncCount() != 0 {
		t.Error("Invalid cookie should result in empty UIDs")
	}
}

func TestGetAllUIDs(t *testing.T) {
	c := NewCookie()
	c.SetUID("appnexus", "uid1")
	c.SetUID("rubicon", "uid2")

	uids := c.GetAllUIDs()

	if len(uids) != 2 {
		t.Errorf("Expected 2 UIDs, got %d", len(uids))
	}
	if uids["appnexus"] != "uid1" {
		t.Errorf("Expected uid1 for appnexus, got %s", uids["appnexus"])
	}
	if uids["rubicon"] != "uid2" {
		t.Errorf("Expected uid2 for rubicon, got %s", uids["rubicon"])
	}
}

func TestCookieExpiredUID(t *testing.T) {
	c := NewCookie()

	// Manually set an expired UID
	c.mu.Lock()
	c.UIDs["expired"] = UID{
		UID:     "old-uid",
		Expires: time.Now().Add(-time.Hour), // Expired
	}
	c.mu.Unlock()

	// Should not return expired UID
	if c.GetUID("expired") != "" {
		t.Error("Should not return expired UID")
	}
	if c.HasUID("expired") {
		t.Error("HasUID should return false for expired UID")
	}
}
