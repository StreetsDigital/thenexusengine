package fpd

import (
	"encoding/json"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// Processor handles First Party Data processing for bid requests
type Processor struct {
	config *Config
}

// NewProcessor creates a new FPD processor with the given configuration
func NewProcessor(config *Config) *Processor {
	if config == nil {
		config = DefaultConfig()
	}
	return &Processor{config: config}
}

// ProcessRequest processes FPD in a bid request and returns bidder-specific FPD
// This is the main entry point for FPD processing
func (p *Processor) ProcessRequest(req *openrtb.BidRequest, bidders []string) (BidderFPD, error) {
	if !p.config.Enabled {
		return nil, nil
	}

	result := make(BidderFPD)

	// Parse the request extension to get Prebid FPD
	var prebidExt *PrebidExt
	if req.Ext != nil {
		var reqExt struct {
			Prebid *PrebidExt `json:"prebid,omitempty"`
		}
		if err := json.Unmarshal(req.Ext, &reqExt); err == nil {
			prebidExt = reqExt.Prebid
		}
	}

	// Extract base FPD from the request
	baseFPD := p.extractBaseFPD(req)

	// Apply global FPD if enabled
	if p.config.GlobalEnabled && prebidExt != nil && prebidExt.Data != nil {
		baseFPD = p.mergeGlobalFPD(baseFPD, prebidExt.Data)
	}

	// Process each bidder
	for _, bidder := range bidders {
		bidderFPD := p.cloneFPD(baseFPD)

		// Apply bidder-specific config if enabled
		if p.config.BidderConfigEnabled && prebidExt != nil {
			bidderFPD = p.applyBidderConfig(bidderFPD, bidder, prebidExt.BidderConfig)
		}

		result[bidder] = bidderFPD
	}

	return result, nil
}

// extractBaseFPD extracts base FPD from the request's site/app/user objects
func (p *Processor) extractBaseFPD(req *openrtb.BidRequest) *ResolvedFPD {
	fpd := &ResolvedFPD{
		Imp: make(map[string]json.RawMessage),
	}

	// Extract site.ext.data if enabled
	if p.config.SiteEnabled && req.Site != nil && req.Site.Ext != nil {
		fpd.Site = p.extractExtData(req.Site.Ext)
	}

	// Extract app.ext.data if enabled
	if p.config.SiteEnabled && req.App != nil && req.App.Ext != nil {
		fpd.App = p.extractExtData(req.App.Ext)
	}

	// Extract user.ext.data if enabled
	if p.config.UserEnabled && req.User != nil && req.User.Ext != nil {
		fpd.User = p.extractExtData(req.User.Ext)
	}

	// Extract imp[].ext.data if enabled
	if p.config.ImpEnabled {
		for _, imp := range req.Imp {
			if imp.Ext != nil {
				if data := p.extractExtData(imp.Ext); data != nil {
					fpd.Imp[imp.ID] = data
				}
			}
		}
	}

	return fpd
}

// extractExtData extracts the data field from an ext object
func (p *Processor) extractExtData(ext json.RawMessage) json.RawMessage {
	if ext == nil {
		return nil
	}

	var extObj struct {
		Data json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(ext, &extObj); err != nil {
		return nil
	}
	return extObj.Data
}

// mergeGlobalFPD merges global FPD (ext.prebid.data) into the base FPD
func (p *Processor) mergeGlobalFPD(base *ResolvedFPD, global *PrebidData) *ResolvedFPD {
	result := p.cloneFPD(base)

	if global.Site != nil {
		result.Site = p.mergeJSON(result.Site, global.Site)
	}
	if global.App != nil {
		result.App = p.mergeJSON(result.App, global.App)
	}
	if global.User != nil {
		result.User = p.mergeJSON(result.User, global.User)
	}

	return result
}

// applyBidderConfig applies bidder-specific FPD from ext.prebid.bidderconfig
func (p *Processor) applyBidderConfig(base *ResolvedFPD, bidder string, configs []BidderConfig) *ResolvedFPD {
	result := p.cloneFPD(base)

	for _, config := range configs {
		// Check if this config applies to this bidder
		if !p.bidderMatches(bidder, config.Bidders) {
			continue
		}

		// Apply the config
		if config.Config != nil && config.Config.ORTB2 != nil {
			ortb2 := config.Config.ORTB2

			if ortb2.Site != nil {
				siteJSON, _ := json.Marshal(ortb2.Site)
				result.Site = p.mergeJSON(result.Site, siteJSON)
			}
			if ortb2.App != nil {
				appJSON, _ := json.Marshal(ortb2.App)
				result.App = p.mergeJSON(result.App, appJSON)
			}
			if ortb2.User != nil {
				userJSON, _ := json.Marshal(ortb2.User)
				result.User = p.mergeJSON(result.User, userJSON)
			}
		}
	}

	return result
}

// bidderMatches checks if a bidder is in the list (supports "*" for all bidders)
func (p *Processor) bidderMatches(bidder string, bidders []string) bool {
	for _, b := range bidders {
		if b == "*" || b == bidder {
			return true
		}
	}
	return false
}

// cloneFPD creates a deep copy of ResolvedFPD
func (p *Processor) cloneFPD(fpd *ResolvedFPD) *ResolvedFPD {
	if fpd == nil {
		return &ResolvedFPD{Imp: make(map[string]json.RawMessage)}
	}

	clone := &ResolvedFPD{
		Site: copyJSON(fpd.Site),
		App:  copyJSON(fpd.App),
		User: copyJSON(fpd.User),
		Imp:  make(map[string]json.RawMessage),
	}

	for k, v := range fpd.Imp {
		clone.Imp[k] = copyJSON(v)
	}

	return clone
}

// mergeJSON merges two JSON objects (shallow merge, second overwrites first)
func (p *Processor) mergeJSON(base, overlay json.RawMessage) json.RawMessage {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}

	var baseMap, overlayMap map[string]json.RawMessage

	if err := json.Unmarshal(base, &baseMap); err != nil {
		return overlay
	}
	if err := json.Unmarshal(overlay, &overlayMap); err != nil {
		return base
	}

	// Merge overlay into base
	for k, v := range overlayMap {
		baseMap[k] = v
	}

	result, _ := json.Marshal(baseMap)
	return result
}

// copyJSON creates a copy of a JSON raw message
func copyJSON(data json.RawMessage) json.RawMessage {
	if data == nil {
		return nil
	}
	clone := make(json.RawMessage, len(data))
	copy(clone, data)
	return clone
}

// ApplyFPDToRequest applies resolved FPD to a bid request for a specific bidder
// This modifies the request in place to include the FPD data
func (p *Processor) ApplyFPDToRequest(req *openrtb.BidRequest, bidder string, fpd *ResolvedFPD) error {
	if fpd == nil {
		return nil
	}

	// Apply site FPD
	if fpd.Site != nil && req.Site != nil {
		req.Site.Ext = p.setExtData(req.Site.Ext, fpd.Site)
	}

	// Apply app FPD
	if fpd.App != nil && req.App != nil {
		req.App.Ext = p.setExtData(req.App.Ext, fpd.App)
	}

	// Apply user FPD
	if fpd.User != nil && req.User != nil {
		req.User.Ext = p.setExtData(req.User.Ext, fpd.User)
	}

	// Apply impression FPD
	for i := range req.Imp {
		if impFPD, ok := fpd.Imp[req.Imp[i].ID]; ok {
			req.Imp[i].Ext = p.setExtData(req.Imp[i].Ext, impFPD)
		}
	}

	return nil
}

// setExtData sets the data field in an ext object
func (p *Processor) setExtData(ext json.RawMessage, data json.RawMessage) json.RawMessage {
	if data == nil {
		return ext
	}

	var extObj map[string]json.RawMessage
	if ext != nil {
		if err := json.Unmarshal(ext, &extObj); err != nil {
			extObj = make(map[string]json.RawMessage)
		}
	} else {
		extObj = make(map[string]json.RawMessage)
	}

	extObj["data"] = data

	result, _ := json.Marshal(extObj)
	return result
}

// GetConfig returns the processor's configuration
func (p *Processor) GetConfig() *Config {
	return p.config
}

// UpdateConfig updates the processor's configuration
func (p *Processor) UpdateConfig(config *Config) {
	if config != nil {
		p.config = config
	}
}
