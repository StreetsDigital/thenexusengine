// Package container provides edge compute/container hook infrastructure
// for the Nexus Engine PBS. This allows publishers to inject custom logic
// at various points in the bid request/response lifecycle.
package container

import (
	"context"
	"time"
)

// HookType represents the type of container hook
type HookType string

const (
	// HookTypePreRequest is called before building the OpenRTB request
	HookTypePreRequest HookType = "pre_request"

	// HookTypePostResponse is called after receiving server response
	HookTypePostResponse HookType = "post_response"

	// HookTypeBidTransform is called for each interpreted bid
	HookTypeBidTransform HookType = "bid_transform"

	// HookTypeOnWin is called when a bid wins
	HookTypeOnWin HookType = "on_win"

	// HookTypeOnTimeout is called on bid timeout
	HookTypeOnTimeout HookType = "on_timeout"
)

// HookConfig defines configuration for a container hook
type HookConfig struct {
	// Type of hook
	Type HookType `json:"type"`

	// Name is a unique identifier for this hook
	Name string `json:"name"`

	// Enabled determines if the hook is active
	Enabled bool `json:"enabled"`

	// Priority determines execution order (lower = first)
	Priority int `json:"priority"`

	// Timeout for hook execution
	Timeout time.Duration `json:"timeout"`

	// Container configuration
	Container ContainerConfig `json:"container"`
}

// ContainerConfig defines the container/function to execute
type ContainerConfig struct {
	// Runtime specifies the execution environment
	// Options: "wasm", "docker", "lambda", "cloudflare", "fastly", "local"
	Runtime string `json:"runtime"`

	// Image is the container image (for docker runtime)
	Image string `json:"image,omitempty"`

	// Function is the function name (for serverless runtimes)
	Function string `json:"function,omitempty"`

	// URL is the endpoint to call (for HTTP-based execution)
	URL string `json:"url,omitempty"`

	// WasmModule is the path to WASM binary (for wasm runtime)
	WasmModule string `json:"wasm_module,omitempty"`

	// Environment variables
	Env map[string]string `json:"env,omitempty"`

	// Memory limit in MB
	MemoryLimit int `json:"memory_limit,omitempty"`

	// CPU limit (0.1 = 10% of one core)
	CPULimit float64 `json:"cpu_limit,omitempty"`
}

// HookContext provides context for hook execution
type HookContext struct {
	// RequestID is the unique request identifier
	RequestID string `json:"request_id"`

	// AccountID is the publisher account
	AccountID string `json:"account_id"`

	// Timestamp of the request
	Timestamp time.Time `json:"timestamp"`

	// SourceIP of the request (if available)
	SourceIP string `json:"source_ip,omitempty"`

	// UserAgent of the request
	UserAgent string `json:"user_agent,omitempty"`

	// Custom metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HookInput is the input to a container hook
type HookInput struct {
	// Context provides execution context
	Context HookContext `json:"context"`

	// Data is the hook-specific payload
	Data interface{} `json:"data"`
}

// HookOutput is the output from a container hook
type HookOutput struct {
	// Success indicates if the hook executed successfully
	Success bool `json:"success"`

	// Data is the transformed payload
	Data interface{} `json:"data,omitempty"`

	// Error message if Success is false
	Error string `json:"error,omitempty"`

	// Metrics from hook execution
	Metrics *HookMetrics `json:"metrics,omitempty"`
}

// HookMetrics contains execution metrics
type HookMetrics struct {
	// Duration of hook execution
	Duration time.Duration `json:"duration"`

	// MemoryUsed in bytes
	MemoryUsed int64 `json:"memory_used,omitempty"`

	// CPUTime consumed
	CPUTime time.Duration `json:"cpu_time,omitempty"`
}

// PreRequestInput is the input for pre-request hooks
type PreRequestInput struct {
	// BidRequest is the OpenRTB bid request
	BidRequest map[string]interface{} `json:"bid_request"`

	// Publisher information
	Publisher PublisherInfo `json:"publisher"`
}

// PostResponseInput is the input for post-response hooks
type PostResponseInput struct {
	// BidResponse is the OpenRTB bid response
	BidResponse map[string]interface{} `json:"bid_response"`

	// OriginalRequest is the original bid request
	OriginalRequest map[string]interface{} `json:"original_request"`
}

// BidTransformInput is the input for bid transform hooks
type BidTransformInput struct {
	// Bid is the individual bid to transform
	Bid map[string]interface{} `json:"bid"`

	// ImpID is the impression ID
	ImpID string `json:"imp_id"`

	// Seat is the bidder seat
	Seat string `json:"seat"`
}

// WinInput is the input for on-win hooks
type WinInput struct {
	// WinningBid details
	WinningBid map[string]interface{} `json:"winning_bid"`

	// Price is the winning price
	Price float64 `json:"price"`

	// Currency of the price
	Currency string `json:"currency"`

	// AdUnitCode identifies the ad unit
	AdUnitCode string `json:"ad_unit_code"`
}

// TimeoutInput is the input for on-timeout hooks
type TimeoutInput struct {
	// BidderCode of the timed out bidder
	BidderCode string `json:"bidder_code"`

	// Timeout duration
	Timeout time.Duration `json:"timeout"`

	// AuctionID of the timed out auction
	AuctionID string `json:"auction_id"`
}

// PublisherInfo contains publisher context
type PublisherInfo struct {
	// AccountID is the publisher account
	AccountID string `json:"account_id"`

	// SiteID is the site identifier
	SiteID string `json:"site_id,omitempty"`

	// Domain of the publisher
	Domain string `json:"domain,omitempty"`
}

// Hook is the interface that all container hooks must implement
type Hook interface {
	// Type returns the hook type
	Type() HookType

	// Name returns the hook name
	Name() string

	// Execute runs the hook with the given input
	Execute(ctx context.Context, input *HookInput) (*HookOutput, error)

	// IsEnabled returns whether the hook is enabled
	IsEnabled() bool
}

// HookRegistry manages registered hooks
type HookRegistry interface {
	// Register adds a hook to the registry
	Register(hook Hook) error

	// Unregister removes a hook from the registry
	Unregister(name string) error

	// GetHooks returns all hooks of the given type, sorted by priority
	GetHooks(hookType HookType) []Hook

	// ExecuteHooks runs all hooks of the given type
	ExecuteHooks(ctx context.Context, hookType HookType, input *HookInput) (*HookOutput, error)
}

// Runtime is the interface for container runtimes
type Runtime interface {
	// Name returns the runtime name
	Name() string

	// Execute runs the container with the given input
	Execute(ctx context.Context, config *ContainerConfig, input *HookInput) (*HookOutput, error)

	// IsAvailable checks if the runtime is available
	IsAvailable() bool
}
