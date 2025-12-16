package usersync

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewPBSCookie(t *testing.T) {
	cookie := NewPBSCookie()

	if cookie.UIDs == nil {
		t.Error("Expected UIDs map to be initialized")
	}

	if cookie.OptOut {
		t.Error("Expected OptOut to be false by default")
	}

	if cookie.Birthday == nil {
		t.Error("Expected Birthday to be set")
	}
}

func TestPBSCookieHasUID(t *testing.T) {
	cookie := NewPBSCookie()

	// Test with no UID
	if cookie.HasUID("appnexus") {
		t.Error("Expected HasUID to return false for missing UID")
	}

	// Add a UID
	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)

	if !cookie.HasUID("appnexus") {
		t.Error("Expected HasUID to return true for existing UID")
	}

	// Test with expired UID
	expiredTime := time.Now().Add(-time.Hour)
	cookie.SetUID("rubicon", "user456", &expiredTime)

	if cookie.HasUID("rubicon") {
		t.Error("Expected HasUID to return false for expired UID")
	}
}

func TestPBSCookieGetUID(t *testing.T) {
	cookie := NewPBSCookie()

	// Test with no UID
	uid, found := cookie.GetUID("appnexus")
	if found {
		t.Error("Expected GetUID to return false for missing UID")
	}
	if uid != "" {
		t.Errorf("Expected empty UID, got '%s'", uid)
	}

	// Add a UID
	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)

	uid, found = cookie.GetUID("appnexus")
	if !found {
		t.Error("Expected GetUID to return true for existing UID")
	}
	if uid != "user123" {
		t.Errorf("Expected UID 'user123', got '%s'", uid)
	}
}

func TestPBSCookieSetUID(t *testing.T) {
	cookie := NewPBSCookie()

	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)

	if len(cookie.UIDs) != 1 {
		t.Errorf("Expected 1 UID, got %d", len(cookie.UIDs))
	}

	entry := cookie.UIDs["appnexus"]
	if entry.UID != "user123" {
		t.Errorf("Expected UID 'user123', got '%s'", entry.UID)
	}
}

func TestPBSCookieDeleteUID(t *testing.T) {
	cookie := NewPBSCookie()

	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)
	cookie.SetUID("rubicon", "user456", &expires)

	cookie.DeleteUID("appnexus")

	if len(cookie.UIDs) != 1 {
		t.Errorf("Expected 1 UID after delete, got %d", len(cookie.UIDs))
	}

	if cookie.HasUID("appnexus") {
		t.Error("Expected appnexus UID to be deleted")
	}
}

func TestPBSCookieSyncersNeedingSync(t *testing.T) {
	cookie := NewPBSCookie()

	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)

	syncersToCheck := []string{"appnexus", "rubicon", "pubmatic"}
	needingSync := cookie.SyncersNeedingSync(syncersToCheck)

	if len(needingSync) != 2 {
		t.Errorf("Expected 2 syncers needing sync, got %d", len(needingSync))
	}

	// Check that appnexus is not in the list
	for _, s := range needingSync {
		if s == "appnexus" {
			t.Error("appnexus should not need sync")
		}
	}
}

func TestPBSCookieMarshalUnmarshal(t *testing.T) {
	cookie := NewPBSCookie()
	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)
	cookie.SetUID("rubicon", "user456", &expires)

	// Marshal
	data, err := cookie.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	cookie2, err := UnmarshalPBSCookie(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(cookie2.UIDs) != 2 {
		t.Errorf("Expected 2 UIDs after unmarshal, got %d", len(cookie2.UIDs))
	}

	uid, _ := cookie2.GetUID("appnexus")
	if uid != "user123" {
		t.Errorf("Expected UID 'user123', got '%s'", uid)
	}
}

func TestUnmarshalPBSCookieEmpty(t *testing.T) {
	cookie, err := UnmarshalPBSCookie(nil)
	if err != nil {
		t.Fatalf("Unmarshal nil failed: %v", err)
	}

	if cookie.UIDs == nil {
		t.Error("Expected UIDs map to be initialized")
	}
}

func TestCookieManager(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// Create a test request with no cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Read should return empty cookie
	cookie, err := manager.Read(req)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(cookie.UIDs) != 0 {
		t.Errorf("Expected empty UIDs, got %d", len(cookie.UIDs))
	}

	// Set a UID
	err = manager.SetUID(w, req, "appnexus", "user123")
	if err != nil {
		t.Fatalf("SetUID failed: %v", err)
	}

	// Check cookie was set in response
	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Error("Expected cookie to be set")
	}

	found := false
	for _, c := range cookies {
		if c.Name == config.CookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected uids cookie to be set")
	}
}

func TestCookieCountUIDs(t *testing.T) {
	cookie := NewPBSCookie()

	if cookie.CountUIDs() != 0 {
		t.Errorf("Expected 0 UIDs, got %d", cookie.CountUIDs())
	}

	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)
	cookie.SetUID("rubicon", "user456", &expires)

	if cookie.CountUIDs() != 2 {
		t.Errorf("Expected 2 UIDs, got %d", cookie.CountUIDs())
	}

	// Add expired UID
	expiredTime := time.Now().Add(-time.Hour)
	cookie.SetUID("pubmatic", "user789", &expiredTime)

	// Should still be 2 (expired doesn't count)
	if cookie.CountUIDs() != 2 {
		t.Errorf("Expected 2 UIDs (expired excluded), got %d", cookie.CountUIDs())
	}
}

func TestCookieGetAllUIDs(t *testing.T) {
	cookie := NewPBSCookie()

	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)
	cookie.SetUID("rubicon", "user456", &expires)

	uids := cookie.GetAllUIDs()

	if len(uids) != 2 {
		t.Errorf("Expected 2 UIDs, got %d", len(uids))
	}

	if uids["appnexus"] != "user123" {
		t.Errorf("Expected appnexus UID 'user123', got '%s'", uids["appnexus"])
	}
}
