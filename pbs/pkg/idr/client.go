// Package idr provides a client for the Python IDR service
package idr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client communicates with the Python IDR service
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewClient creates a new IDR client
func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 50 * time.Millisecond // IDR should be fast
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// SelectPartnersRequest is the request to select partners
type SelectPartnersRequest struct {
	Request         json.RawMessage `json:"request"`          // OpenRTB request
	AvailableBidders []string       `json:"available_bidders"` // Bidders to consider
}

// SelectPartnersResponse is the response from partner selection
type SelectPartnersResponse struct {
	SelectedBidders  []SelectedBidder `json:"selected_bidders"`
	ExcludedBidders  []ExcludedBidder `json:"excluded_bidders,omitempty"`
	Mode             string           `json:"mode"`    // "normal", "shadow", "bypass"
	ProcessingTimeMs float64          `json:"processing_time_ms"`
}

// SelectedBidder represents a selected bidder
type SelectedBidder struct {
	BidderCode string  `json:"bidder_code"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"` // ANCHOR, HIGH_SCORE, DIVERSITY, EXPLORATION, etc.
}

// ExcludedBidder represents an excluded bidder (shadow mode)
type ExcludedBidder struct {
	BidderCode string  `json:"bidder_code"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
}

// SelectPartners calls the IDR service to select optimal bidders
func (c *Client) SelectPartners(ctx context.Context, ortbRequest json.RawMessage, availableBidders []string) (*SelectPartnersResponse, error) {
	reqBody := SelectPartnersRequest{
		Request:          ortbRequest,
		AvailableBidders: availableBidders,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/api/select"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call IDR service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IDR service returned status %d", resp.StatusCode)
	}

	var result SelectPartnersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetConfig retrieves current IDR configuration
func (c *Client) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	url := c.baseURL + "/api/config"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call IDR service: %w", err)
	}
	defer resp.Body.Close()

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return config, nil
}

// SetBypassMode enables/disables bypass mode
func (c *Client) SetBypassMode(ctx context.Context, enabled bool) error {
	return c.setMode(ctx, "/api/mode/bypass", enabled)
}

// SetShadowMode enables/disables shadow mode
func (c *Client) SetShadowMode(ctx context.Context, enabled bool) error {
	return c.setMode(ctx, "/api/mode/shadow", enabled)
}

func (c *Client) setMode(ctx context.Context, path string, enabled bool) error {
	body, _ := json.Marshal(map[string]bool{"enabled": enabled})

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call IDR service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IDR service returned status %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck checks if IDR service is healthy
func (c *Client) HealthCheck(ctx context.Context) error {
	url := c.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IDR service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// FPDConfig represents First Party Data configuration from the IDR service
type FPDConfig struct {
	Enabled             bool     `json:"enabled"`
	SiteEnabled         bool     `json:"site_enabled"`
	UserEnabled         bool     `json:"user_enabled"`
	ImpEnabled          bool     `json:"imp_enabled"`
	GlobalEnabled       bool     `json:"global_enabled"`
	BidderConfigEnabled bool     `json:"bidderconfig_enabled"`
	ContentEnabled      bool     `json:"content_enabled"`
	EIDsEnabled         bool     `json:"eids_enabled"`
	EIDSources          string   `json:"eid_sources"` // Comma-separated list
}

// GetFPDConfig retrieves FPD configuration from the IDR service
func (c *Client) GetFPDConfig(ctx context.Context) (*FPDConfig, error) {
	config, err := c.GetConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Extract FPD section from config
	fpdSection, ok := config["fpd"].(map[string]interface{})
	if !ok {
		// Return default config if no FPD section
		return &FPDConfig{
			Enabled:        true,
			SiteEnabled:    true,
			UserEnabled:    true,
			ImpEnabled:     true,
			ContentEnabled: true,
			EIDsEnabled:    true,
			EIDSources:     "liveramp.com,uidapi.com,id5-sync.com,criteo.com",
		}, nil
	}

	// Parse FPD config
	fpd := &FPDConfig{}

	if v, ok := fpdSection["enabled"].(bool); ok {
		fpd.Enabled = v
	}
	if v, ok := fpdSection["site_enabled"].(bool); ok {
		fpd.SiteEnabled = v
	}
	if v, ok := fpdSection["user_enabled"].(bool); ok {
		fpd.UserEnabled = v
	}
	if v, ok := fpdSection["imp_enabled"].(bool); ok {
		fpd.ImpEnabled = v
	}
	if v, ok := fpdSection["global_enabled"].(bool); ok {
		fpd.GlobalEnabled = v
	}
	if v, ok := fpdSection["bidderconfig_enabled"].(bool); ok {
		fpd.BidderConfigEnabled = v
	}
	if v, ok := fpdSection["content_enabled"].(bool); ok {
		fpd.ContentEnabled = v
	}
	if v, ok := fpdSection["eids_enabled"].(bool); ok {
		fpd.EIDsEnabled = v
	}
	if v, ok := fpdSection["eid_sources"].(string); ok {
		fpd.EIDSources = v
	}

	return fpd, nil
}
