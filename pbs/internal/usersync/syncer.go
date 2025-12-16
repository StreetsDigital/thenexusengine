package usersync

import (
	"fmt"
	"net/url"
	"strings"
)

// Syncer handles user sync URL generation and bidder selection
type Syncer struct {
	// configs maps bidder codes to their sync configurations
	configs map[string]*SyncerConfig
	// bidderToSyncerKey maps bidder codes to syncer keys (for aliases like appnexus -> adnxs)
	bidderToSyncerKey map[string]string
	// syncerKeyToBidder maps syncer keys back to primary bidder codes
	syncerKeyToBidder map[string]string
	// config is the cookie sync configuration
	config CookieSyncConfig
}

// NewSyncer creates a new Syncer with the given configurations
func NewSyncer(configs map[string]*SyncerConfig, config CookieSyncConfig) *Syncer {
	bidderToSyncerKey := make(map[string]string)
	syncerKeyToBidder := make(map[string]string)

	for bidder, cfg := range configs {
		key := cfg.Key
		if key == "" {
			key = bidder
		}
		bidderToSyncerKey[bidder] = key
		// Only set if not already set (first bidder wins for a syncer key)
		if _, exists := syncerKeyToBidder[key]; !exists {
			syncerKeyToBidder[key] = bidder
		}
	}

	return &Syncer{
		configs:           configs,
		bidderToSyncerKey: bidderToSyncerKey,
		syncerKeyToBidder: syncerKeyToBidder,
		config:            config,
	}
}

// GetSyncerKey returns the syncer key for a bidder
func (s *Syncer) GetSyncerKey(bidder string) string {
	if key, ok := s.bidderToSyncerKey[bidder]; ok {
		return key
	}
	return bidder
}

// GetBidderForSyncerKey returns the primary bidder code for a syncer key
func (s *Syncer) GetBidderForSyncerKey(syncerKey string) string {
	if bidder, ok := s.syncerKeyToBidder[syncerKey]; ok {
		return bidder
	}
	return syncerKey
}

// HasSyncer checks if a bidder has sync configuration
func (s *Syncer) HasSyncer(bidder string) bool {
	_, ok := s.configs[bidder]
	return ok
}

// GetSyncerConfig returns the sync configuration for a bidder
func (s *Syncer) GetSyncerConfig(bidder string) (*SyncerConfig, bool) {
	cfg, ok := s.configs[bidder]
	return cfg, ok
}

// SupportedBidders returns all bidders with sync configuration
func (s *Syncer) SupportedBidders() []string {
	bidders := make([]string, 0, len(s.configs))
	for bidder := range s.configs {
		bidders = append(bidders, bidder)
	}
	return bidders
}

// SyncResult contains the result of building a sync URL
type SyncResult struct {
	BidderCode  string
	SyncerKey   string
	URL         string
	Type        SyncType
	SupportCORS bool
	Error       error
}

// BuildSyncURL builds a sync URL for a bidder with privacy macros replaced
func (s *Syncer) BuildSyncURL(bidder string, syncType SyncType, privacy *PrivacyInfo) (*SyncResult, error) {
	cfg, ok := s.configs[bidder]
	if !ok {
		return nil, fmt.Errorf("no syncer config for bidder: %s", bidder)
	}

	result := &SyncResult{
		BidderCode:  bidder,
		SyncerKey:   s.GetSyncerKey(bidder),
		SupportCORS: cfg.SupportCORS,
	}

	// Choose sync type
	chosenType := syncType
	if chosenType == "" {
		chosenType = cfg.Default
	}

	// Validate sync type is supported
	if !s.isSyncTypeSupported(cfg, chosenType) {
		// Fall back to default if requested type not supported
		chosenType = cfg.Default
	}

	result.Type = chosenType

	// Get the appropriate template
	var template *SyncURLTemplate
	switch chosenType {
	case SyncTypeIFrame:
		template = cfg.IFrame
	case SyncTypeRedirect:
		template = cfg.Redirect
	}

	if template == nil || template.URL == "" {
		return nil, fmt.Errorf("no sync URL template for bidder %s type %s", bidder, chosenType)
	}

	// Build the URL with macro replacement
	result.URL = s.replaceMacros(template.URL, privacy)

	return result, nil
}

// isSyncTypeSupported checks if a sync type is supported by the config
func (s *Syncer) isSyncTypeSupported(cfg *SyncerConfig, syncType SyncType) bool {
	for _, supported := range cfg.Supports {
		if supported == syncType {
			return true
		}
	}
	return false
}

// replaceMacros replaces privacy macros in a URL template
func (s *Syncer) replaceMacros(urlTemplate string, privacy *PrivacyInfo) string {
	if privacy == nil {
		privacy = &PrivacyInfo{}
	}

	result := urlTemplate

	// Build redirect URL for the setuid endpoint
	redirectURL := s.buildRedirectURL()

	// Replace macros
	replacements := map[string]string{
		"{{gdpr}}":         privacy.GDPRSignal(),
		"{{gdpr_consent}}": url.QueryEscape(privacy.GDPRConsent),
		"{{us_privacy}}":   url.QueryEscape(privacy.USPrivacy),
		"{{gpp}}":          url.QueryEscape(privacy.GPP),
		"{{gpp_sid}}":      privacy.GPPSIDString(),
		"{{redirect_url}}": url.QueryEscape(redirectURL),
	}

	for macro, value := range replacements {
		result = strings.ReplaceAll(result, macro, value)
	}

	return result
}

// buildRedirectURL builds the redirect URL back to setuid endpoint
func (s *Syncer) buildRedirectURL() string {
	if s.config.ExternalURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/setuid", strings.TrimSuffix(s.config.ExternalURL, "/"))
}

// ChooseBiddersRequest contains parameters for choosing bidders to sync
type ChooseBiddersRequest struct {
	// RequestedBidders is the list of bidders requested by the client
	RequestedBidders []string
	// Limit is the max number of syncs to return
	Limit int
	// Cookie is the existing UID cookie
	Cookie *PBSCookie
	// Privacy contains privacy information
	Privacy *PrivacyInfo
	// FilterSettings contains sync type filters
	FilterSettings *FilterSettings
	// CooperativeSync enables cooperative syncing
	CooperativeSync bool
	// GDPREnforcer is the GDPR consent checker (optional)
	GDPREnforcer GDPREnforcer
}

// ChooseBiddersResult contains the result of choosing bidders
type ChooseBiddersResult struct {
	// BiddersToSync is the list of bidders that need syncing
	BiddersToSync []string
	// RejectedBidders maps rejected bidders to reasons
	RejectedBidders map[string]string
}

// ChooseBidders selects bidders that need user sync
func (s *Syncer) ChooseBidders(req *ChooseBiddersRequest) *ChooseBiddersResult {
	result := &ChooseBiddersResult{
		BiddersToSync:   make([]string, 0),
		RejectedBidders: make(map[string]string),
	}

	// Determine which bidders to consider
	candidateBidders := req.RequestedBidders
	if len(candidateBidders) == 0 {
		// No specific bidders requested - use all supported bidders if cooperative sync
		if req.CooperativeSync {
			candidateBidders = s.SupportedBidders()
		}
	}

	// Apply limit
	limit := req.Limit
	if limit <= 0 {
		limit = s.config.DefaultLimit
	}
	if limit > s.config.MaxLimit {
		limit = s.config.MaxLimit
	}

	for _, bidder := range candidateBidders {
		// Stop if we've hit the limit
		if len(result.BiddersToSync) >= limit {
			break
		}

		// Check if bidder has sync configuration
		if !s.HasSyncer(bidder) {
			result.RejectedBidders[bidder] = "no sync configuration"
			continue
		}

		// Check if bidder already has a valid UID
		syncerKey := s.GetSyncerKey(bidder)
		if req.Cookie != nil && req.Cookie.HasUID(syncerKey) {
			result.RejectedBidders[bidder] = "already synced"
			continue
		}

		// Check GDPR consent if enforcer provided
		if req.GDPREnforcer != nil && req.Privacy != nil && req.Privacy.GDPRApplies {
			if !req.GDPREnforcer.CanSync(bidder, req.Privacy.GDPRConsent) {
				result.RejectedBidders[bidder] = "GDPR consent denied"
				continue
			}
		}

		// Check filter settings
		if req.FilterSettings != nil {
			cfg, _ := s.GetSyncerConfig(bidder)
			if cfg != nil && !s.passesFilter(bidder, cfg.Default, req.FilterSettings) {
				result.RejectedBidders[bidder] = "filtered by settings"
				continue
			}
		}

		result.BiddersToSync = append(result.BiddersToSync, bidder)
	}

	return result
}

// passesFilter checks if a bidder passes the filter settings
func (s *Syncer) passesFilter(bidder string, syncType SyncType, filter *FilterSettings) bool {
	if filter == nil {
		return true
	}

	var typeFilter *SyncFilter
	switch syncType {
	case SyncTypeIFrame:
		typeFilter = filter.IFrame
	case SyncTypeRedirect:
		typeFilter = filter.Redirect
	}

	if typeFilter == nil {
		return true
	}

	// Check if bidder is in the filter list
	inList := false
	for _, b := range typeFilter.Bidders {
		if b == bidder {
			inList = true
			break
		}
	}

	switch typeFilter.Mode {
	case FilterModeInclude:
		return inList
	case FilterModeExclude:
		return !inList
	default:
		return true
	}
}

// BuildSyncs builds sync URLs for a list of bidders
func (s *Syncer) BuildSyncs(bidders []string, syncType SyncType, privacy *PrivacyInfo) []*SyncResult {
	results := make([]*SyncResult, 0, len(bidders))

	for _, bidder := range bidders {
		result, err := s.BuildSyncURL(bidder, syncType, privacy)
		if err != nil {
			results = append(results, &SyncResult{
				BidderCode: bidder,
				SyncerKey:  s.GetSyncerKey(bidder),
				Error:      err,
			})
		} else {
			results = append(results, result)
		}
	}

	return results
}

// GDPREnforcer interface for GDPR consent checking
type GDPREnforcer interface {
	// CanSync checks if a bidder can perform user sync given GDPR consent
	CanSync(bidder string, consentString string) bool
}
