package openrtb

import "encoding/json"

// BidResponse represents an OpenRTB 2.5/2.6 bid response
type BidResponse struct {
	ID         string          `json:"id"`
	SeatBid    []SeatBid       `json:"seatbid,omitempty"`
	BidID      string          `json:"bidid,omitempty"`
	Cur        string          `json:"cur,omitempty"`
	CustomData string          `json:"customdata,omitempty"`
	NBR        int             `json:"nbr,omitempty"` // No-bid reason code
	Ext        json.RawMessage `json:"ext,omitempty"`
}

// SeatBid represents a seat bid
type SeatBid struct {
	Bid   []Bid           `json:"bid"`
	Seat  string          `json:"seat,omitempty"`
	Group int             `json:"group,omitempty"`
	Ext   json.RawMessage `json:"ext,omitempty"`
}

// Bid represents a bid
type Bid struct {
	ID             string          `json:"id"`
	ImpID          string          `json:"impid"`
	Price          float64         `json:"price"`
	NURL           string          `json:"nurl,omitempty"`
	BURL           string          `json:"burl,omitempty"`
	LURL           string          `json:"lurl,omitempty"`
	AdM            string          `json:"adm,omitempty"`
	AdID           string          `json:"adid,omitempty"`
	ADomain        []string        `json:"adomain,omitempty"`
	Bundle         string          `json:"bundle,omitempty"`
	IURL           string          `json:"iurl,omitempty"`
	CID            string          `json:"cid,omitempty"`
	CRID           string          `json:"crid,omitempty"`
	Tactic         string          `json:"tactic,omitempty"`
	Cat            []string        `json:"cat,omitempty"`
	Attr           []int           `json:"attr,omitempty"`
	API            int             `json:"api,omitempty"`
	Protocol       int             `json:"protocol,omitempty"`
	QAGMediaRating int             `json:"qagmediarating,omitempty"`
	Language       string          `json:"language,omitempty"`
	DealID         string          `json:"dealid,omitempty"`
	W              int             `json:"w,omitempty"`
	H              int             `json:"h,omitempty"`
	WRatio         int             `json:"wratio,omitempty"`
	HRatio         int             `json:"hratio,omitempty"`
	Exp            int             `json:"exp,omitempty"`
	Ext            json.RawMessage `json:"ext,omitempty"`
}

// NoBidReason represents no-bid reason codes (NBR)
type NoBidReason int

const (
	NoBidUnknown              NoBidReason = 0
	NoBidTechnicalError       NoBidReason = 1
	NoBidInvalidRequest       NoBidReason = 2
	NoBidKnownWebSpider       NoBidReason = 3
	NoBidSuspectedNonHuman    NoBidReason = 4
	NoBidCloudDataCenter      NoBidReason = 5
	NoBidUnsupportedDevice    NoBidReason = 6
	NoBidBlockedPublisher     NoBidReason = 7
	NoBidUnmatchedUser        NoBidReason = 8
	NoBidDailyReaderCapMet    NoBidReason = 9
	NoBidDailyDomainCapMet    NoBidReason = 10
)

// BidResponseExt represents PBS-specific response extensions
type BidResponseExt struct {
	ResponseTimeMillis map[string]int    `json:"responsetimemillis,omitempty"`
	Errors             map[string][]ExtBidderMessage `json:"errors,omitempty"`
	Warnings           map[string][]ExtBidderMessage `json:"warnings,omitempty"`
	TMMaxRequest       int               `json:"tmaxrequest,omitempty"`
	Prebid             *ExtBidResponsePrebid `json:"prebid,omitempty"`
}

// ExtBidderMessage represents bidder message
type ExtBidderMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ExtBidResponsePrebid represents prebid response extension
type ExtBidResponsePrebid struct {
	AuctionTimestamp int64                     `json:"auctiontimestamp,omitempty"`
	Passthrough      json.RawMessage           `json:"passthrough,omitempty"`
}

// BidExt represents bid extension
type BidExt struct {
	Prebid *ExtBidPrebid `json:"prebid,omitempty"`
}

// ExtBidPrebid represents prebid bid extension
type ExtBidPrebid struct {
	Cache       *ExtBidPrebidCache `json:"cache,omitempty"`
	Targeting   map[string]string  `json:"targeting,omitempty"`
	Type        string             `json:"type,omitempty"`
	Video       *ExtBidPrebidVideo `json:"video,omitempty"`
	Events      *ExtBidPrebidEvents `json:"events,omitempty"`
	Meta        *ExtBidPrebidMeta  `json:"meta,omitempty"`
}

// ExtBidPrebidCache represents cache info
type ExtBidPrebidCache struct {
	Key  string `json:"key,omitempty"`
	URL  string `json:"url,omitempty"`
	Bids *CacheInfo `json:"bids,omitempty"`
	VastXML *CacheInfo `json:"vastXml,omitempty"`
}

// CacheInfo represents cache information
type CacheInfo struct {
	URL     string `json:"url,omitempty"`
	CacheID string `json:"cacheId,omitempty"`
}

// ExtBidPrebidVideo represents video info
type ExtBidPrebidVideo struct {
	Duration        int    `json:"duration,omitempty"`
	PrimaryCategory string `json:"primary_category,omitempty"`
}

// ExtBidPrebidEvents represents event URLs
type ExtBidPrebidEvents struct {
	Win string `json:"win,omitempty"`
	Imp string `json:"imp,omitempty"`
}

// ExtBidPrebidMeta represents bid metadata
type ExtBidPrebidMeta struct {
	AdvertiserID        int      `json:"advertiserId,omitempty"`
	AdvertiserName      string   `json:"advertiserName,omitempty"`
	AgencyID            int      `json:"agencyId,omitempty"`
	AgencyName          string   `json:"agencyName,omitempty"`
	BrandID             int      `json:"brandId,omitempty"`
	BrandName           string   `json:"brandName,omitempty"`
	DChain              json.RawMessage `json:"dchain,omitempty"`
	DemandSource        string   `json:"demandSource,omitempty"`
	MediaType           string   `json:"mediaType,omitempty"`
	NetworkID           int      `json:"networkId,omitempty"`
	NetworkName         string   `json:"networkName,omitempty"`
	PrimaryCatID        string   `json:"primaryCatId,omitempty"`
	SecondaryCatIDs     []string `json:"secondaryCatIds,omitempty"`
	RendererName        string   `json:"rendererName,omitempty"`
	RendererVersion     string   `json:"rendererVersion,omitempty"`
	RendererURL         string   `json:"rendererUrl,omitempty"`
}
