package usersync

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewCookie(t *testing.T) {
	cookie := NewCookie()

	if cookie == nil {
		t.Fatal("NewCookie returned nil")
	}

	if cookie.UIDs == nil {
		t.Error("UIDs map should be initialized")
	}

	if cookie.OptOut {
		t.Error("OptOut should be false by default")
	}

	if cookie.Birthday == nil {
		t.Error("Birthday should be set")
	}
}

func TestCookieSetAndGetUID(t *testing.T) {
	cookie := NewCookie()

	// Test setting and getting a UID
	cookie.SetUID("appnexus", "user123")

	uid, ok := cookie.GetUID("appnexus")
	if !ok {
		t.Error("GetUID should return true for existing bidder")
	}
	if uid != "user123" {
		t.Errorf("GetUID returned %s, expected user123", uid)
	}

	// Test getting non-existent UID
	_, ok = cookie.GetUID("nonexistent")
	if ok {
		t.Error("GetUID should return false for non-existent bidder")
	}
}

func TestCookieOptOut(t *testing.T) {
	cookie := NewCookie()

	// Set some UIDs
	cookie.SetUID("appnexus", "user123")
	cookie.SetUID("rubicon", "user456")

	// Opt out
	cookie.SetOptOut(true)

	// UIDs should be cleared
	if len(cookie.UIDs) != 0 {
		t.Error("UIDs should be cleared on opt out")
	}

	// Setting UIDs should have no effect
	cookie.SetUID("appnexus", "newuser")
	_, ok := cookie.GetUID("appnexus")
	if ok {
		t.Error("GetUID should return false when opted out")
	}
}

func TestCookieDeleteUID(t *testing.T) {
	cookie := NewCookie()
	cookie.SetUID("appnexus", "user123")

	cookie.DeleteUID("appnexus")

	_, ok := cookie.GetUID("appnexus")
	if ok {
		t.Error("GetUID should return false after delete")
	}
}

func TestCookieHasLiveSync(t *testing.T) {
	cookie := NewCookie()

	if cookie.HasLiveSync("appnexus") {
		t.Error("HasLiveSync should return false for new cookie")
	}

	cookie.SetUID("appnexus", "user123")

	if !cookie.HasLiveSync("appnexus") {
		t.Error("HasLiveSync should return true after SetUID")
	}
}

func TestCookieSyncedBidders(t *testing.T) {
	cookie := NewCookie()
	cookie.SetUID("appnexus", "user123")
	cookie.SetUID("rubicon", "user456")

	bidders := cookie.SyncedBidders()
	if len(bidders) != 2 {
		t.Errorf("SyncedBidders returned %d bidders, expected 2", len(bidders))
	}
}

func TestCookieEncodeDecode(t *testing.T) {
	original := NewCookie()
	original.SetUID("appnexus", "user123")
	original.SetUID("rubicon", "user456")

	// Encode
	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded := DecodeCookie(encoded)

	// Verify
	uid1, ok := decoded.GetUID("appnexus")
	if !ok || uid1 != "user123" {
		t.Error("Decoded cookie doesn't have correct appnexus UID")
	}

	uid2, ok := decoded.GetUID("rubicon")
	if !ok || uid2 != "user456" {
		t.Error("Decoded cookie doesn't have correct rubicon UID")
	}
}

func TestParseCookieFromRequest(t *testing.T) {
	original := NewCookie()
	original.SetUID("appnexus", "user123")

	encoded, _ := original.Encode()

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  UIDsCookieName,
		Value: encoded,
	})

	parsed := ParseCookie(req)

	uid, ok := parsed.GetUID("appnexus")
	if !ok || uid != "user123" {
		t.Error("ParseCookie didn't parse cookie correctly")
	}
}

func TestCookieWriteCookie(t *testing.T) {
	cookie := NewCookie()
	cookie.SetUID("appnexus", "user123")

	recorder := httptest.NewRecorder()
	err := cookie.WriteCookie(recorder, "", 24*time.Hour)
	if err != nil {
		t.Fatalf("WriteCookie failed: %v", err)
	}

	// Check that cookie was written
	result := recorder.Result()
	cookies := result.Cookies()

	found := false
	for _, c := range cookies {
		if c.Name == UIDsCookieName {
			found = true
			if c.Value == "" {
				t.Error("Cookie value should not be empty")
			}
		}
	}

	if !found {
		t.Error("Cookie was not written to response")
	}
}
