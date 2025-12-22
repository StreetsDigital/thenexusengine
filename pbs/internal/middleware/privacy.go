// Package middleware provides HTTP middleware components
package middleware

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
)

// PrivacyConfig configures the privacy middleware behavior
type PrivacyConfig struct {
	// EnforceGDPR requires valid consent when regs.gdpr=1
	EnforceGDPR bool
	// EnforceCOPPA blocks requests with COPPA=1 (child-directed)
	EnforceCOPPA bool
	// AllowedGDPRPurposes - if empty, just validate consent string exists
	AllowedGDPRPurposes []int
	// StrictMode - if true, reject invalid consent strings; if false, strip PII
	StrictMode bool
}

// DefaultPrivacyConfig returns a sensible default config
func DefaultPrivacyConfig() PrivacyConfig {
	return PrivacyConfig{
		EnforceGDPR:  true,
		EnforceCOPPA: true,
		StrictMode:   true, // P0-4: Default to strict for GDPR compliance
	}
}

// PrivacyMiddleware enforces privacy regulations before auction execution
type PrivacyMiddleware struct {
	config PrivacyConfig
	next   http.Handler
}

// NewPrivacyMiddleware creates a new privacy enforcement middleware
func NewPrivacyMiddleware(config PrivacyConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &PrivacyMiddleware{
			config: config,
			next:   next,
		}
	}
}

// ServeHTTP implements the http.Handler interface
func (m *PrivacyMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only process POST requests to auction endpoint
	if r.Method != http.MethodPost {
		m.next.ServeHTTP(w, r)
		return
	}

	// Read and parse the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.next.ServeHTTP(w, r)
		return
	}
	r.Body.Close()

	var bidRequest openrtb.BidRequest
	if err := json.Unmarshal(body, &bidRequest); err != nil {
		// Let the handler deal with invalid JSON
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		m.next.ServeHTTP(w, r)
		return
	}

	// Check privacy compliance
	violation := m.checkPrivacyCompliance(&bidRequest)
	if violation != nil {
		logger.Log.Warn().
			Str("request_id", bidRequest.ID).
			Str("violation", violation.Reason).
			Str("regulation", violation.Regulation).
			Msg("Privacy compliance violation - blocking request")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":      "Privacy compliance violation",
			"reason":     violation.Reason,
			"regulation": violation.Regulation,
			"nbr":        violation.NoBidReason,
		})
		return
	}

	// Re-create request body for downstream handler
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	m.next.ServeHTTP(w, r)
}

// PrivacyViolation describes a privacy compliance failure
type PrivacyViolation struct {
	Regulation  string // "GDPR", "COPPA", "CCPA"
	Reason      string // Human-readable reason
	NoBidReason int    // OpenRTB no-bid reason code
}

// OpenRTB No-Bid Reason Codes (Section 5.24)
const (
	NoBidUnknown           = 0
	NoBidTechError         = 1
	NoBidInvalidRequest    = 2
	NoBidKnownWebSpider    = 3
	NoBidSuspectedNonHuman = 4
	NoBidCloudDataCenter   = 5
	NoBidUnsupportedDevice = 6
	NoBidBlockedPublisher  = 7
	NoBidUnmatchedUser     = 8
	NoBidDailyReaderCap    = 9
	NoBidDailyDomainCap    = 10
	NoBidAdsNotAllowed     = 11 // Used for privacy violations
)

// checkPrivacyCompliance verifies the request meets privacy requirements
func (m *PrivacyMiddleware) checkPrivacyCompliance(req *openrtb.BidRequest) *PrivacyViolation {
	// Check COPPA compliance
	if m.config.EnforceCOPPA && req.Regs != nil && req.Regs.COPPA == 1 {
		// COPPA requests require special handling - we block by default
		// Production systems might strip identifiers instead
		return &PrivacyViolation{
			Regulation:  "COPPA",
			Reason:      "Child-directed content requires COPPA-compliant handling",
			NoBidReason: NoBidAdsNotAllowed,
		}
	}

	// Check GDPR compliance
	if m.config.EnforceGDPR && m.isGDPRApplicable(req) {
		violation := m.validateGDPRConsent(req)
		if violation != nil {
			return violation
		}
	}

	// Check US Privacy (CCPA) - currently just log, don't block
	if req.Regs != nil && req.Regs.USPrivacy != "" {
		m.logUSPrivacySignal(req.ID, req.Regs.USPrivacy)
	}

	return nil
}

// isGDPRApplicable checks if GDPR applies to this request
func (m *PrivacyMiddleware) isGDPRApplicable(req *openrtb.BidRequest) bool {
	if req.Regs == nil {
		return false
	}
	// GDPR applies if regs.gdpr == 1
	return req.Regs.GDPR != nil && *req.Regs.GDPR == 1
}

// validateGDPRConsent validates the TCF consent string
func (m *PrivacyMiddleware) validateGDPRConsent(req *openrtb.BidRequest) *PrivacyViolation {
	// Get consent string
	consentString := ""
	if req.User != nil {
		consentString = req.User.Consent
	}

	// No consent string when GDPR applies = violation
	if consentString == "" {
		return &PrivacyViolation{
			Regulation:  "GDPR",
			Reason:      "Missing consent string when GDPR applies (regs.gdpr=1)",
			NoBidReason: NoBidAdsNotAllowed,
		}
	}

	// Validate the consent string format (TCF v2)
	if !m.isValidTCFv2String(consentString) {
		return &PrivacyViolation{
			Regulation:  "GDPR",
			Reason:      "Invalid TCF v2 consent string format",
			NoBidReason: NoBidInvalidRequest,
		}
	}

	return nil
}

// isValidTCFv2String performs basic validation of a TCF v2 consent string
// P0-4: This is a lightweight check - full parsing happens in IDR
func (m *PrivacyMiddleware) isValidTCFv2String(consent string) bool {
	if consent == "" {
		return false
	}

	// TCF v2 strings should be base64url encoded
	// Minimum reasonable length for a TCF v2 string is ~20 chars
	if len(consent) < 20 {
		return false
	}

	// Try to decode the string
	decoded, err := base64.RawURLEncoding.DecodeString(consent)
	if err != nil {
		// Try with standard base64 (some implementations use this)
		decoded, err = base64.StdEncoding.DecodeString(consent)
		if err != nil {
			return false
		}
	}

	// Minimum decoded length for TCF v2
	if len(decoded) < 5 {
		return false
	}

	// Extract version from first 6 bits
	version := (decoded[0] >> 2) & 0x3F

	// TCF v2 should have version 2
	// We also accept version 1 for backwards compat, but log a warning
	if version != 2 && version != 1 {
		logger.Log.Debug().
			Int("version", int(version)).
			Msg("Unexpected TCF version in consent string")
		return false
	}

	if version == 1 {
		logger.Log.Warn().Msg("TCF v1 consent string detected - consider upgrading to v2")
	}

	return true
}

// logUSPrivacySignal logs CCPA signals for audit purposes
func (m *PrivacyMiddleware) logUSPrivacySignal(requestID, usPrivacy string) {
	// US Privacy string format: VCC (Version, Explicit Notice, Opt-Out Sale)
	// 1YNY = v1, explicit notice given, user opted out of sale
	if len(usPrivacy) < 4 {
		return
	}

	optOut := usPrivacy[2]
	if optOut == 'Y' {
		logger.Log.Info().
			Str("request_id", requestID).
			Str("us_privacy", usPrivacy).
			Msg("CCPA opt-out signal received")
	}
}
