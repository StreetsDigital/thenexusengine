// Package idr provides event recording for the Python IDR service
package idr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EventRecorder sends auction events to the IDR service
type EventRecorder struct {
	baseURL    string
	httpClient *http.Client
	buffer     []BidEvent
	bufferSize int
}

// BidEvent represents a bid event to record
type BidEvent struct {
	AuctionID   string   `json:"auction_id"`
	BidderCode  string   `json:"bidder_code"`
	EventType   string   `json:"event_type"` // "bid_response" or "win"
	LatencyMs   float64  `json:"latency_ms,omitempty"`
	HadBid      bool     `json:"had_bid,omitempty"`
	BidCPM      *float64 `json:"bid_cpm,omitempty"`
	WinCPM      *float64 `json:"win_cpm,omitempty"`
	FloorPrice  *float64 `json:"floor_price,omitempty"`
	Country     string   `json:"country,omitempty"`
	DeviceType  string   `json:"device_type,omitempty"`
	MediaType   string   `json:"media_type,omitempty"`
	AdSize      string   `json:"ad_size,omitempty"`
	PublisherID string   `json:"publisher_id,omitempty"`
	TimedOut    bool     `json:"timed_out,omitempty"`
	HadError    bool     `json:"had_error,omitempty"`
	ErrorMsg    string   `json:"error_message,omitempty"`
}

// NewEventRecorder creates a new event recorder
func NewEventRecorder(baseURL string, bufferSize int) *EventRecorder {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventRecorder{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		buffer:     make([]BidEvent, 0, bufferSize),
		bufferSize: bufferSize,
	}
}

// RecordBidResponse records a bid response event
func (r *EventRecorder) RecordBidResponse(
	auctionID string,
	bidderCode string,
	latencyMs float64,
	hadBid bool,
	bidCPM *float64,
	floorPrice *float64,
	country string,
	deviceType string,
	mediaType string,
	adSize string,
	publisherID string,
	timedOut bool,
	hadError bool,
	errorMsg string,
) {
	event := BidEvent{
		AuctionID:   auctionID,
		BidderCode:  bidderCode,
		EventType:   "bid_response",
		LatencyMs:   latencyMs,
		HadBid:      hadBid,
		BidCPM:      bidCPM,
		FloorPrice:  floorPrice,
		Country:     country,
		DeviceType:  deviceType,
		MediaType:   mediaType,
		AdSize:      adSize,
		PublisherID: publisherID,
		TimedOut:    timedOut,
		HadError:    hadError,
		ErrorMsg:    errorMsg,
	}

	r.buffer = append(r.buffer, event)

	// Flush if buffer is full
	if len(r.buffer) >= r.bufferSize {
		go r.Flush(context.Background())
	}
}

// RecordWin records a win event
func (r *EventRecorder) RecordWin(
	auctionID string,
	bidderCode string,
	winCPM float64,
	country string,
	deviceType string,
	mediaType string,
	adSize string,
	publisherID string,
) {
	event := BidEvent{
		AuctionID:   auctionID,
		BidderCode:  bidderCode,
		EventType:   "win",
		WinCPM:      &winCPM,
		Country:     country,
		DeviceType:  deviceType,
		MediaType:   mediaType,
		AdSize:      adSize,
		PublisherID: publisherID,
	}

	r.buffer = append(r.buffer, event)

	// Flush if buffer is full
	if len(r.buffer) >= r.bufferSize {
		go r.Flush(context.Background())
	}
}

// Flush sends buffered events to the IDR service
func (r *EventRecorder) Flush(ctx context.Context) error {
	if len(r.buffer) == 0 {
		return nil
	}

	// Swap buffer
	events := r.buffer
	r.buffer = make([]BidEvent, 0, r.bufferSize)

	// Build request
	reqBody := map[string]interface{}{
		"events": events,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	url := r.baseURL + "/api/events"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IDR service returned status %d", resp.StatusCode)
	}

	return nil
}

// Close flushes remaining events and closes the recorder
func (r *EventRecorder) Close() error {
	return r.Flush(context.Background())
}
