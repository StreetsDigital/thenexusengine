package usersync

import (
	"encoding/base64"
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

func TestCookieOptOut(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	req := httptest.NewRequest(http.MethodPost, "/optout", nil)
	w := httptest.NewRecorder()

	err := manager.OptOut(w, req)
	if err != nil {
		t.Fatalf("OptOut failed: %v", err)
	}

	// Read back the cookie
	resp := w.Result()
	cookies := resp.Cookies()

	// Create new request with the cookie
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}

	pbsCookie, _ := manager.Read(req2)
	if !pbsCookie.OptOut {
		t.Error("Expected OptOut to be true")
	}
}

func TestCookieOptIn(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// First opt out
	req := httptest.NewRequest(http.MethodPost, "/optout", nil)
	w := httptest.NewRecorder()
	manager.OptOut(w, req)

	// Then opt in
	req2 := httptest.NewRequest(http.MethodPost, "/optin", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()

	err := manager.OptIn(w2, req2)
	if err != nil {
		t.Fatalf("OptIn failed: %v", err)
	}
}

func TestIsOptedOut(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// Without cookie, should not be opted out
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if manager.IsOptedOut(req) {
		t.Error("Expected IsOptedOut to return false without cookie")
	}

	// After opt-out, should be opted out
	w := httptest.NewRecorder()
	manager.OptOut(w, req)

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(c)
	}

	if !manager.IsOptedOut(req2) {
		t.Error("Expected IsOptedOut to return true after opt-out")
	}
}

func TestParseOptOutCookie(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// Without legacy cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if manager.ParseOptOutCookie(req) {
		t.Error("Expected false without legacy optout cookie")
	}

	// With legacy cookie
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "uids_optout", Value: "true"})
	if !manager.ParseOptOutCookie(req2) {
		t.Error("Expected true with legacy optout cookie")
	}
}

func TestCookieToJSON(t *testing.T) {
	cookie := NewPBSCookie()
	expires := time.Now().Add(time.Hour)
	cookie.SetUID("appnexus", "user123", &expires)

	jsonStr := cookie.ToJSON()
	if jsonStr == "" || jsonStr == "{}" {
		t.Error("Expected non-empty JSON")
	}

	// Should contain the UID
	if !containsSubstring(jsonStr, "user123") {
		t.Error("Expected JSON to contain UID")
	}
}

func TestCookieReaderWithInvalidCookie(t *testing.T) {
	config := DefaultCookieSyncConfig()
	reader := NewCookieReader(config)

	// Test with invalid base64
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: "not-valid-base64!!!"})

	cookie, err := reader.Read(req)
	if err != nil {
		t.Fatalf("Read should not error: %v", err)
	}
	// Should return a new empty cookie
	if len(cookie.UIDs) != 0 {
		t.Error("Expected empty cookie for invalid base64")
	}
}

func TestCookieReaderWithInvalidJSON(t *testing.T) {
	config := DefaultCookieSyncConfig()
	reader := NewCookieReader(config)

	// Test with valid base64 but invalid JSON
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	invalidJSON := base64.URLEncoding.EncodeToString([]byte("not json"))
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: invalidJSON})

	cookie, err := reader.Read(req)
	if err != nil {
		t.Fatalf("Read should not error: %v", err)
	}
	// Should return a new empty cookie
	if len(cookie.UIDs) != 0 {
		t.Error("Expected empty cookie for invalid JSON")
	}
}

func TestSetUIDWithEmptyUID(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// First set a UID
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	manager.SetUID(w, req, "appnexus", "user123")

	// Then delete it with empty UID
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	manager.SetUID(w2, req2, "appnexus", "")

	// Verify it was deleted
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w2.Result().Cookies() {
		req3.AddCookie(c)
	}
	cookie, _ := manager.Read(req3)
	if cookie.HasUID("appnexus") {
		t.Error("Expected UID to be deleted")
	}
}

func TestSetUIDWithZeroUID(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// "0" should also delete the UID
	manager.SetUID(w, req, "appnexus", "user123")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	manager.SetUID(w2, req2, "appnexus", "0")

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w2.Result().Cookies() {
		req3.AddCookie(c)
	}
	cookie, _ := manager.Read(req3)
	if cookie.HasUID("appnexus") {
		t.Error("Expected UID to be deleted with '0'")
	}
}

func TestSetUIDWithOptedOutUser(t *testing.T) {
	config := DefaultCookieSyncConfig()
	manager := NewCookieManager(config)

	// First opt out
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	manager.OptOut(w, req)

	// Try to set UID - should be ignored
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	manager.SetUID(w2, req2, "appnexus", "user123")

	// Cookie should still be opted out with no UIDs
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w2.Result().Cookies() {
		req3.AddCookie(c)
	}
	cookie, _ := manager.Read(req3)
	if cookie.HasUID("appnexus") {
		t.Error("Expected no UID for opted-out user")
	}
}

func TestNilCookieMethods(t *testing.T) {
	var cookie *PBSCookie

	// HasUID on nil cookie
	if cookie.HasUID("appnexus") {
		t.Error("Expected false for nil cookie")
	}

	// GetUID on nil cookie
	uid, found := cookie.GetUID("appnexus")
	if found || uid != "" {
		t.Error("Expected empty result for nil cookie")
	}

	// CountUIDs on nil cookie
	if cookie.CountUIDs() != 0 {
		t.Error("Expected 0 for nil cookie")
	}

	// GetAllUIDs on nil cookie
	uids := cookie.GetAllUIDs()
	if len(uids) != 0 {
		t.Error("Expected empty map for nil cookie")
	}
}

func TestCookieWithNilUIDs(t *testing.T) {
	cookie := &PBSCookie{UIDs: nil}

	// HasUID should handle nil UIDs
	if cookie.HasUID("appnexus") {
		t.Error("Expected false with nil UIDs")
	}

	// SetUID should initialize UIDs map
	cookie.SetUID("appnexus", "user123", nil)
	if !cookie.HasUID("appnexus") {
		t.Error("Expected true after SetUID")
	}
}

func TestDeleteUIDFromNilMap(t *testing.T) {
	cookie := &PBSCookie{UIDs: nil}

	// Should not panic
	cookie.DeleteUID("appnexus")
}

func TestCookieWriterWithDomain(t *testing.T) {
	config := DefaultCookieSyncConfig()
	config.CookieDomain = "example.com"
	writer := NewCookieWriter(config)

	w := httptest.NewRecorder()
	cookie := NewPBSCookie()
	cookie.SetUID("appnexus", "user123", nil)

	err := writer.Write(w, cookie)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("Expected cookie to be set")
	}

	// http.Cookie normalizes domain (strips leading dot)
	if cookies[0].Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", cookies[0].Domain)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
