package usersync

import "sync"

// BidderSyncerConfigs contains sync configurations for known bidders.
// These are the standard Prebid Server syncer configurations.
// The {{.RedirectURL}} placeholder will be replaced with the PBS callback URL.
// Privacy macros: {{.GDPR}}, {{.GDPRConsent}}, {{.USPrivacy}}, {{.GPP}}, {{.GPPSID}}
var BidderSyncerConfigs = map[string]SyncerConfig{
	"appnexus": {
		Key:         "appnexus",
		RedirectURL: "https://ib.adnxs.com/getuid?{{.RedirectURL}}",
		SupportCORS: false,
	},
	"rubicon": {
		Key:         "rubicon",
		RedirectURL: "https://pixel.rubiconproject.com/exchange/sync.php?p=prebid&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redirect={{.RedirectURL}}",
		IFrameURL:   "https://eus.rubiconproject.com/usync.html?p=prebid&endpoint={{.RedirectURL}}&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}",
		SupportCORS: false,
	},
	"pubmatic": {
		Key:         "pubmatic",
		RedirectURL: "https://image8.pubmatic.com/AdServer/ImgSync?p=159706&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&pu={{.RedirectURL}}",
		IFrameURL:   "https://ads.pubmatic.com/AdServer/js/user_sync.html?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&predirect={{.RedirectURL}}",
		SupportCORS: false,
	},
	"openx": {
		Key:         "openx",
		RedirectURL: "https://rtb.openx.net/sync/prebid?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&r={{.RedirectURL}}",
		SupportCORS: false,
	},
	"ix": {
		Key:         "ix",
		RedirectURL: "https://ssum.casalemedia.com/usermatchredir?s=194962&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&cb={{.RedirectURL}}",
		SupportCORS: false,
	},
	"criteo": {
		Key:         "criteo",
		RedirectURL: "https://bidder.criteo.com/cdb?profileId=230&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&cb={{.RedirectURL}}",
		IFrameURL:   "https://static.criteo.net/js/ld/publishertag.prebid.js?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&url={{.RedirectURL}}",
		SupportCORS: true,
	},
	"triplelift": {
		Key:         "triplelift",
		RedirectURL: "https://eb2.3lift.com/getuid?gdpr={{.GDPR}}&cmp_cs={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redir={{.RedirectURL}}",
		SupportCORS: false,
	},
	"sovrn": {
		Key:         "sovrn",
		RedirectURL: "https://ap.lijit.com/pixel?redir={{.RedirectURL}}",
		IFrameURL:   "https://ap.lijit.com/sync?src=prebid&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redirect={{.RedirectURL}}",
		SupportCORS: false,
	},
	"sharethrough": {
		Key:         "sharethrough",
		RedirectURL: "https://match.sharethrough.com/FGMrCMMc/v1?redirectUri={{.RedirectURL}}",
		SupportCORS: false,
	},
	"gumgum": {
		Key:         "gumgum",
		RedirectURL: "https://rtb.gumgum.com/usync/prbds2s?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&r={{.RedirectURL}}",
		SupportCORS: false,
	},
	"33across": {
		Key:         "33across",
		RedirectURL: "https://ssc.33across.com/ps/?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redir={{.RedirectURL}}",
		IFrameURL:   "https://ic.tynt.com/r/d?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&m=xch&rt=html&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&ru={{.RedirectURL}}",
		SupportCORS: false,
	},
	"conversant": {
		Key:         "conversant",
		RedirectURL: "https://prebid-match.dotomi.com/match/bounce/current?version=1&networkId=72563&rurl={{.RedirectURL}}",
		SupportCORS: false,
	},
	"medianet": {
		Key:         "medianet",
		RedirectURL: "https://prebid.media.net/rtb/prebid/userSync?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redirect={{.RedirectURL}}",
		SupportCORS: false,
	},
	"adform": {
		Key:         "adform",
		RedirectURL: "https://cm.adform.net/cookie?redirect_url={{.RedirectURL}}",
		SupportCORS: false,
	},
	"smartadserver": {
		Key:         "smartadserver",
		RedirectURL: "https://ssbsync.smartadserver.com/getuid?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&url={{.RedirectURL}}",
		SupportCORS: false,
	},
	"improvedigital": {
		Key:         "improvedigital",
		RedirectURL: "https://ad.360yield.com/server_match?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&r={{.RedirectURL}}",
		SupportCORS: false,
	},
	"beachfront": {
		Key:         "beachfront",
		RedirectURL: "https://sync.bfmio.com/sync_s2s?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&url={{.RedirectURL}}",
		SupportCORS: false,
	},
	"taboola": {
		Key:         "taboola",
		RedirectURL: "https://trc.taboola.com/sg/prebidserver/1/rtb-h/?taboola_hm=$UID&redirect={{.RedirectURL}}",
		SupportCORS: false,
	},
	"outbrain": {
		Key:         "outbrain",
		RedirectURL: "https://prebidtest.zemanta.com/usersync/prebidserver?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&r={{.RedirectURL}}",
		SupportCORS: false,
	},
	"unruly": {
		Key:         "unruly",
		RedirectURL: "https://sync.1rx.io/usersync/pbs?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&rurl={{.RedirectURL}}",
		SupportCORS: false,
	},
	"teads": {
		Key:         "teads",
		RedirectURL: "https://sync.teads.tv/page/prebid-server-sync?gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redirect={{.RedirectURL}}",
		SupportCORS: false,
	},
	"spotx": {
		Key:         "spotx",
		RedirectURL: "https://sync.search.spotxchange.com/partner?adv_id=16569&gdpr={{.GDPR}}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}&redir={{.RedirectURL}}",
		SupportCORS: false,
	},
}

// InitDefaultSyncers initializes syncers for all configured bidders
func InitDefaultSyncers(externalURL string) (*SyncerRegistry, error) {
	registry := NewSyncerRegistry()

	for bidder, config := range BidderSyncerConfigs {
		config.ExternalURL = externalURL
		syncer, err := NewSyncer(config)
		if err != nil {
			// Log warning but continue - not all syncers are required
			continue
		}
		if err := registry.Register(syncer); err != nil {
			continue
		}
		_ = bidder // Silence unused variable warning
	}

	return registry, nil
}

// GetSyncerConfig returns the syncer config for a bidder
func GetSyncerConfig(bidder string) (SyncerConfig, bool) {
	config, ok := BidderSyncerConfigs[bidder]
	return config, ok
}

// AddSyncerConfig adds a custom syncer config
func AddSyncerConfig(bidder string, config SyncerConfig) {
	BidderSyncerConfigs[bidder] = config
}

// DynamicSyncerRegistry extends SyncerRegistry with dynamic syncer support
// for custom ORTB bidder integrations
type DynamicSyncerRegistry struct {
	*SyncerRegistry
	mu          sync.RWMutex
	externalURL string
}

// NewDynamicSyncerRegistry creates a new dynamic syncer registry
func NewDynamicSyncerRegistry(externalURL string) (*DynamicSyncerRegistry, error) {
	// Initialize with default static syncers
	staticRegistry, err := InitDefaultSyncers(externalURL)
	if err != nil {
		return nil, err
	}

	return &DynamicSyncerRegistry{
		SyncerRegistry: staticRegistry,
		externalURL:    externalURL,
	}, nil
}

// RegisterDynamicSyncer registers a syncer for a custom ORTB bidder
// This is thread-safe and can be called at runtime when new bidders are loaded from Redis
func (r *DynamicSyncerRegistry) RegisterDynamicSyncer(bidderCode string, config DynamicSyncerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !config.Enabled {
		return nil
	}

	if config.IFrameURL == "" && config.RedirectURL == "" {
		return nil // No sync URLs configured, skip silently
	}

	syncerConfig := SyncerConfig{
		Key:         bidderCode,
		ExternalURL: r.externalURL,
		IFrameURL:   config.IFrameURL,
		RedirectURL: config.RedirectURL,
		SupportCORS: config.SupportCORS,
		UserMacro:   config.UserMacro,
	}

	syncer, err := NewSyncer(syncerConfig)
	if err != nil {
		return err
	}

	// Remove existing syncer if present (for updates)
	delete(r.syncers, bidderCode)

	return r.Register(syncer)
}

// UnregisterDynamicSyncer removes a dynamic syncer
func (r *DynamicSyncerRegistry) UnregisterDynamicSyncer(bidderCode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.syncers, bidderCode)
}

// UpdateDynamicSyncer updates an existing dynamic syncer
func (r *DynamicSyncerRegistry) UpdateDynamicSyncer(bidderCode string, config DynamicSyncerConfig) error {
	return r.RegisterDynamicSyncer(bidderCode, config)
}

// DynamicSyncerConfig represents the syncer configuration for a dynamic ORTB bidder
// This maps to the UserSyncConfig in the ORTB BidderConfig
type DynamicSyncerConfig struct {
	Enabled     bool
	IFrameURL   string
	RedirectURL string
	SupportCORS bool
	UserMacro   string
}

// Get overrides the base Get to ensure thread-safety
func (r *DynamicSyncerRegistry) Get(key string) (Syncer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.SyncerRegistry.Get(key)
}

// GetAll overrides the base GetAll to ensure thread-safety
func (r *DynamicSyncerRegistry) GetAll() map[string]Syncer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.SyncerRegistry.GetAll()
}

// Keys overrides the base Keys to ensure thread-safety
func (r *DynamicSyncerRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.SyncerRegistry.Keys()
}
