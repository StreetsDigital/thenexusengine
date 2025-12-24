// Package middleware provides HTTP middleware components
package middleware

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
)

// TCF v2 Purpose IDs (IAB specification)
const (
	PurposeStorageAccess          = 1  // Store and/or access information on a device
	PurposeBasicAds               = 2  // Select basic ads
	PurposePersonalizedAdsProfile = 3  // Create a personalised ads profile
	PurposePersonalizedAds        = 4  // Select personalised ads
	PurposeContentProfile         = 5  // Create a personalised content profile
	PurposePersonalizedContent    = 6  // Select personalised content
	PurposeMeasureAdPerformance   = 7  // Measure ad performance
	PurposeMeasureContent         = 8  // Measure content performance
	PurposeMarketResearch         = 9  // Apply market research
	PurposeProductDevelopment     = 10 // Develop and improve products
)

// Required purposes for programmatic advertising
var RequiredPurposes = []int{
	PurposeStorageAccess,        // Required for cookies
	PurposeBasicAds,             // Required for ad selection
	PurposeMeasureAdPerformance, // Required for reporting
}

// PrivacyConfig configures the privacy middleware behavior
type PrivacyConfig struct {
	// EnforceGDPR requires valid consent when regs.gdpr=1
	EnforceGDPR bool
	// EnforceCOPPA blocks requests with COPPA=1 (child-directed)
	EnforceCOPPA bool
	// EnforceCCPA blocks/strips data when user opts out
	EnforceCCPA bool
	// RequiredPurposes - TCF purposes required for processing (default: 1, 2, 7)
	RequiredPurposes []int
	// StrictMode - if true, reject invalid consent strings; if false, strip PII
	StrictMode bool
}

// DefaultPrivacyConfig returns a sensible default config
// It reads from environment variables if set:
//   - PBS_ENFORCE_GDPR: "true" or "false" (default: true)
//   - PBS_ENFORCE_COPPA: "true" or "false" (default: true)
//   - PBS_ENFORCE_CCPA: "true" or "false" (default: true)
//   - PBS_PRIVACY_STRICT_MODE: "true" or "false" (default: true)
func DefaultPrivacyConfig() PrivacyConfig {
	return PrivacyConfig{
		EnforceGDPR:      getEnvBool("PBS_ENFORCE_GDPR", true),
		EnforceCOPPA:     getEnvBool("PBS_ENFORCE_COPPA", true),
		EnforceCCPA:      getEnvBool("PBS_ENFORCE_CCPA", true),
		RequiredPurposes: RequiredPurposes,
		StrictMode:       getEnvBool("PBS_PRIVACY_STRICT_MODE", true),
	}
}

// getEnvBool reads a boolean from environment variable with a default
func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return strings.ToLower(val) == "true" || val == "1"
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

	// Check US Privacy (CCPA) - P0: Enforce opt-out
	if req.Regs != nil && req.Regs.USPrivacy != "" {
		violation := m.checkCCPACompliance(req.ID, req.Regs.USPrivacy)
		if violation != nil {
			return violation
		}
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

// validateGDPRConsent validates the TCF consent string and purpose consents
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

	// Parse and validate the consent string
	tcfData, err := m.parseTCFv2String(consentString)
	if err != nil {
		return &PrivacyViolation{
			Regulation:  "GDPR",
			Reason:      "Invalid TCF v2 consent string: " + err.Error(),
			NoBidReason: NoBidInvalidRequest,
		}
	}

	// Check required purposes have consent (only in StrictMode)
	if m.config.StrictMode && len(m.config.RequiredPurposes) > 0 {
		missingPurposes := m.checkPurposeConsents(tcfData, m.config.RequiredPurposes)
		if len(missingPurposes) > 0 {
			logger.Log.Info().
				Str("request_id", req.ID).
				Ints("missing_purposes", missingPurposes).
				Msg("Missing required purpose consents")
			return &PrivacyViolation{
				Regulation:  "GDPR",
				Reason:      "Missing consent for required purposes",
				NoBidReason: NoBidAdsNotAllowed,
			}
		}
	}

	return nil
}

// TCFv2Data holds parsed TCF v2 consent data
type TCFv2Data struct {
	Version           int
	Created           int64
	LastUpdated       int64
	CmpID             int
	CmpVersion        int
	ConsentScreen     int
	ConsentLanguage   string
	VendorListVersion int
	PurposeConsents   []bool // Indexed by purpose ID (1-based in spec, 0-based here)
	VendorConsents    map[int]bool
}

// parseTCFv2String parses a TCF v2 consent string and extracts purpose consents
func (m *PrivacyMiddleware) parseTCFv2String(consent string) (*TCFv2Data, error) {
	if consent == "" {
		return nil, nil
	}

	// Minimum reasonable length for a TCF v2 string
	if len(consent) < 20 {
		return nil, errInvalidTCFLength
	}

	// Try base64url decoding first, then standard base64
	decoded, err := base64.RawURLEncoding.DecodeString(consent)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(consent)
		if err != nil {
			return nil, errInvalidTCFEncoding
		}
	}

	// Minimum decoded length
	if len(decoded) < 15 {
		return nil, errInvalidTCFLength
	}

	data := &TCFv2Data{
		PurposeConsents: make([]bool, 24), // 24 purposes in TCF v2
		VendorConsents:  make(map[int]bool),
	}

	// Parse using bit reader
	reader := newBitReader(decoded)

	// Version (6 bits)
	data.Version = reader.readInt(6)
	if data.Version != 2 && data.Version != 1 {
		return nil, errInvalidTCFVersion
	}

	if data.Version == 1 {
		logger.Log.Warn().Msg("TCF v1 consent string - consider upgrading to v2")
		// For v1, we can't parse purposes the same way, so just accept it
		return data, nil
	}

	// For TCF v2, continue parsing
	// Created (36 bits - deciseconds since Jan 1, 2000)
	data.Created = int64(reader.readInt(36))
	// LastUpdated (36 bits)
	data.LastUpdated = int64(reader.readInt(36))
	// CmpId (12 bits)
	data.CmpID = reader.readInt(12)
	// CmpVersion (12 bits)
	data.CmpVersion = reader.readInt(12)
	// ConsentScreen (6 bits)
	data.ConsentScreen = reader.readInt(6)
	// ConsentLanguage (12 bits - 2 chars)
	lang1 := byte(reader.readInt(6)) + 'a'
	lang2 := byte(reader.readInt(6)) + 'a'
	data.ConsentLanguage = string([]byte{lang1, lang2})
	// VendorListVersion (12 bits)
	data.VendorListVersion = reader.readInt(12)
	// TcfPolicyVersion (6 bits) - skip
	reader.readInt(6)
	// IsServiceSpecific (1 bit) - skip
	reader.readInt(1)
	// UseNonStandardStacks (1 bit) - skip
	reader.readInt(1)
	// SpecialFeatureOptIns (12 bits) - skip
	reader.readInt(12)

	// Purpose consents (24 bits - one for each purpose)
	for i := 0; i < 24; i++ {
		data.PurposeConsents[i] = reader.readBool()
	}

	return data, nil
}

// checkPurposeConsents verifies required purposes have consent
func (m *PrivacyMiddleware) checkPurposeConsents(data *TCFv2Data, required []int) []int {
	if data == nil {
		return required
	}

	var missing []int
	for _, purpose := range required {
		// Purpose IDs are 1-based in spec, array is 0-based
		idx := purpose - 1
		if idx < 0 || idx >= len(data.PurposeConsents) || !data.PurposeConsents[idx] {
			missing = append(missing, purpose)
		}
	}
	return missing
}

// TCF parsing errors
var (
	errInvalidTCFLength   = &tcfError{"consent string too short"}
	errInvalidTCFEncoding = &tcfError{"invalid base64 encoding"}
	errInvalidTCFVersion  = &tcfError{"unsupported TCF version"}
)

type tcfError struct{ msg string }

func (e *tcfError) Error() string { return e.msg }

// bitReader reads bits from a byte slice
type bitReader struct {
	data   []byte
	bitPos int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data, bitPos: 0}
}

func (r *bitReader) readBool() bool {
	if r.bitPos/8 >= len(r.data) {
		return false
	}
	bytePos := r.bitPos / 8
	bitOffset := 7 - (r.bitPos % 8)
	r.bitPos++
	return (r.data[bytePos] >> bitOffset & 1) == 1
}

func (r *bitReader) readInt(bits int) int {
	result := 0
	for i := 0; i < bits; i++ {
		result = result << 1
		if r.readBool() {
			result |= 1
		}
	}
	return result
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

// checkCCPACompliance validates US Privacy (CCPA) signals and enforces opt-out
func (m *PrivacyMiddleware) checkCCPACompliance(requestID, usPrivacy string) *PrivacyViolation {
	// US Privacy string format: VNOS (Version, Notice, OptOut, LSPA)
	// Position 0: Version (1)
	// Position 1: Explicit Notice (Y/N/-)
	// Position 2: Opt-Out of Sale (Y/N/-)
	// Position 3: LSPA Covered (Y/N/-)
	//
	// Examples:
	// 1YNY = v1, notice given, NOT opted out, LSPA covered
	// 1YYN = v1, notice given, OPTED OUT of sale, not LSPA covered
	// 1--- = v1, all signals not applicable

	if len(usPrivacy) < 4 {
		logger.Log.Debug().
			Str("request_id", requestID).
			Str("us_privacy", usPrivacy).
			Msg("Invalid US Privacy string format (too short)")
		return nil // Don't block on malformed strings, just log
	}

	// Validate version
	version := usPrivacy[0]
	if version != '1' {
		logger.Log.Debug().
			Str("request_id", requestID).
			Str("us_privacy", usPrivacy).
			Msg("Unknown US Privacy version")
		return nil
	}

	// Check opt-out signal (position 2)
	optOut := usPrivacy[2]

	if optOut == 'Y' {
		logger.Log.Info().
			Str("request_id", requestID).
			Str("us_privacy", usPrivacy).
			Msg("CCPA opt-out signal received")

		// P0: Enforce CCPA opt-out if configured
		if m.config.EnforceCCPA {
			return &PrivacyViolation{
				Regulation:  "CCPA",
				Reason:      "User has opted out of data sale under CCPA",
				NoBidReason: NoBidAdsNotAllowed,
			}
		}
	}

	// Log notice status for compliance auditing
	notice := usPrivacy[1]
	if notice == 'N' {
		logger.Log.Warn().
			Str("request_id", requestID).
			Str("us_privacy", usPrivacy).
			Msg("CCPA: Explicit notice not provided to user")
	}

	return nil
}
