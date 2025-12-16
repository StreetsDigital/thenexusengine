package usersync

// DefaultSyncerConfigs returns the default sync configurations for all supported bidders
// These URLs are based on Prebid.org documentation and bidder specifications
// The {{redirect_url}} macro will be replaced with the PBS setuid endpoint
func DefaultSyncerConfigs() map[string]*SyncerConfig {
	return map[string]*SyncerConfig{
		"33across": {
			Key:      "33across",
			Supports: []SyncType{SyncTypeIFrame},
			Default:  SyncTypeIFrame,
			IFrame: &SyncURLTemplate{
				URL:       "https://ssc.33across.com/ps/?pid=0013a00001kQhnMQAS&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}%3Fbidder%3D33across%26uid%3D33XUSERID33X",
				UserMacro: "33XUSERID33X",
			},
			SupportCORS: false,
		},

		"adform": {
			Key:      "adform",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://cm.adform.net/cookie?redirect_url={{redirect_url}}%3Fbidder%3Dadform%26uid%3D%24UID",
				UserMacro: "$UID",
			},
			SupportCORS: false,
		},

		"appnexus": {
			Key:      "adnxs",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			IFrame: &SyncURLTemplate{
				URL:       "https://ib.adnxs.com/getuid?{{redirect_url}}%3Fbidder%3Dappnexus%26uid%3D%24UID",
				UserMacro: "$UID",
			},
			Redirect: &SyncURLTemplate{
				URL:       "https://ib.adnxs.com/getuid?{{redirect_url}}%3Fbidder%3Dappnexus%26uid%3D%24UID",
				UserMacro: "$UID",
			},
			SupportCORS: false,
		},

		"beachfront": {
			Key:      "beachfront",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeIFrame,
			IFrame: &SyncURLTemplate{
				URL:       "https://sync.bfmio.com/sync_s2s?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&url={{redirect_url}}%3Fbidder%3Dbeachfront%26uid%3D%5Bio_cid%5D",
				UserMacro: "[io_cid]",
			},
			Redirect: &SyncURLTemplate{
				URL:       "https://sync.bfmio.com/syncb?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&url={{redirect_url}}%3Fbidder%3Dbeachfront%26uid%3D%5Bio_cid%5D",
				UserMacro: "[io_cid]",
			},
			SupportCORS: false,
		},

		"conversant": {
			Key:      "conversant",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://prebid-match.dotomi.com/match/bounce/current?version=1&networkId=72582&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&rurl={{redirect_url}}%3Fbidder%3Dconversant%26uid%3D",
				UserMacro: "",
			},
			SupportCORS: false,
		},

		"criteo": {
			Key:      "criteo",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ssp-sync.criteo.com/user-sync/prebid?p={{redirect_url}}%3Fbidder%3Dcriteo%26uid%3D%24%7Buser_id%7D&gdpr={{gdpr}}&consent={{gdpr_consent}}&us_privacy={{us_privacy}}",
				UserMacro: "${user_id}",
			},
			SupportCORS: false,
		},

		"gumgum": {
			Key:      "gumgum",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://rtb.gumgum.com/usync/prbds2s?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&r={{redirect_url}}%3Fbidder%3Dgumgum%26uid%3D",
				UserMacro: "",
			},
			SupportCORS: false,
		},

		"improvedigital": {
			Key:      "improvedigital",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ad.360yield.com/server_match?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&r={{redirect_url}}%3Fbidder%3Dimprovedigital%26uid%3D%7BPUB_USER_ID%7D",
				UserMacro: "{PUB_USER_ID}",
			},
			SupportCORS: false,
		},

		"ix": {
			Key:      "ix",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ssum.casalemedia.com/usermatchredir?s=186523&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&cb={{redirect_url}}%3Fbidder%3Dix%26uid%3D",
				UserMacro: "",
			},
			SupportCORS: false,
		},

		"medianet": {
			Key:      "medianet",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://dir.adsrvr.org/um/1/13216/s_match?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&ru={{redirect_url}}%3Fbidder%3Dmedianet%26uid%3D%24%7BUM_UUID%7D",
				UserMacro: "${UM_UUID}",
			},
			SupportCORS: false,
		},

		"openx": {
			Key:      "openx",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			IFrame: &SyncURLTemplate{
				URL:       "https://u.openx.net/w/1.0/pd?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}",
				UserMacro: "",
			},
			Redirect: &SyncURLTemplate{
				URL:       "https://rtb.openx.net/sync/prebid?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&r={{redirect_url}}%3Fbidder%3Dopenx%26uid%3D%24%7BUID%7D",
				UserMacro: "${UID}",
			},
			SupportCORS: false,
		},

		"outbrain": {
			Key:      "outbrain",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://prebidtest.zemanta.com/usersync/prebid?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&cb={{redirect_url}}%3Fbidder%3Doutbrain%26uid%3D__ZUID__",
				UserMacro: "__ZUID__",
			},
			SupportCORS: false,
		},

		"pubmatic": {
			Key:      "pubmatic",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeIFrame,
			IFrame: &SyncURLTemplate{
				URL:       "https://ads.pubmatic.com/AdServer/js/user_sync.html?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&predirect={{redirect_url}}%3Fbidder%3Dpubmatic%26uid%3D",
				UserMacro: "",
			},
			Redirect: &SyncURLTemplate{
				URL:       "https://image8.pubmatic.com/AdServer/ImgSync?p=159706&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&pu={{redirect_url}}%3Fbidder%3Dpubmatic%26uid%3D",
				UserMacro: "",
			},
			SupportCORS: false,
		},

		"rubicon": {
			Key:      "rubicon",
			Supports: []SyncType{SyncTypeIFrame, SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			IFrame: &SyncURLTemplate{
				URL:       "https://pixel.rubiconproject.com/exchange/sync.php?p=prebid&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}",
				UserMacro: "",
			},
			Redirect: &SyncURLTemplate{
				URL:       "https://pixel.rubiconproject.com/exchange/sync.php?p=prebid&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&ru={{redirect_url}}%3Fbidder%3Drubicon%26uid%3D%24%7BUSER_ID%7D",
				UserMacro: "${USER_ID}",
			},
			SupportCORS: false,
		},

		"sharethrough": {
			Key:      "sharethrough",
			Supports: []SyncType{SyncTypeIFrame},
			Default:  SyncTypeIFrame,
			IFrame: &SyncURLTemplate{
				URL:       "https://match.sharethrough.com/universal/v1?supply_id=WYu2FL58&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}%3Fbidder%3Dsharethrough%26uid%3D__SUID__",
				UserMacro: "__SUID__",
			},
			SupportCORS: false,
		},

		"smartadserver": {
			Key:      "smartadserver",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ssbsync-global.smartadserver.com/api/sync?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&rurl={{redirect_url}}%3Fbidder%3Dsmartadserver%26uid%3D%5Bssb_sync_pid%5D",
				UserMacro: "[ssb_sync_pid]",
			},
			SupportCORS: false,
		},

		"sovrn": {
			Key:      "sovrn",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://ap.lijit.com/pixel?redir={{redirect_url}}%3Fbidder%3Dsovrn%26uid%3D%24UID&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}",
				UserMacro: "$UID",
			},
			SupportCORS: false,
		},

		"spotx": {
			Key:      "spotx",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://sync.search.spotxchange.com/partner?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redirect={{redirect_url}}%3Fbidder%3Dspotx%26uid%3D%24SPOTX_USER_ID",
				UserMacro: "$SPOTX_USER_ID",
			},
			SupportCORS: false,
		},

		"taboola": {
			Key:      "taboola",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://trc.taboola.com/sg/prebidserver/1/cm?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redirect={{redirect_url}}%3Fbidder%3Dtaboola%26uid%3D%5BTABOOLA_USER_ID%5D",
				UserMacro: "[TABOOLA_USER_ID]",
			},
			SupportCORS: false,
		},

		"teads": {
			Key:      "teads",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://sync.teads.tv/page/1?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redirect={{redirect_url}}%3Fbidder%3Dteads%26uid%3D%5BTEADS_USER_ID%5D",
				UserMacro: "[TEADS_USER_ID]",
			},
			SupportCORS: false,
		},

		"triplelift": {
			Key:      "triplelift",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://eb2.3lift.com/getuid?gdpr={{gdpr}}&cmp_cs={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}%3Fbidder%3Dtriplelift%26uid%3D%24UID",
				UserMacro: "$UID",
			},
			SupportCORS: false,
		},

		"unruly": {
			Key:      "unruly",
			Supports: []SyncType{SyncTypeRedirect},
			Default:  SyncTypeRedirect,
			Redirect: &SyncURLTemplate{
				URL:       "https://sync.1rx.io/usersync2/rmphb?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}%3Fbidder%3Dunruly%26uid%3D%5BRX_UUID%5D",
				UserMacro: "[RX_UUID]",
			},
			SupportCORS: false,
		},
	}
}

// NewDefaultSyncer creates a new Syncer with default configurations
func NewDefaultSyncer(config CookieSyncConfig) *Syncer {
	return NewSyncer(DefaultSyncerConfigs(), config)
}

// GetSyncerConfigForBidder is a helper to get sync config for a specific bidder
func GetSyncerConfigForBidder(bidder string) (*SyncerConfig, bool) {
	configs := DefaultSyncerConfigs()
	cfg, ok := configs[bidder]
	return cfg, ok
}

// SupportedSyncBidders returns a list of all bidders that support user syncing
func SupportedSyncBidders() []string {
	configs := DefaultSyncerConfigs()
	bidders := make([]string, 0, len(configs))
	for bidder := range configs {
		bidders = append(bidders, bidder)
	}
	return bidders
}
