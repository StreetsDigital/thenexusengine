// Package openx implements the OpenX bidder adapter
package openx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

const defaultEndpoint = "https://rtb.openx.net/openrtb/prebid"

// Adapter implements the OpenX bidder
type Adapter struct {
	endpoint string
}

// New creates a new OpenX adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

// MakeRequests builds HTTP requests for OpenX
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal request: %v", err)}
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json;charset=utf-8")
	headers.Set("Accept", "application/json")

	return []*adapters.RequestData{
		{Method: "POST", URI: a.endpoint, Body: requestBody, Headers: headers},
	}, nil
}

// MakeBids parses OpenX responses into bids
func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if responseData.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		return nil, []error{fmt.Errorf("failed to parse response: %v", err)}
	}

	response := &adapters.BidderResponse{Currency: bidResp.Cur, Bids: make([]*adapters.TypedBid, 0)}
	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     &seatBid.Bid[i],
				BidType: getBidType(&seatBid.Bid[i], request),
			})
		}
	}
	return response, nil
}

func getBidType(bid *openrtb.Bid, request *openrtb.BidRequest) adapters.BidType {
	for _, imp := range request.Imp {
		if imp.ID == bid.ImpID {
			if imp.Video != nil {
				return adapters.BidTypeVideo
			}
			if imp.Native != nil {
				return adapters.BidTypeNative
			}
			return adapters.BidTypeBanner
		}
	}
	return adapters.BidTypeBanner
}

// Info returns bidder information
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled:     true,
		GVLVendorID: 69,
		Endpoint:    defaultEndpoint,
		Maintainer:  &adapters.MaintainerInfo{Email: "prebid@openx.com"},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
			App:  &adapters.PlatformInfo{MediaTypes: []adapters.BidType{adapters.BidTypeBanner, adapters.BidTypeVideo}},
		},
	}
}

func init() {
	adapters.RegisterAdapter("openx", New(""), Info())
}
