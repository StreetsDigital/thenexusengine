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

func TestNewDefaultSyncer(t *testing.T) {
	config := DefaultCookieSyncConfig()
	config.ExternalURL = "https://pbs.example.com"

	syncer := NewDefaultSyncer(config)

	// Should have all 22 bidders
	bidders := syncer.SupportedBidders()
	if len(bidders) != 22 {
		t.Errorf("Expected 22 bidders, got %d", len(bidders))
	}

	// Test that appnexus maps to adnxs
	if syncer.GetSyncerKey("appnexus") != "adnxs" {
		t.Error("Expected appnexus to map to adnxs")
	}
}

func TestGetSyncerConfigForBidder(t *testing.T) {
	cfg, ok := GetSyncerConfigForBidder("appnexus")
	if !ok {
		t.Fatal("Expected to find config for appnexus")
	}

	if cfg.Key != "adnxs" {
		t.Errorf("Expected key 'adnxs', got '%s'", cfg.Key)
	}

	_, ok = GetSyncerConfigForBidder("nonexistent")
	if ok {
		t.Error("Expected no config for nonexistent bidder")
	}
}

func TestSupportedSyncBidders(t *testing.T) {
	bidders := SupportedSyncBidders()

	if len(bidders) != 22 {
		t.Errorf("Expected 22 bidders, got %d", len(bidders))
	}

	// Check some expected bidders
	found := make(map[string]bool)
	for _, b := range bidders {
		found[b] = true
	}

	expectedBidders := []string{"appnexus", "rubicon", "pubmatic", "criteo"}
	for _, expected := range expectedBidders {
		if !found[expected] {
			t.Errorf("Expected %s in supported bidders", expected)
		}
	}
}

func TestGetBidderForSyncerKey(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Test reverse lookup
	bidder := syncer.GetBidderForSyncerKey("adnxs")
	if bidder != "appnexus" {
		t.Errorf("Expected bidder 'appnexus' for key 'adnxs', got '%s'", bidder)
	}

	bidder = syncer.GetBidderForSyncerKey("rubicon")
	if bidder != "rubicon" {
		t.Errorf("Expected bidder 'rubicon' for key 'rubicon', got '%s'", bidder)
	}

	// Unknown key returns the key itself
	bidder = syncer.GetBidderForSyncerKey("unknown")
	if bidder != "unknown" {
		t.Errorf("Expected 'unknown' for unknown key, got '%s'", bidder)
	}
}

func TestGetSyncerConfig(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	cfg, ok := syncer.GetSyncerConfig("appnexus")
	if !ok {
		t.Fatal("Expected to find config for appnexus")
	}
	if cfg.Key != "adnxs" {
		t.Errorf("Expected key 'adnxs', got '%s'", cfg.Key)
	}

	_, ok = syncer.GetSyncerConfig("unknown")
	if ok {
		t.Error("Expected no config for unknown bidder")
	}
}

func TestSupportedBidders(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	bidders := syncer.SupportedBidders()
	if len(bidders) != 2 {
		t.Errorf("Expected 2 bidders, got %d", len(bidders))
	}
}

func TestBuildSyncs(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {
			Key:      "adnxs",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com/sync?gdpr={{gdpr}}"},
		},
		"rubicon": {
			Key:      "rubicon",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://rubicon.com/sync"},
		},
	}

	config := DefaultCookieSyncConfig()
	config.ExternalURL = "https://pbs.example.com"
	syncer := NewSyncer(configs, config)

	privacy := &PrivacyInfo{GDPRApplies: true, GDPRConsent: "test"}
	results := syncer.BuildSyncs([]string{"appnexus", "rubicon", "unknown"}, "", privacy)

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// First two should succeed
	if results[0].Error != nil {
		t.Errorf("Expected no error for appnexus: %v", results[0].Error)
	}
	if results[1].Error != nil {
		t.Errorf("Expected no error for rubicon: %v", results[1].Error)
	}

	// Third should have error
	if results[2].Error == nil {
		t.Error("Expected error for unknown bidder")
	}
}

func TestChooseBiddersWithGDPREnforcer(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Create a mock enforcer that denies appnexus
	enforcer := &mockGDPREnforcer{denied: map[string]bool{"appnexus": true}}

	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"appnexus", "rubicon"},
		Limit:            10,
		Cookie:           NewPBSCookie(),
		Privacy:          &PrivacyInfo{GDPRApplies: true, GDPRConsent: "test"},
		GDPREnforcer:     enforcer,
	}

	result := syncer.ChooseBidders(req)

	// Only rubicon should be in the list
	if len(result.BiddersToSync) != 1 {
		t.Errorf("Expected 1 bidder to sync, got %d", len(result.BiddersToSync))
	}

	if result.BiddersToSync[0] != "rubicon" {
		t.Errorf("Expected rubicon, got %s", result.BiddersToSync[0])
	}

	// Check appnexus was rejected due to GDPR
	if reason, ok := result.RejectedBidders["appnexus"]; !ok || reason != "GDPR consent denied" {
		t.Errorf("Expected appnexus rejected for GDPR, got: %s", reason)
	}
}

func TestChooseBiddersCooperativeSync(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	config.DefaultLimit = 10
	syncer := NewSyncer(configs, config)

	// With cooperative sync and no requested bidders, should return all supported bidders
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{}, // Empty
		Limit:            10,
		Cookie:           NewPBSCookie(),
		CooperativeSync:  true,
	}

	result := syncer.ChooseBidders(req)

	if len(result.BiddersToSync) != 2 {
		t.Errorf("Expected 2 bidders with cooperative sync, got %d", len(result.BiddersToSync))
	}
}

func TestChooseBiddersWithFilterSettings(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"pubmatic": {Key: "pubmatic", Supports: []SyncType{SyncTypeIFrame}, Default: SyncTypeIFrame, IFrame: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Exclude appnexus from redirect syncs
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"appnexus", "rubicon", "pubmatic"},
		Limit:            10,
		Cookie:           NewPBSCookie(),
		FilterSettings: &FilterSettings{
			Redirect: &SyncFilter{
				Bidders: []string{"appnexus"},
				Mode:    FilterModeExclude,
			},
		},
	}

	result := syncer.ChooseBidders(req)

	// appnexus should be filtered out (redirect type, excluded)
	for _, b := range result.BiddersToSync {
		if b == "appnexus" {
			t.Error("Expected appnexus to be filtered out")
		}
	}
}

func TestChooseBiddersWithFilterInclude(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"rubicon":  {Key: "rubicon", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Only include appnexus for redirect syncs
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"appnexus", "rubicon"},
		Limit:            10,
		Cookie:           NewPBSCookie(),
		FilterSettings: &FilterSettings{
			Redirect: &SyncFilter{
				Bidders: []string{"appnexus"},
				Mode:    FilterModeInclude,
			},
		},
	}

	result := syncer.ChooseBidders(req)

	// Only appnexus should be included
	if len(result.BiddersToSync) != 1 || result.BiddersToSync[0] != "appnexus" {
		t.Errorf("Expected only appnexus, got %v", result.BiddersToSync)
	}
}

func TestBuildSyncURLWithIFrame(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"pubmatic": {
			Key:      "pubmatic",
			Supports: []SyncType{SyncTypeIFrame},
			Default:  SyncTypeIFrame,
			IFrame: &SyncURLTemplate{
				URL:       "https://pubmatic.com/sync?gdpr={{gdpr}}",
				UserMacro: "",
			},
		},
	}

	config := DefaultCookieSyncConfig()
	config.ExternalURL = "https://pbs.example.com"
	syncer := NewSyncer(configs, config)

	privacy := &PrivacyInfo{GDPRApplies: false}
	result, err := syncer.BuildSyncURL("pubmatic", SyncTypeIFrame, privacy)
	if err != nil {
		t.Fatalf("BuildSyncURL failed: %v", err)
	}

	if result.Type != SyncTypeIFrame {
		t.Errorf("Expected iframe type, got %s", result.Type)
	}

	if !contains(result.URL, "gdpr=0") {
		t.Error("Expected gdpr=0 in URL")
	}
}

func TestBuildSyncURLFallbackType(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder": {
			Key:      "bidder",
			Supports: []SyncType{SyncTypeRedirect}, // Only redirect
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com/sync"},
		},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Request iframe, but only redirect is supported - should fallback
	result, err := syncer.BuildSyncURL("bidder", SyncTypeIFrame, nil)
	if err != nil {
		t.Fatalf("BuildSyncURL failed: %v", err)
	}

	// Should fallback to default (redirect)
	if result.Type != SyncTypeRedirect {
		t.Errorf("Expected redirect type (fallback), got %s", result.Type)
	}
}

func TestBuildSyncURLNoTemplate(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder": {
			Key:      "bidder",
			Supports: []SyncType{SyncTypeIFrame},
			Default:  SyncTypeIFrame,
			// No IFrame template set
		},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	_, err := syncer.BuildSyncURL("bidder", SyncTypeIFrame, nil)
	if err == nil {
		t.Error("Expected error for missing template")
	}
}

func TestBuildRedirectURLEmpty(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder": {
			Key:      "bidder",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com?redir={{redirect_url}}"},
		},
	}

	config := DefaultCookieSyncConfig()
	// No external URL set
	syncer := NewSyncer(configs, config)

	result, _ := syncer.BuildSyncURL("bidder", SyncTypeRedirect, nil)

	// redirect_url should be empty (URL encoded)
	if contains(result.URL, "{{redirect_url}}") {
		t.Error("redirect_url macro should be replaced even if empty")
	}
}

func TestGPPSIDString(t *testing.T) {
	privacy := &PrivacyInfo{
		GPPSID: []int{},
	}

	if privacy.GPPSIDString() != "" {
		t.Error("Expected empty string for empty GPPSID")
	}

	privacy.GPPSID = []int{2}
	result := privacy.GPPSIDString()
	if result == "" {
		t.Error("Expected non-empty string for GPPSID")
	}

	privacy.GPPSID = []int{2, 6, 11}
	result = privacy.GPPSIDString()
	if result == "" {
		t.Error("Expected non-empty string for multiple GPPSIDs")
	}
}

func TestReplaceMacrosWithNilPrivacy(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder": {
			Key:      "bidder",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com?gdpr={{gdpr}}&gpp={{gpp}}"},
		},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	result, err := syncer.BuildSyncURL("bidder", SyncTypeRedirect, nil)
	if err != nil {
		t.Fatalf("BuildSyncURL failed: %v", err)
	}

	// Should have gdpr=0 (default)
	if !contains(result.URL, "gdpr=0") {
		t.Error("Expected gdpr=0 with nil privacy")
	}
}

func TestSyncerWithEmptyKey(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder": {
			Key:      "", // Empty key - should use bidder code
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{URL: "https://example.com"},
		},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// With empty key, should use bidder code
	key := syncer.GetSyncerKey("bidder")
	if key != "bidder" {
		t.Errorf("Expected 'bidder' as key, got '%s'", key)
	}
}

func TestGetSyncerKeyUnknown(t *testing.T) {
	configs := map[string]*SyncerConfig{}
	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Unknown bidder returns the bidder code
	key := syncer.GetSyncerKey("unknown")
	if key != "unknown" {
		t.Errorf("Expected 'unknown', got '%s'", key)
	}
}

func TestChooseBiddersNoLimit(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder1": {Key: "bidder1", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
		"bidder2": {Key: "bidder2", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	config.DefaultLimit = 5
	config.MaxLimit = 10
	syncer := NewSyncer(configs, config)

	// Test with 0 limit - should use default
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"bidder1", "bidder2"},
		Limit:            0,
		Cookie:           NewPBSCookie(),
	}

	result := syncer.ChooseBidders(req)
	if len(result.BiddersToSync) != 2 {
		t.Errorf("Expected 2 bidders, got %d", len(result.BiddersToSync))
	}
}

func TestChooseBiddersExceedsMaxLimit(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"bidder1": {Key: "bidder1", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	config.MaxLimit = 5
	syncer := NewSyncer(configs, config)

	// Request limit exceeds max - should be capped
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"bidder1"},
		Limit:            100, // Exceeds max
		Cookie:           NewPBSCookie(),
	}

	result := syncer.ChooseBidders(req)
	// Should still work, just capped
	if len(result.BiddersToSync) != 1 {
		t.Errorf("Expected 1 bidder, got %d", len(result.BiddersToSync))
	}
}

func TestPassesFilterWithIFrameType(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"pubmatic": {Key: "pubmatic", Supports: []SyncType{SyncTypeIFrame}, Default: SyncTypeIFrame, IFrame: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Filter on iframe type
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"pubmatic"},
		Limit:            10,
		Cookie:           NewPBSCookie(),
		FilterSettings: &FilterSettings{
			IFrame: &SyncFilter{
				Bidders: []string{"pubmatic"},
				Mode:    FilterModeExclude,
			},
		},
	}

	result := syncer.ChooseBidders(req)

	// pubmatic uses iframe and is excluded
	if len(result.BiddersToSync) != 0 {
		t.Errorf("Expected 0 bidders (pubmatic excluded), got %d", len(result.BiddersToSync))
	}
}

func TestChooseBiddersUnknownBidder(t *testing.T) {
	configs := map[string]*SyncerConfig{
		"appnexus": {Key: "adnxs", Supports: []SyncType{SyncTypeRedirect}, Default: SyncTypeRedirect, Redirect: &SyncURLTemplate{URL: "https://example.com"}},
	}

	config := DefaultCookieSyncConfig()
	syncer := NewSyncer(configs, config)

	// Request includes unknown bidder
	req := &ChooseBiddersRequest{
		RequestedBidders: []string{"appnexus", "unknownbidder"},
		Limit:            10,
		Cookie:           NewPBSCookie(),
	}

	result := syncer.ChooseBidders(req)

	// Only appnexus should be synced
	if len(result.BiddersToSync) != 1 {
		t.Errorf("Expected 1 bidder, got %d", len(result.BiddersToSync))
	}

	// Unknown should be rejected
	if _, ok := result.RejectedBidders["unknownbidder"]; !ok {
		t.Error("Expected unknownbidder to be rejected")
	}
}

func TestGetUIDExpired(t *testing.T) {
	cookie := NewPBSCookie()

	// Add expired UID
	expired := time.Now().Add(-time.Hour)
	cookie.SetUID("appnexus", "user123", &expired)

	uid, found := cookie.GetUID("appnexus")
	if found || uid != "" {
		t.Error("Expected empty result for expired UID")
	}
}

func TestToJSONError(t *testing.T) {
	// This is hard to trigger with PBSCookie, but we can test the success path
	cookie := NewPBSCookie()
	json := cookie.ToJSON()
	if json == "" {
		t.Error("Expected non-empty JSON")
	}
}

// Mock GDPR enforcer for testing
type mockGDPREnforcer struct {
	denied map[string]bool
}

func (m *mockGDPREnforcer) CanSync(bidder string, consentString string) bool {
	return !m.denied[bidder]
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
