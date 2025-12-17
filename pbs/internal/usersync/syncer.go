package usersync

import (
	"fmt"
	"net/url"
	"strings"
	"text/template"
)

// Syncer represents a user syncer for a bidder
type Syncer interface {
	// Key returns the unique key for this syncer (usually the bidder code)
	Key() string

	// GetSync returns the sync info for this syncer
	GetSync(syncTypes []SyncType, privacy Privacy) (Sync, error)

	// SupportsType returns true if the syncer supports the given sync type
	SupportsType(syncType SyncType) bool
}

// SyncType represents the type of user sync
type SyncType string

const (
	SyncTypePixel  SyncType = "redirect"
	SyncTypeIFrame SyncType = "iframe"
)

// Sync represents a user sync configuration
type Sync struct {
	URL         string   `json:"url"`
	Type        SyncType `json:"type"`
	SupportCORS bool     `json:"supportCORS,omitempty"`
}

// Privacy contains privacy-related information for sync URL generation
type Privacy struct {
	GDPR        bool
	GDPRConsent string
	USPrivacy   string
	GPP         string
	GPPSID      string
}

// SyncerConfig contains configuration for a syncer
type SyncerConfig struct {
	Key          string
	IFrameURL    string
	RedirectURL  string
	ExternalURL  string
	SupportCORS  bool
	UserMacro    string
}

// DefaultSyncer is the standard syncer implementation
type DefaultSyncer struct {
	key              string
	iframeTemplate   *template.Template
	redirectTemplate *template.Template
	externalURL      string
	supportCORS      bool
}

// NewSyncer creates a new syncer from config
func NewSyncer(config SyncerConfig) (Syncer, error) {
	if config.Key == "" {
		return nil, fmt.Errorf("syncer key is required")
	}

	if config.IFrameURL == "" && config.RedirectURL == "" {
		return nil, fmt.Errorf("syncer must have at least one sync URL")
	}

	syncer := &DefaultSyncer{
		key:         config.Key,
		externalURL: config.ExternalURL,
		supportCORS: config.SupportCORS,
	}

	if config.IFrameURL != "" {
		tmpl, err := template.New("iframe").Parse(config.IFrameURL)
		if err != nil {
			return nil, fmt.Errorf("invalid iframe URL template: %w", err)
		}
		syncer.iframeTemplate = tmpl
	}

	if config.RedirectURL != "" {
		tmpl, err := template.New("redirect").Parse(config.RedirectURL)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect URL template: %w", err)
		}
		syncer.redirectTemplate = tmpl
	}

	return syncer, nil
}

// Key returns the syncer key
func (s *DefaultSyncer) Key() string {
	return s.key
}

// SupportsType returns true if the syncer supports the given type
func (s *DefaultSyncer) SupportsType(syncType SyncType) bool {
	switch syncType {
	case SyncTypeIFrame:
		return s.iframeTemplate != nil
	case SyncTypePixel:
		return s.redirectTemplate != nil
	default:
		return false
	}
}

// GetSync returns the sync info for this syncer
func (s *DefaultSyncer) GetSync(syncTypes []SyncType, privacy Privacy) (Sync, error) {
	for _, syncType := range syncTypes {
		if s.SupportsType(syncType) {
			syncURL, err := s.buildSyncURL(syncType, privacy)
			if err != nil {
				continue
			}
			return Sync{
				URL:         syncURL,
				Type:        syncType,
				SupportCORS: s.supportCORS,
			}, nil
		}
	}
	return Sync{}, fmt.Errorf("no supported sync type found for %s", s.key)
}

// buildSyncURL builds the sync URL from template
func (s *DefaultSyncer) buildSyncURL(syncType SyncType, privacy Privacy) (string, error) {
	var tmpl *template.Template
	switch syncType {
	case SyncTypeIFrame:
		tmpl = s.iframeTemplate
	case SyncTypePixel:
		tmpl = s.redirectTemplate
	default:
		return "", fmt.Errorf("unsupported sync type: %s", syncType)
	}

	if tmpl == nil {
		return "", fmt.Errorf("no template for sync type: %s", syncType)
	}

	// Build the redirect URL (callback to PBS)
	redirectURL := s.buildRedirectURL(privacy)

	// Template data
	data := map[string]string{
		"RedirectURL": url.QueryEscape(redirectURL),
		"GDPR":        boolToString(privacy.GDPR),
		"GDPRConsent": privacy.GDPRConsent,
		"USPrivacy":   privacy.USPrivacy,
		"GPP":         privacy.GPP,
		"GPPSID":      privacy.GPPSID,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// buildRedirectURL builds the PBS callback URL
func (s *DefaultSyncer) buildRedirectURL(privacy Privacy) string {
	base := fmt.Sprintf("%s/setuid?bidder=%s&uid=$UID", s.externalURL, s.key)

	// Add privacy params
	params := url.Values{}
	if privacy.GDPR {
		params.Set("gdpr", "1")
		params.Set("gdpr_consent", privacy.GDPRConsent)
	}
	if privacy.USPrivacy != "" {
		params.Set("us_privacy", privacy.USPrivacy)
	}
	if privacy.GPP != "" {
		params.Set("gpp", privacy.GPP)
		params.Set("gpp_sid", privacy.GPPSID)
	}

	if len(params) > 0 {
		return base + "&" + params.Encode()
	}
	return base
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// SyncerRegistry holds all registered syncers
type SyncerRegistry struct {
	syncers map[string]Syncer
}

// NewSyncerRegistry creates a new syncer registry
func NewSyncerRegistry() *SyncerRegistry {
	return &SyncerRegistry{
		syncers: make(map[string]Syncer),
	}
}

// Register adds a syncer to the registry
func (r *SyncerRegistry) Register(syncer Syncer) error {
	if _, exists := r.syncers[syncer.Key()]; exists {
		return fmt.Errorf("syncer already registered: %s", syncer.Key())
	}
	r.syncers[syncer.Key()] = syncer
	return nil
}

// Get returns a syncer by key
func (r *SyncerRegistry) Get(key string) (Syncer, bool) {
	syncer, ok := r.syncers[key]
	return syncer, ok
}

// GetAll returns all registered syncers
func (r *SyncerRegistry) GetAll() map[string]Syncer {
	result := make(map[string]Syncer, len(r.syncers))
	for k, v := range r.syncers {
		result[k] = v
	}
	return result
}

// Keys returns all registered syncer keys
func (r *SyncerRegistry) Keys() []string {
	keys := make([]string, 0, len(r.syncers))
	for k := range r.syncers {
		keys = append(keys, k)
	}
	return keys
}

// DefaultSyncerRegistry is the global syncer registry
var DefaultSyncerRegistry = NewSyncerRegistry()
