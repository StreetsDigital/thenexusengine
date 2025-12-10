package adapters

import (
	"fmt"
	"sync"
)

// Registry holds all registered bidder adapters
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]AdapterWithInfo
}

// NewRegistry creates a new adapter registry
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]AdapterWithInfo),
	}
}

// Register adds a bidder adapter to the registry
func (r *Registry) Register(bidderCode string, adapter Adapter, info BidderInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[bidderCode]; exists {
		return fmt.Errorf("adapter already registered: %s", bidderCode)
	}

	r.adapters[bidderCode] = AdapterWithInfo{
		Adapter: adapter,
		Info:    info,
	}
	return nil
}

// Get retrieves an adapter by bidder code
func (r *Registry) Get(bidderCode string) (AdapterWithInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[bidderCode]
	return adapter, ok
}

// GetAll returns all registered adapters
func (r *Registry) GetAll() map[string]AdapterWithInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]AdapterWithInfo, len(r.adapters))
	for k, v := range r.adapters {
		result[k] = v
	}
	return result
}

// ListBidders returns all registered bidder codes
func (r *Registry) ListBidders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bidders := make([]string, 0, len(r.adapters))
	for code := range r.adapters {
		bidders = append(bidders, code)
	}
	return bidders
}

// ListEnabledBidders returns enabled bidder codes
func (r *Registry) ListEnabledBidders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bidders := make([]string, 0)
	for code, awi := range r.adapters {
		if awi.Info.Enabled {
			bidders = append(bidders, code)
		}
	}
	return bidders
}

// DefaultRegistry is the global adapter registry
var DefaultRegistry = NewRegistry()

// RegisterAdapter is a convenience function to register with the default registry
func RegisterAdapter(bidderCode string, adapter Adapter, info BidderInfo) error {
	return DefaultRegistry.Register(bidderCode, adapter, info)
}
