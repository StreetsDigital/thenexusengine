package usersync

// DefaultGDPREnforcer provides basic GDPR enforcement
// For full TCF consent parsing, a dedicated TCF library should be used
type DefaultGDPREnforcer struct {
	// vendorIDs maps bidder codes to their TCF Global Vendor List IDs
	vendorIDs map[string]int
	// enabled indicates if GDPR enforcement is enabled
	enabled bool
	// defaultPermission is the default when consent can't be determined
	defaultPermission bool
}

// NewDefaultGDPREnforcer creates a new GDPR enforcer
func NewDefaultGDPREnforcer(vendorIDs map[string]int, enabled bool) *DefaultGDPREnforcer {
	return &DefaultGDPREnforcer{
		vendorIDs:         vendorIDs,
		enabled:           enabled,
		defaultPermission: true, // Default to allowing sync when consent unclear
	}
}

// CanSync checks if a bidder can perform user sync
// This is a simplified implementation - for production use, implement full TCF consent parsing
func (e *DefaultGDPREnforcer) CanSync(bidder string, consentString string) bool {
	if !e.enabled {
		return true
	}

	// If no consent string provided, use default permission
	if consentString == "" {
		return e.defaultPermission
	}

	// Check if bidder has a vendor ID
	vendorID, hasVendorID := e.vendorIDs[bidder]
	if !hasVendorID || vendorID == 0 {
		// Bidder not in vendor list - allow by default
		return e.defaultPermission
	}

	// For a full implementation, parse the TCF consent string and check:
	// 1. If the vendor has consent for purpose 1 (storage/access)
	// 2. If the vendor is in the allowed vendors list
	// 3. If legitimate interest applies
	//
	// For now, we do a simplified check - if consent string exists, we assume it's valid
	// In production, use a TCF library like:
	// - github.com/prebid/go-gdpr
	// - github.com/LiveRamp/iabconsent

	return e.hasBasicConsent(consentString, vendorID)
}

// hasBasicConsent does a simplified TCF v2 consent check
// This is NOT a full implementation - use a proper TCF library for production
func (e *DefaultGDPREnforcer) hasBasicConsent(consentString string, vendorID int) bool {
	// TCF v2 consent strings start with a version identifier
	// A proper implementation would:
	// 1. Decode the base64url consent string
	// 2. Parse the bit field according to TCF v2 spec
	// 3. Check vendor consent and purpose consent

	// For this simplified version, we check basic validity
	if len(consentString) < 10 {
		return e.defaultPermission
	}

	// If the consent string looks valid (proper length for TCF v2), allow sync
	// This is a placeholder - implement proper TCF parsing for production
	return true
}

// SetDefaultPermission sets the default permission when consent is unclear
func (e *DefaultGDPREnforcer) SetDefaultPermission(allow bool) {
	e.defaultPermission = allow
}

// IsEnabled returns whether GDPR enforcement is enabled
func (e *DefaultGDPREnforcer) IsEnabled() bool {
	return e.enabled
}

// GetVendorID returns the TCF vendor ID for a bidder
func (e *DefaultGDPREnforcer) GetVendorID(bidder string) (int, bool) {
	id, ok := e.vendorIDs[bidder]
	return id, ok
}

// DefaultVendorIDs returns the default vendor ID mapping for common bidders
func DefaultVendorIDs() map[string]int {
	return map[string]int{
		"33across":       58,
		"adform":         50,
		"appnexus":       32,
		"beachfront":     0,  // No GVL ID
		"conversant":     24,
		"criteo":         91,
		"gumgum":         61,
		"improvedigital": 253,
		"ix":             10,
		"medianet":       142,
		"openx":          69,
		"outbrain":       164,
		"pubmatic":       76,
		"rubicon":        52,
		"sharethrough":   80,
		"smartadserver":  45,
		"sovrn":          13,
		"spotx":          165,
		"taboola":        42,
		"teads":          132,
		"triplelift":     28,
		"unruly":         36,
	}
}

// PrivacyPolicy represents the privacy policy for a sync request
type PrivacyPolicy struct {
	// GDPR indicates if GDPR applies
	GDPR bool
	// CCPA indicates if CCPA applies
	CCPA bool
	// COPPA indicates if COPPA applies (children's privacy)
	COPPA bool
}

// CanSyncWithPolicy checks if syncing is allowed given privacy policies
func CanSyncWithPolicy(policy *PrivacyPolicy, privacy *PrivacyInfo) bool {
	if policy == nil {
		return true
	}

	// COPPA always blocks sync
	if policy.COPPA {
		return false
	}

	// CCPA: Check for opt-out signal
	if policy.CCPA && privacy != nil && privacy.USPrivacy != "" {
		// USPrivacy format: 1YNN
		// Position 3 (index 2) is opt-out sale: Y = opted out, N = not opted out
		if len(privacy.USPrivacy) >= 3 && privacy.USPrivacy[2] == 'Y' {
			return false
		}
	}

	// GDPR: Handled by GDPREnforcer separately

	return true
}

// NullGDPREnforcer always allows syncing (for when GDPR is disabled)
type NullGDPREnforcer struct{}

// CanSync always returns true
func (e *NullGDPREnforcer) CanSync(bidder string, consentString string) bool {
	return true
}
