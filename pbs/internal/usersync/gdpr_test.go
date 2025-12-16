package usersync

import (
	"testing"
)

func TestDefaultGDPREnforcer(t *testing.T) {
	vendorIDs := map[string]int{
		"appnexus": 32,
		"rubicon":  52,
		"pubmatic": 76,
	}

	// Test with enforcement enabled
	enforcer := NewDefaultGDPREnforcer(vendorIDs, true)

	if !enforcer.IsEnabled() {
		t.Error("Expected enforcer to be enabled")
	}

	// Test GetVendorID
	id, ok := enforcer.GetVendorID("appnexus")
	if !ok || id != 32 {
		t.Errorf("Expected vendor ID 32 for appnexus, got %d", id)
	}

	_, ok = enforcer.GetVendorID("unknown")
	if ok {
		t.Error("Expected no vendor ID for unknown bidder")
	}
}

func TestGDPREnforcerCanSync(t *testing.T) {
	vendorIDs := DefaultVendorIDs()
	enforcer := NewDefaultGDPREnforcer(vendorIDs, true)

	// Test with no consent string - should use default permission (true)
	if !enforcer.CanSync("appnexus", "") {
		t.Error("Expected CanSync to return true with empty consent")
	}

	// Test with valid-looking consent string
	validConsent := "BONciguONcjGKADACHENAOLS1rAHDAFAAEAASABQAMwAeACEAFw"
	if !enforcer.CanSync("appnexus", validConsent) {
		t.Error("Expected CanSync to return true with valid consent")
	}

	// Test with short consent string
	shortConsent := "BON"
	if !enforcer.CanSync("appnexus", shortConsent) {
		t.Error("Expected CanSync to return default permission with short consent")
	}

	// Test with unknown bidder (no vendor ID)
	if !enforcer.CanSync("unknownbidder", validConsent) {
		t.Error("Expected CanSync to return true for unknown bidder")
	}
}

func TestGDPREnforcerDisabled(t *testing.T) {
	vendorIDs := DefaultVendorIDs()
	enforcer := NewDefaultGDPREnforcer(vendorIDs, false)

	if enforcer.IsEnabled() {
		t.Error("Expected enforcer to be disabled")
	}

	// Should always allow sync when disabled
	if !enforcer.CanSync("appnexus", "") {
		t.Error("Expected CanSync to return true when enforcer disabled")
	}
}

func TestSetDefaultPermission(t *testing.T) {
	vendorIDs := DefaultVendorIDs()
	enforcer := NewDefaultGDPREnforcer(vendorIDs, true)

	// Set default to deny
	enforcer.SetDefaultPermission(false)

	// With no consent and default deny, should return false for unknown bidder
	// (because it falls back to default)
	// Actually the logic returns default when no vendor ID found
	if enforcer.CanSync("unknownbidder", "") {
		t.Error("Expected CanSync to return false with default permission set to false")
	}
}

func TestDefaultVendorIDs(t *testing.T) {
	vendorIDs := DefaultVendorIDs()

	// Check some known vendor IDs
	expectedIDs := map[string]int{
		"appnexus":  32,
		"rubicon":   52,
		"pubmatic":  76,
		"criteo":    91,
		"ix":        10,
		"openx":     69,
		"triplelift": 28,
	}

	for bidder, expectedID := range expectedIDs {
		if id, ok := vendorIDs[bidder]; !ok || id != expectedID {
			t.Errorf("Expected vendor ID %d for %s, got %d", expectedID, bidder, id)
		}
	}
}

func TestCanSyncWithPolicy(t *testing.T) {
	// Test nil policy
	if !CanSyncWithPolicy(nil, nil) {
		t.Error("Expected true with nil policy")
	}

	// Test COPPA blocks sync
	policy := &PrivacyPolicy{COPPA: true}
	if CanSyncWithPolicy(policy, nil) {
		t.Error("Expected false with COPPA policy")
	}

	// Test CCPA opt-out blocks sync
	policy = &PrivacyPolicy{CCPA: true}
	privacy := &PrivacyInfo{USPrivacy: "1YYN"} // Opt-out = Y at position 2
	if CanSyncWithPolicy(policy, privacy) {
		t.Error("Expected false with CCPA opt-out")
	}

	// Test CCPA without opt-out allows sync
	privacy = &PrivacyInfo{USPrivacy: "1YNN"} // Opt-out = N
	if !CanSyncWithPolicy(policy, privacy) {
		t.Error("Expected true with CCPA but no opt-out")
	}

	// Test short US Privacy string
	privacy = &PrivacyInfo{USPrivacy: "1Y"} // Too short
	if !CanSyncWithPolicy(policy, privacy) {
		t.Error("Expected true with short USPrivacy string")
	}

	// Test GDPR only policy (handled by enforcer separately)
	policy = &PrivacyPolicy{GDPR: true}
	if !CanSyncWithPolicy(policy, nil) {
		t.Error("Expected true with GDPR policy (enforcer handles separately)")
	}
}

func TestNullGDPREnforcer(t *testing.T) {
	enforcer := &NullGDPREnforcer{}

	// Should always return true
	if !enforcer.CanSync("appnexus", "") {
		t.Error("NullGDPREnforcer should always return true")
	}

	if !enforcer.CanSync("anybidder", "anyconsent") {
		t.Error("NullGDPREnforcer should always return true")
	}
}

func TestHasBasicConsent(t *testing.T) {
	vendorIDs := DefaultVendorIDs()
	enforcer := NewDefaultGDPREnforcer(vendorIDs, true)

	// Test the hasBasicConsent internal function via CanSync
	// Short consent should return default permission
	result := enforcer.CanSync("appnexus", "short")
	if !result {
		t.Error("Expected default permission for short consent")
	}

	// Valid length consent should be accepted
	longConsent := "BONciguONcjGKADACHENAOLS1rAHDAFAAEAASABQAMwAeACEAFwBONcig"
	result = enforcer.CanSync("appnexus", longConsent)
	if !result {
		t.Error("Expected true for valid-looking consent")
	}
}
