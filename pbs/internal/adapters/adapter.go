// Package adapters provides the bidder adapter framework
package adapters

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// maxResponseSize limits bidder response size to prevent OOM attacks
const maxResponseSize = 1024 * 1024 // 1MB

// Adapter defines the interface for bidder adapters
type Adapter interface {
	// MakeRequests builds HTTP requests for the bidder
	MakeRequests(request *openrtb.BidRequest, extraInfo *ExtraRequestInfo) ([]*RequestData, []error)

	// MakeBids parses bidder responses into bids
	MakeBids(request *openrtb.BidRequest, responseData *ResponseData) (*BidderResponse, []error)
}

// ExtraRequestInfo contains additional info for request building
type ExtraRequestInfo struct {
	PbsEntryPoint string
	GlobalPrivacy GlobalPrivacy
	BidderCoreName string
}

// GlobalPrivacy contains privacy settings
type GlobalPrivacy struct {
	GDPR      bool
	GDPRConsent string
	CCPA      string
	GPP       string
	GPPSID    []int
}

// RequestData represents an HTTP request to a bidder
type RequestData struct {
	Method  string
	URI     string
	Body    []byte
	Headers http.Header
}

// ResponseData represents an HTTP response from a bidder
type ResponseData struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// BidderResponse contains parsed bids from a bidder
type BidderResponse struct {
	Bids     []*TypedBid
	Currency string
}

// TypedBid is a bid with its type
type TypedBid struct {
	Bid      *openrtb.Bid
	BidType  BidType
	BidVideo *BidVideo
	BidMeta  *openrtb.ExtBidPrebidMeta
	DealPriority int
}

// BidType represents the type of bid
type BidType string

const (
	BidTypeBanner BidType = "banner"
	BidTypeVideo  BidType = "video"
	BidTypeAudio  BidType = "audio"
	BidTypeNative BidType = "native"
)

// BidVideo contains video-specific bid info
type BidVideo struct {
	Duration        int
	PrimaryCategory string
}

// BidderInfo contains bidder configuration
type BidderInfo struct {
	Enabled                 bool
	Maintainer              *MaintainerInfo
	Capabilities            *CapabilitiesInfo
	ModifyingVastXmlAllowed bool
	Debug                   *DebugInfo
	GVLVendorID             int
	Syncer                  *SyncerInfo
	Endpoint                string
	ExtraInfo               string
}

// MaintainerInfo contains maintainer info
type MaintainerInfo struct {
	Email string
}

// CapabilitiesInfo contains bidder capabilities
type CapabilitiesInfo struct {
	App  *PlatformInfo
	Site *PlatformInfo
}

// PlatformInfo contains platform capabilities
type PlatformInfo struct {
	MediaTypes []BidType
}

// DebugInfo contains debug configuration
type DebugInfo struct {
	AllowDebugOverride bool
}

// SyncerInfo contains user sync configuration
type SyncerInfo struct {
	Supports []string
}

// AdapterConfig holds runtime adapter configuration
type AdapterConfig struct {
	Endpoint string
	Disabled bool
	ExtraInfo string
}

// AdapterWithInfo wraps an adapter with its info
type AdapterWithInfo struct {
	Adapter Adapter
	Info    BidderInfo
}

// BidderResult contains bidding results
type BidderResult struct {
	BidderCode string
	Bids       []*TypedBid
	Errors     []error
	Latency    time.Duration
}

// HTTPClient defines the interface for HTTP requests
type HTTPClient interface {
	Do(ctx context.Context, req *RequestData, timeout time.Duration) (*ResponseData, error)
}

// DefaultHTTPClient implements HTTPClient
type DefaultHTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Do executes an HTTP request with proper timeout handling
func (c *DefaultHTTPClient) Do(ctx context.Context, req *RequestData, timeout time.Duration) (*ResponseData, error) {
	// Create timeout context for this specific request if timeout is provided
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URI, nil)
	if err != nil {
		return nil, err
	}

	if len(req.Body) > 0 {
		httpReq.Body = &bodyReader{data: req.Body}
		httpReq.ContentLength = int64(len(req.Body))
	}

	for k, v := range req.Headers {
		httpReq.Header[k] = v
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read body with size limit to prevent OOM from malicious bidders
	body := make([]byte, 0, 4096) // Pre-allocate reasonable initial capacity
	buf := make([]byte, 4096)
	totalRead := 0

	for {
		// Check context before each read to avoid blocking on slow responses
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			totalRead += n
			if totalRead > maxResponseSize {
				return nil, fmt.Errorf("response too large: exceeded %d bytes", maxResponseSize)
			}
			body = append(body, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break // Normal end of response
			}
			// For other errors, return what we have if we got some data
			if totalRead > 0 {
				break
			}
			return nil, err
		}
	}

	return &ResponseData{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}

// bodyReader wraps bytes for http.Request.Body
type bodyReader struct {
	data []byte
	pos  int
}

// Read implements io.Reader with proper EOF handling
func (r *bodyReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF // Return io.EOF when all data has been read
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	// Return io.EOF along with the final bytes if we've reached the end
	if r.pos >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

// Close implements io.Closer
func (r *bodyReader) Close() error {
	return nil
}
