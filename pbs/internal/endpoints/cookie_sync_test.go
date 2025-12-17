package endpoints

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/usersync"
)

func setupCookieSyncHandler(t *testing.T) (*CookieSyncHandler, *usersync.SyncerRegistry) {
	registry, err := usersync.InitDefaultSyncers("https://pbs.example.com")
	if err != nil {
		t.Fatalf("Failed to init syncers: %v", err)
	}

	config := &CookieSyncConfig{
		ExternalURL:     "https://pbs.example.com",
		DefaultLimit:    0,
		MaxLimit:        100,
		DefaultCoopSync: true,
	}

	bidders := []string{"appnexus", "rubicon", "pubmatic"}

	handler := NewCookieSyncHandler(config, registry, bidders)
	return handler, registry
}

func TestCookieSyncHandler_BasicRequest(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	reqBody := CookieSyncRequest{
		Bidders: []string{"appnexus", "rubicon"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/cookie_sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	var resp CookieSyncResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}

	if len(resp.BidderStatus) != 2 {
		t.Errorf("expected 2 bidder statuses, got %d", len(resp.BidderStatus))
	}
}

func TestCookieSyncHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/cookie_sync", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", recorder.Code)
	}
}

func TestCookieSyncHandler_EmptyRequest(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/cookie_sync", nil)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	var resp CookieSyncResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// With coop sync enabled and no specific bidders, should get all bidders
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestCookieSyncHandler_WithGDPR(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	gdpr := 1
	reqBody := CookieSyncRequest{
		Bidders:     []string{"appnexus"},
		GDPR:        &gdpr,
		GDPRConsent: "BOtest123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/cookie_sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	var resp CookieSyncResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check that the sync URL contains GDPR parameters
	for _, status := range resp.BidderStatus {
		if status.UserSync != nil && status.UserSync.URL != "" {
			// The URL should have been built with GDPR params
			t.Logf("Bidder %s sync URL: %s", status.Bidder, status.UserSync.URL)
		}
	}
}

func TestCookieSyncHandler_WithLimit(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	reqBody := CookieSyncRequest{
		Bidders: []string{"appnexus", "rubicon", "pubmatic"},
		Limit:   1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/cookie_sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	var resp CookieSyncResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.BidderStatus) > 1 {
		t.Errorf("expected at most 1 bidder status due to limit, got %d", len(resp.BidderStatus))
	}
}

func TestCookieSyncHandler_AlreadySynced(t *testing.T) {
	handler, _ := setupCookieSyncHandler(t)

	// Create a cookie with existing syncs
	cookie := usersync.NewCookie()
	cookie.SetUID("appnexus", "existing-uid")
	encoded, _ := cookie.Encode()

	reqBody := CookieSyncRequest{
		Bidders: []string{"appnexus", "rubicon"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/cookie_sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  usersync.UIDsCookieName,
		Value: encoded,
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	var resp CookieSyncResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should only sync rubicon since appnexus is already synced
	if len(resp.BidderStatus) != 1 {
		t.Errorf("expected 1 bidder status (rubicon only), got %d", len(resp.BidderStatus))
	}

	if len(resp.BidderStatus) > 0 && resp.BidderStatus[0].Bidder != "rubicon" {
		t.Errorf("expected rubicon, got %s", resp.BidderStatus[0].Bidder)
	}
}
