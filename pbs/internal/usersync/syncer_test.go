package usersync

import (
	"testing"
	"time"
)

func TestNewSyncer(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {
			Key:      "adnxs",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ib.adnxs.com/getuid?{{redirect_url}}",
				UserMacro: "$UID",
			},
		},
		"rubicon": {
			Key:      "rubicon",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://pixel.rubiconproject.com/exchange/sync.php?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}",
				UserMacro: "${USER_ID}",
			},
		},
	}

	config := DefaultCookieSyncConfig()
	config.ExternalURL = "https://pbs.example.com"

	syncer := NewSyncer(configs, config)

	// Test syncer key mapping
	if syncer.GetSyncerKey("appnexus") != "adnxs" {
		t.Errorf("Expected syncer key 'adnxs' for appnexus, got '%s'", syncer.GetSyncerKey("appnexus"))
	}

	if syncer.GetSyncerKey("rubicon") != "rubicon" {
		t.Errorf("Expected syncer key 'rubicon' for rubicon, got '%s'", syncer.GetSyncerKey("rubicon"))
	}

	// Test HasSyncer
	if !syncer.HasSyncer("appnexus") {
		t.Error("Expected HasSyncer to return true for appnexus")
	}

	if syncer.HasSyncer("unknown") {
		t.Error("Expected HasSyncer to return false for unknown bidder")
	}
}

func TestBuildSyncURL(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {
			Key:      "adnxs",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ib.adnxs.com/getuid?gdpr={{gdpr}}&consent={{gdpr_consent}}&redir={{redirect_url}}",
				UserMacro: "$UID",
			},
		},
	}

	config := DefaultCookieSyncConfig()
	config.ExternalURL = "https://pbs.example.com"

	syncer := NewSyncer(configs, config)

	privacy := &PrivacyInfo{
		GDPRApplies: true,
		GDPRConsent: "BOtest123",
		USPrivacy:   "1YNN",
	}

	result, err := syncer.BuildSyncURL("appnexus", SyncTypeRedirect, privacy)
	if err != nil {
		t.Fatalf("BuildSyncURL failed: %v", err)
	}

	if result.BidderCode != "appnexus" {
		t.Errorf("Expected bidder code 'appnexus', got '%s'", result.BidderCode)
	}

	if result.SyncerKey != "adnxs" {
		t.Errorf("Expected syncer key 'adnxs', got '%s'", result.SyncerKey)
	}

	if result.Type != SyncTypeRedirect {
		t.Errorf("Expected sync type 'redirect', got '%s'", result.Type)
	}

	// Check URL contains replaced macros
	if result.URL == "" {
		t.Error("Expected non-empty URL")
	}

	// Check GDPR macro was replaced
	if contains(result.URL, "{{gdpr}}") {
		t.Error("GDPR macro was not replaced")
	}
}

func TestBuildSyncURLUnknownBidder(t *testing.T) {
	configs := map[string]*SyncerConfig{}
	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	_, err := syncer.BuildSyncURL("unknown", SyncTypeRedirect, nil)
	if err == nil {
		t.Error("Expected error for unknown bidder")
	}
}

func TestChooseBidders(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {
			Key:      "adnxs",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com"},
		},
		"rubicon": {
			Key:      "rubicon",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com"},
		},
		"pubmatic": {
			Key:      "pubmatic",
			Supports: []SyncType{SyncTypeIFrame},
			Default:  SyncTypeIFrame,
			IFrame:   &SyncURLTemplate{URL: "https://example.com"},
		},
	}

	config := DefaultCookieSyncConfig()
	config.DefaultLimit = 10
	syncer := NewSyncer(configs, config)

	// Test with empty cookie - should return all requested bidders
	cookie := NewPBSCookie()
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"appnexus", "rubicon", "pubmatic"},
		Limit:            10,
		Cookie:           cookie,
		CooperativeSync:  false,
	}

	result := syncer.ChooseBidders(req)

	if len(result.BiddersToSync) != 3 {
		t.Errorf("Expected 3 bidders to sync, got %d", len(result.BiddersToSync))
	}

	// Test with cookie that has one UID - should exclude that bidder
	expires := time.Now().Add(time.Hour)
	cookie.SetUID("adnxs", "user123", &expires)

	result = syncer.ChooseBidders(req)

	if len(result.BiddersToSync) != 2 {
		t.Errorf("Expected 2 bidders to sync (excluding appnexus), got %d", len(result.BiddersToSync))
	}

	// Check appnexus was rejected
	if _, ok := result.RejectedBidders["appnexus"]; !ok {
		t.Error("Expected appnexus to be rejected as already synced")
	}
}

func TestChooseBiddersWithLimit(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder1": {Key: "bidder1", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"bidder2": {Key: "bidder2", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"bidder3": {Key: "bidder3", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"bidder1", "bidder2", "bidder3"},
		Limit:            2,
		Cookie:           NewPBSCookie(),
	}

	result := syncer.ChooseBidders(req)

	if len(result.BiddersToSync) != 2 {
		t.Errorf("Expected 2 bidders (limit), got %d", len(result.BiddersToSync))
	}
}

func TestPrivacyInfo(t *testing.T) {
	privacy := &PrivacyInfo{
		GDPRApplies: true,
		GDPRConsent: "BOtest123",
		USPrivacy:   "1YNN",
		GPPSID:      []int{2, 6},
	}

	if privacy.GDPRSignal() != "1" {
		t.Errorf("Expected GDPR signal '1', got '%s'", privacy.GDPRSignal())
	}

	privacy.GDPRApplies = false
	if privacy.GDPRSignal() != "0" {
		t.Errorf("Expected GDPR signal '0', got '%s'", privacy.GDPRSignal())
	}
}

func TestDefaultSyncerConfigs(t *testing.T) {
	configs := DefaultSyncerConfigs()

	// Check we have configs for expected bidders
	expectedBidders := []string{
		"appnexus", "rubicon", "pubmatic", "ix", "criteo",
		"openx", "triplelift", "sharethrough", "sovrn",
	}

	for _, bidder := range expectedBidders {
		if _, ok := configs[bidder]; !ok {
			t.Errorf("Missing sync config for bidder: %s", bidder)
		}
	}

	// Check appnexus config
	appnexus := configs["appnexus"]
	if appnexus.Key != "adnxs" {
		t.Errorf("Expected appnexus syncer key 'adnxs', got '%s'", appnexus.Key)
	}

	if appnexus.Redirect == nil || appnexus.Redirect.URL == "" {
		t.Error("Expected appnexus to have redirect URL")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
