package endpoints

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

func setupSetUIDHandler(t *testing.T) (*SetUIDHandler, *usersync.SyncerRegistry) {
	registry, err := usersync.InitDefaultSyncers("https://pbs.example.com")
	if err != nil {
		t.Fatalf("Failed to init syncers: %v", err)
	}

	config := &SetUIDConfig{
		CookieDomain: "",
		CookieTTL:    24 * time.Hour,
	}

	handler := NewSetUIDHandler(config, registry)
	return handler, registry
}

func TestSetUIDHandler_BasicRequest(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	// Check that cookie was set
	cookies := recorder.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == usersync.UIDsCookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cookie to be set")
	}
}

func TestSetUIDHandler_MissingBidder(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?uid=user123", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", recorder.Code)
	}
}

func TestSetUIDHandler_UnknownBidder(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=unknownbidder&uid=user123", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", recorder.Code)
	}
}

func TestSetUIDHandler_EmptyUID(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	// First set a UID
	cookie := usersync.NewCookie()
	cookie.SetUID("appnexus", "existing-uid")
	encoded, _ := cookie.Encode()

	// Then delete it by sending empty UID
	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=", nil)
	req.AddCookie(&http.Cookie{
		Name:  usersync.UIDsCookieName,
		Value: encoded,
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}

func TestSetUIDHandler_PixelResponse(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123&f=i", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected content-type image/gif, got %s", contentType)
	}

	// Check it's a GIF (starts with GIF89a)
	body := recorder.Body.Bytes()
	if len(body) < 6 || string(body[:6]) != "GIF89a" {
		t.Error("expected GIF response")
	}
}

func TestSetUIDHandler_JSONResponse(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123&f=j", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %s", contentType)
	}
}

func TestSetUIDHandler_BlankResponse(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123&f=b", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	if recorder.Body.Len() != 0 {
		t.Error("expected empty body for blank response")
	}
}

func TestSetUIDHandler_OptedOut(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	// Create an opted-out cookie
	cookie := usersync.NewCookie()
	cookie.SetOptOut(true)
	encoded, _ := cookie.Encode()

	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123", nil)
	req.AddCookie(&http.Cookie{
		Name:  usersync.UIDsCookieName,
		Value: encoded,
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// Should succeed but not store the UID
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}

func TestSetUIDHandler_GDPRNoConsent(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	// GDPR applies but no consent provided
	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=user123&gdpr=1", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// Should return pixel but not store UID
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}

func TestSetUIDHandler_UpdateExisting(t *testing.T) {
	handler, _ := setupSetUIDHandler(t)

	// Create a cookie with existing UID
	cookie := usersync.NewCookie()
	cookie.SetUID("appnexus", "old-uid")
	encoded, _ := cookie.Encode()

	// Update the UID
	req := httptest.NewRequest(http.MethodGet, "/setuid?bidder=appnexus&uid=new-uid", nil)
	req.AddCookie(&http.Cookie{
		Name:  usersync.UIDsCookieName,
		Value: encoded,
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	// Verify the updated cookie
	cookies := recorder.Result().Cookies()
	for _, c := range cookies {
		if c.Name == usersync.UIDsCookieName {
			decoded := usersync.DecodeCookie(c.Value)
			uid, ok := decoded.GetUID("appnexus")
			if !ok {
				t.Error("expected appnexus UID in cookie")
			}
			if uid != "new-uid" {
				t.Errorf("expected UID 'new-uid', got '%s'", uid)
			}
		}
	}
}
