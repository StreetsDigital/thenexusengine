package openrtb

import (
	"encoding/json"
	"testing"
)

func TestBidResponse_JSONRoundTrip(t *testing.T) {
	original := BidResponse{
		ID:         "resp-123",
		BidID:      "bid-456",
		Cur:        "USD",
		CustomData: "custom-data",
		NBR:        0,
		SeatBid: []SeatBid{
			{
				Seat:  "bidder-1",
				Group: 0,
				Bid: []Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-1",
						Price: 1.50,
						AdM:   "<div>ad markup</div>",
					},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("expected ID %s, got %s", original.ID, decoded.ID)
	}
	if decoded.BidID != original.BidID {
		t.Errorf("expected BidID %s, got %s", original.BidID, decoded.BidID)
	}
	if decoded.Cur != original.Cur {
		t.Errorf("expected Cur %s, got %s", original.Cur, decoded.Cur)
	}
	if len(decoded.SeatBid) != 1 {
		t.Fatalf("expected 1 seatbid, got %d", len(decoded.SeatBid))
	}
	if len(decoded.SeatBid[0].Bid) != 1 {
		t.Fatalf("expected 1 bid, got %d", len(decoded.SeatBid[0].Bid))
	}
	if decoded.SeatBid[0].Bid[0].Price != 1.50 {
		t.Errorf("expected price 1.50, got %f", decoded.SeatBid[0].Bid[0].Price)
	}
}

func TestBidResponse_MinimalValid(t *testing.T) {
	resp := BidResponse{
		ID: "resp-minimal",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != "resp-minimal" {
		t.Errorf("expected ID resp-minimal, got %s", decoded.ID)
	}
	if decoded.SeatBid != nil {
		t.Error("expected nil seatbid for minimal response")
	}
}

func TestBidResponse_NoBid(t *testing.T) {
	resp := BidResponse{
		ID:  "resp-nobid",
		NBR: int(NoBidBlockedPublisher),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.NBR != int(NoBidBlockedPublisher) {
		t.Errorf("expected NBR %d, got %d", NoBidBlockedPublisher, decoded.NBR)
	}
}

func TestBidResponse_WithExtension(t *testing.T) {
	ext := json.RawMessage(`{"custom":"value"}`)
	resp := BidResponse{
		ID:  "resp-ext",
		Ext: ext,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	var decodedExt map[string]string
	if err := json.Unmarshal(decoded.Ext, &decodedExt); err != nil {
		t.Fatalf("failed to unmarshal ext: %v", err)
	}
	if decodedExt["custom"] != "value" {
		t.Errorf("expected custom=value, got %s", decodedExt["custom"])
	}
}

func TestSeatBid_MultipleBids(t *testing.T) {
	seatBid := SeatBid{
		Seat:  "appnexus",
		Group: 1,
		Bid: []Bid{
			{ID: "bid-1", ImpID: "imp-1", Price: 1.00},
			{ID: "bid-2", ImpID: "imp-2", Price: 2.00},
			{ID: "bid-3", ImpID: "imp-3", Price: 3.00},
		},
	}

	data, err := json.Marshal(seatBid)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SeatBid
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Seat != "appnexus" {
		t.Errorf("expected seat appnexus, got %s", decoded.Seat)
	}
	if decoded.Group != 1 {
		t.Errorf("expected group 1, got %d", decoded.Group)
	}
	if len(decoded.Bid) != 3 {
		t.Fatalf("expected 3 bids, got %d", len(decoded.Bid))
	}

	// Verify bid order and prices
	expectedPrices := []float64{1.00, 2.00, 3.00}
	for i, bid := range decoded.Bid {
		if bid.Price != expectedPrices[i] {
			t.Errorf("bid %d: expected price %f, got %f", i, expectedPrices[i], bid.Price)
		}
	}
}

func TestBid_AllFields(t *testing.T) {
	bid := Bid{
		ID:             "bid-full",
		ImpID:          "imp-1",
		Price:          2.50,
		NURL:           "http://example.com/win",
		BURL:           "http://example.com/billing",
		LURL:           "http://example.com/loss",
		AdM:            "<div>creative</div>",
		AdID:           "ad-123",
		ADomain:        []string{"advertiser.com", "brand.com"},
		Bundle:         "com.example.app",
		IURL:           "http://example.com/image.png",
		CID:            "campaign-1",
		CRID:           "creative-1",
		Tactic:         "retargeting",
		Cat:            []string{"IAB1", "IAB2"},
		Attr:           []int{1, 2, 3},
		API:            3,
		Protocol:       2,
		QAGMediaRating: 1,
		Language:       "en",
		DealID:         "deal-premium",
		W:              300,
		H:              250,
		WRatio:         6,
		HRatio:         5,
		Exp:            3600,
	}

	data, err := json.Marshal(bid)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Bid
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != "bid-full" {
		t.Errorf("expected ID bid-full, got %s", decoded.ID)
	}
	if decoded.ImpID != "imp-1" {
		t.Errorf("expected ImpID imp-1, got %s", decoded.ImpID)
	}
	if decoded.Price != 2.50 {
		t.Errorf("expected price 2.50, got %f", decoded.Price)
	}
	if decoded.NURL != "http://example.com/win" {
		t.Errorf("expected NURL, got %s", decoded.NURL)
	}
	if decoded.BURL != "http://example.com/billing" {
		t.Errorf("expected BURL, got %s", decoded.BURL)
	}
	if len(decoded.ADomain) != 2 {
		t.Errorf("expected 2 adomains, got %d", len(decoded.ADomain))
	}
	if decoded.DealID != "deal-premium" {
		t.Errorf("expected dealid deal-premium, got %s", decoded.DealID)
	}
	if decoded.W != 300 || decoded.H != 250 {
		t.Errorf("expected 300x250, got %dx%d", decoded.W, decoded.H)
	}
	if decoded.Exp != 3600 {
		t.Errorf("expected exp 3600, got %d", decoded.Exp)
	}
}

func TestBid_MinimalValid(t *testing.T) {
	bid := Bid{
		ID:    "bid-min",
		ImpID: "imp-1",
		Price: 0.01,
	}

	data, err := json.Marshal(bid)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Bid
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != "bid-min" {
		t.Errorf("expected ID bid-min, got %s", decoded.ID)
	}
	if decoded.Price != 0.01 {
		t.Errorf("expected price 0.01, got %f", decoded.Price)
	}
	// Verify omitted fields are empty
	if decoded.AdM != "" {
		t.Error("expected empty AdM")
	}
	if decoded.ADomain != nil {
		t.Error("expected nil ADomain")
	}
}

func TestNoBidReason_Constants(t *testing.T) {
	tests := []struct {
		reason   NoBidReason
		expected int
	}{
		{NoBidUnknown, 0},
		{NoBidTechnicalError, 1},
		{NoBidInvalidRequest, 2},
		{NoBidKnownWebSpider, 3},
		{NoBidSuspectedNonHuman, 4},
		{NoBidCloudDataCenter, 5},
		{NoBidUnsupportedDevice, 6},
		{NoBidBlockedPublisher, 7},
		{NoBidUnmatchedUser, 8},
		{NoBidDailyReaderCapMet, 9},
		{NoBidDailyDomainCapMet, 10},
	}

	for _, tt := range tests {
		if int(tt.reason) != tt.expected {
			t.Errorf("expected %d, got %d", tt.expected, int(tt.reason))
		}
	}
}

func TestBidResponseExt_AllFields(t *testing.T) {
	ext := BidResponseExt{
		ResponseTimeMillis: map[string]int{
			"appnexus": 50,
			"rubicon":  75,
		},
		Errors: map[string][]ExtBidderMessage{
			"failed-bidder": {
				{Code: 1, Message: "timeout"},
			},
		},
		Warnings: map[string][]ExtBidderMessage{
			"warning-bidder": {
				{Code: 2, Message: "deprecated field"},
			},
		},
		TMMaxRequest: 500,
		Prebid: &ExtBidResponsePrebid{
			AuctionTimestamp: 1234567890,
		},
	}

	data, err := json.Marshal(ext)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponseExt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ResponseTimeMillis["appnexus"] != 50 {
		t.Errorf("expected appnexus time 50, got %d", decoded.ResponseTimeMillis["appnexus"])
	}
	if decoded.ResponseTimeMillis["rubicon"] != 75 {
		t.Errorf("expected rubicon time 75, got %d", decoded.ResponseTimeMillis["rubicon"])
	}
	if len(decoded.Errors["failed-bidder"]) != 1 {
		t.Error("expected 1 error for failed-bidder")
	}
	if decoded.Errors["failed-bidder"][0].Message != "timeout" {
		t.Error("expected timeout error message")
	}
	if decoded.TMMaxRequest != 500 {
		t.Errorf("expected tmax 500, got %d", decoded.TMMaxRequest)
	}
	if decoded.Prebid.AuctionTimestamp != 1234567890 {
		t.Errorf("expected timestamp 1234567890, got %d", decoded.Prebid.AuctionTimestamp)
	}
}

func TestExtBidderMessage(t *testing.T) {
	msg := ExtBidderMessage{
		Code:    999,
		Message: "custom error message",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ExtBidderMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Code != 999 {
		t.Errorf("expected code 999, got %d", decoded.Code)
	}
	if decoded.Message != "custom error message" {
		t.Errorf("expected message, got %s", decoded.Message)
	}
}

func TestBidExt_WithPrebid(t *testing.T) {
	bidExt := BidExt{
		Prebid: &ExtBidPrebid{
			Type: "banner",
			Targeting: map[string]string{
				"hb_pb":     "1.50",
				"hb_bidder": "appnexus",
			},
			Cache: &ExtBidPrebidCache{
				Key: "cache-key-123",
				URL: "http://cache.example.com/123",
				Bids: &CacheInfo{
					URL:     "http://cache.example.com/bid",
					CacheID: "cache-id-1",
				},
			},
			Video: &ExtBidPrebidVideo{
				Duration:        30,
				PrimaryCategory: "IAB1-1",
			},
			Events: &ExtBidPrebidEvents{
				Win: "http://example.com/win",
				Imp: "http://example.com/imp",
			},
			Meta: &ExtBidPrebidMeta{
				AdvertiserID:   123,
				AdvertiserName: "Advertiser Inc",
				BrandID:        456,
				BrandName:      "Brand Corp",
				MediaType:      "banner",
			},
		},
	}

	data, err := json.Marshal(bidExt)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidExt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Prebid == nil {
		t.Fatal("expected prebid extension")
	}
	if decoded.Prebid.Type != "banner" {
		t.Errorf("expected type banner, got %s", decoded.Prebid.Type)
	}
	if decoded.Prebid.Targeting["hb_pb"] != "1.50" {
		t.Error("expected hb_pb targeting")
	}
	if decoded.Prebid.Cache.Key != "cache-key-123" {
		t.Error("expected cache key")
	}
	if decoded.Prebid.Video.Duration != 30 {
		t.Errorf("expected duration 30, got %d", decoded.Prebid.Video.Duration)
	}
	if decoded.Prebid.Events.Win != "http://example.com/win" {
		t.Error("expected win event URL")
	}
	if decoded.Prebid.Meta.AdvertiserID != 123 {
		t.Errorf("expected advertiser ID 123, got %d", decoded.Prebid.Meta.AdvertiserID)
	}
}

func TestExtBidPrebidCache_VastXML(t *testing.T) {
	cache := ExtBidPrebidCache{
		VastXML: &CacheInfo{
			URL:     "http://cache.example.com/vast",
			CacheID: "vast-cache-id",
		},
	}

	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ExtBidPrebidCache
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.VastXML == nil {
		t.Fatal("expected VastXML cache info")
	}
	if decoded.VastXML.CacheID != "vast-cache-id" {
		t.Errorf("expected cache ID vast-cache-id, got %s", decoded.VastXML.CacheID)
	}
}

func TestExtBidPrebidMeta_AllFields(t *testing.T) {
	meta := ExtBidPrebidMeta{
		AdvertiserID:    100,
		AdvertiserName:  "Test Advertiser",
		AgencyID:        200,
		AgencyName:      "Test Agency",
		BrandID:         300,
		BrandName:       "Test Brand",
		DemandSource:    "rtb",
		MediaType:       "video",
		NetworkID:       400,
		NetworkName:     "Test Network",
		PrimaryCatID:    "IAB1",
		SecondaryCatIDs: []string{"IAB2", "IAB3"},
		RendererName:    "custom-renderer",
		RendererVersion: "1.0.0",
		RendererURL:     "http://example.com/renderer.js",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ExtBidPrebidMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.AdvertiserID != 100 {
		t.Errorf("expected advertiser ID 100, got %d", decoded.AdvertiserID)
	}
	if decoded.AgencyName != "Test Agency" {
		t.Errorf("expected agency Test Agency, got %s", decoded.AgencyName)
	}
	if len(decoded.SecondaryCatIDs) != 2 {
		t.Errorf("expected 2 secondary cats, got %d", len(decoded.SecondaryCatIDs))
	}
	if decoded.RendererURL != "http://example.com/renderer.js" {
		t.Error("expected renderer URL")
	}
}

func TestExtBidPrebidMeta_DChain(t *testing.T) {
	dchain := json.RawMessage(`{"ver":"1.0","complete":1,"nodes":[{"asi":"exchange.com","sid":"1234"}]}`)
	meta := ExtBidPrebidMeta{
		AdvertiserID: 1,
		DChain:       dchain,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ExtBidPrebidMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	var decodedDChain struct {
		Ver      string `json:"ver"`
		Complete int    `json:"complete"`
	}
	if err := json.Unmarshal(decoded.DChain, &decodedDChain); err != nil {
		t.Fatalf("failed to unmarshal dchain: %v", err)
	}
	if decodedDChain.Ver != "1.0" {
		t.Errorf("expected dchain ver 1.0, got %s", decodedDChain.Ver)
	}
}

func TestBidResponse_MultipleSeatBids(t *testing.T) {
	resp := BidResponse{
		ID:  "multi-seat",
		Cur: "USD",
		SeatBid: []SeatBid{
			{
				Seat: "bidder-a",
				Bid: []Bid{
					{ID: "a-1", ImpID: "imp-1", Price: 1.00},
					{ID: "a-2", ImpID: "imp-2", Price: 1.50},
				},
			},
			{
				Seat: "bidder-b",
				Bid: []Bid{
					{ID: "b-1", ImpID: "imp-1", Price: 1.25},
				},
			},
			{
				Seat: "bidder-c",
				Bid: []Bid{
					{ID: "c-1", ImpID: "imp-2", Price: 2.00},
					{ID: "c-2", ImpID: "imp-3", Price: 0.75},
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BidResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.SeatBid) != 3 {
		t.Fatalf("expected 3 seatbids, got %d", len(decoded.SeatBid))
	}

	// Count total bids
	totalBids := 0
	for _, sb := range decoded.SeatBid {
		totalBids += len(sb.Bid)
	}
	if totalBids != 5 {
		t.Errorf("expected 5 total bids, got %d", totalBids)
	}

	// Verify highest bid
	var highestPrice float64
	for _, sb := range decoded.SeatBid {
		for _, bid := range sb.Bid {
			if bid.Price > highestPrice {
				highestPrice = bid.Price
			}
		}
	}
	if highestPrice != 2.00 {
		t.Errorf("expected highest price 2.00, got %f", highestPrice)
	}
}

func TestBidResponse_RealWorld(t *testing.T) {
	responseJSON := `{
		"id": "auction-1234",
		"bidid": "response-5678",
		"cur": "USD",
		"seatbid": [
			{
				"seat": "appnexus",
				"bid": [
					{
						"id": "bid-abc",
						"impid": "banner-imp-1",
						"price": 2.35,
						"adm": "<div class=\"ad\">Banner Ad</div>",
						"adid": "creative-123",
						"adomain": ["brand.example.com"],
						"crid": "cr-789",
						"w": 300,
						"h": 250,
						"dealid": "premium-deal-1",
						"ext": {"bidder":"appnexus"}
					}
				]
			},
			{
				"seat": "rubicon",
				"bid": [
					{
						"id": "bid-xyz",
						"impid": "video-imp-1",
						"price": 5.00,
						"nurl": "https://rubicon.com/win?price=${AUCTION_PRICE}",
						"adm": "<VAST version=\"3.0\">...</VAST>",
						"adomain": ["advertiser.example.com"],
						"cat": ["IAB1-1"],
						"crid": "vcr-456",
						"w": 640,
						"h": 480
					}
				]
			}
		],
		"ext": {
			"responsetimemillis": {
				"appnexus": 45,
				"rubicon": 62
			}
		}
	}`

	var resp BidResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != "auction-1234" {
		t.Errorf("expected auction ID, got %s", resp.ID)
	}
	if resp.Cur != "USD" {
		t.Errorf("expected USD, got %s", resp.Cur)
	}
	if len(resp.SeatBid) != 2 {
		t.Fatalf("expected 2 seatbids, got %d", len(resp.SeatBid))
	}

	// Check appnexus bid
	appnexusSeat := resp.SeatBid[0]
	if appnexusSeat.Seat != "appnexus" {
		t.Errorf("expected appnexus, got %s", appnexusSeat.Seat)
	}
	if len(appnexusSeat.Bid) != 1 {
		t.Fatal("expected 1 appnexus bid")
	}
	if appnexusSeat.Bid[0].Price != 2.35 {
		t.Errorf("expected price 2.35, got %f", appnexusSeat.Bid[0].Price)
	}
	if appnexusSeat.Bid[0].DealID != "premium-deal-1" {
		t.Error("expected deal ID")
	}

	// Check rubicon bid
	rubiconSeat := resp.SeatBid[1]
	if rubiconSeat.Seat != "rubicon" {
		t.Errorf("expected rubicon, got %s", rubiconSeat.Seat)
	}
	if rubiconSeat.Bid[0].Price != 5.00 {
		t.Errorf("expected price 5.00, got %f", rubiconSeat.Bid[0].Price)
	}
	if rubiconSeat.Bid[0].W != 640 || rubiconSeat.Bid[0].H != 480 {
		t.Error("expected 640x480 dimensions")
	}
}

// Benchmark tests
func BenchmarkBidResponse_Marshal(b *testing.B) {
	resp := BidResponse{
		ID:  "bench-response",
		Cur: "USD",
		SeatBid: []SeatBid{
			{
				Seat: "bidder-1",
				Bid: []Bid{
					{ID: "bid-1", ImpID: "imp-1", Price: 1.50, AdM: "<div>ad</div>"},
					{ID: "bid-2", ImpID: "imp-2", Price: 2.00, AdM: "<div>ad</div>"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}

func BenchmarkBidResponse_Unmarshal(b *testing.B) {
	data := []byte(`{"id":"bench","cur":"USD","seatbid":[{"seat":"bidder","bid":[{"id":"b1","impid":"i1","price":1.5,"adm":"<div>ad</div>"}]}]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp BidResponse
		_ = json.Unmarshal(data, &resp)
	}
}

func BenchmarkBid_Marshal(b *testing.B) {
	bid := Bid{
		ID:      "bench-bid",
		ImpID:   "imp-1",
		Price:   1.50,
		AdM:     "<div>benchmark ad markup</div>",
		ADomain: []string{"example.com"},
		CID:     "campaign-1",
		CRID:    "creative-1",
		W:       300,
		H:       250,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(bid)
	}
}
