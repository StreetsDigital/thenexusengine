package usersync

import (
	"strings"
	"testing"
)

func TestNewSyncer(t *testing.T) {
	config := SyncerConfig{
		BidderCode:      "appnexus",
		RedirectSyncURL: "https://example.com/sync?redirect={{redirect_url}}",
		Enabled:         true,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	if syncer.BidderCode() != "appnexus" {
		t.Errorf("Expected bidder code appnexus, got %s", syncer.BidderCode())
	}
	if !syncer.IsEnabled() {
		t.Error("Syncer should be enabled")
	}
}

func TestSyncerGetSync_Redirect(t *testing.T) {
	config := SyncerConfig{
		BidderCode:      "appnexus",
		RedirectSyncURL: "https://example.com/sync?gdpr={{gdpr}}&consent={{gdpr_consent}}&redirect={{redirect_url}}",
		Enabled:         true,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	syncInfo, err := syncer.GetSync(SyncTypeRedirect, "1", "consent-string", "")
	if err != nil {
		t.Fatalf("GetSync failed: %v", err)
	}

	if syncInfo.Type != SyncTypeRedirect {
		t.Errorf("Expected redirect type, got %s", syncInfo.Type)
	}
	if syncInfo.Bidder != "appnexus" {
		t.Errorf("Expected bidder appnexus, got %s", syncInfo.Bidder)
	}
	if !strings.Contains(syncInfo.URL, "gdpr=1") {
		t.Error("URL should contain gdpr=1")
	}
	if !strings.Contains(syncInfo.URL, "consent=consent-string") {
		t.Error("URL should contain consent string")
	}
	if !strings.Contains(syncInfo.URL, "pbs.example.com") {
		t.Error("URL should contain redirect back to PBS")
	}
}

func TestSyncerGetSync_Iframe(t *testing.T) {
	config := SyncerConfig{
		BidderCode:    "rubicon",
		IframeSyncURL: "https://example.com/iframe?redirect={{redirect_url}}",
		Enabled:       true,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	syncInfo, err := syncer.GetSync(SyncTypeIframe, "0", "", "")
	if err != nil {
		t.Fatalf("GetSync failed: %v", err)
	}

	if syncInfo.Type != SyncTypeIframe {
		t.Errorf("Expected iframe type, got %s", syncInfo.Type)
	}
}

func TestSyncerGetSync_DefaultType(t *testing.T) {
	config := SyncerConfig{
		BidderCode:      "appnexus",
		RedirectSyncURL: "https://example.com/redirect",
		IframeSyncURL:   "https://example.com/iframe",
		Enabled:         true,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	// Empty string should prefer redirect
	syncInfo, err := syncer.GetSync("", "0", "", "")
	if err != nil {
		t.Fatalf("GetSync failed: %v", err)
	}

	if syncInfo.Type != SyncTypeRedirect {
		t.Errorf("Should prefer redirect, got %s", syncInfo.Type)
	}
}

func TestSyncerGetSync_Disabled(t *testing.T) {
	config := SyncerConfig{
		BidderCode:      "appnexus",
		RedirectSyncURL: "https://example.com/sync",
		Enabled:         false,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	_, err := syncer.GetSync(SyncTypeRedirect, "0", "", "")
	if err == nil {
		t.Error("Should return error when disabled")
	}
}

func TestSyncerGetSync_NoURL(t *testing.T) {
	config := SyncerConfig{
		BidderCode: "appnexus",
		Enabled:    true,
		// No URLs configured
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	_, err := syncer.GetSync(SyncTypeRedirect, "0", "", "")
	if err == nil {
		t.Error("Should return error when no URL configured")
	}
}

func TestDefaultSyncerConfigs(t *testing.T) {
	configs := DefaultSyncerConfigs()

	// Check that common bidders are configured
	expectedBidders := []string{"appnexus", "rubicon", "pubmatic", "openx", "triplelift"}

	for _, bidder := range expectedBidders {
		config, ok := configs[bidder]
		if !ok {
			t.Errorf("Expected %s to be configured", bidder)
			continue
		}
		if !config.Enabled {
			t.Errorf("Expected %s to be enabled", bidder)
		}
		if config.RedirectSyncURL == "" && config.IframeSyncURL == "" {
			t.Errorf("Expected %s to have at least one sync URL", bidder)
		}
	}
}

func TestSyncerUSPrivacy(t *testing.T) {
	config := SyncerConfig{
		BidderCode:      "rubicon",
		RedirectSyncURL: "https://example.com/sync?us_privacy={{us_privacy}}&redirect={{redirect_url}}",
		Enabled:         true,
	}

	syncer := NewSyncer(config, "https://pbs.example.com")

	syncInfo, err := syncer.GetSync(SyncTypeRedirect, "0", "", "1YNN")
	if err != nil {
		t.Fatalf("GetSync failed: %v", err)
	}

	if !strings.Contains(syncInfo.URL, "us_privacy=1YNN") {
		t.Errorf("URL should contain US privacy string, got: %s", syncInfo.URL)
	}
}
