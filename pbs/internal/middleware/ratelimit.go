package middleware

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond int           // Max requests per second per client
	BurstSize         int           // Max burst size
	CleanupInterval   time.Duration // How often to clean up old entries
	WindowSize        time.Duration // Time window for rate limiting
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	rps, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_RPS"))
	if rps <= 0 {
		rps = 1000 // Default: 1000 requests per second
	}

	burst, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_BURST"))
	if burst <= 0 {
		burst = 100 // Default burst size
	}

	return &RateLimitConfig{
		Enabled:           os.Getenv("RATE_LIMIT_ENABLED") == "true",
		RequestsPerSecond: rps,
		BurstSize:         burst,
		CleanupInterval:   time.Minute,
		WindowSize:        time.Second,
	}
}

// clientState tracks rate limit state for a single client
type clientState struct {
	tokens    float64
	lastCheck time.Time
}

// RateLimiter provides rate limiting middleware using token bucket algorithm
type RateLimiter struct {
	config  *RateLimitConfig
	clients map[string]*clientState
	mu      sync.Mutex
	stopCh  chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:  config,
		clients: make(map[string]*clientState),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// cleanup periodically removes stale client entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, state := range rl.clients {
				// Remove entries not seen in the last minute
				if now.Sub(state.lastCheck) > time.Minute {
					delete(rl.clients, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Middleware returns the rate limiting middleware handler
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get client identifier (prefer publisher ID from auth, fallback to IP)
		clientID := r.Header.Get("X-Publisher-ID")
		if clientID == "" {
			clientID = getClientIP(r)
		}

		// Check rate limit
		if !rl.allow(clientID) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerSecond))
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerSecond))

		next.ServeHTTP(w, r)
	})
}

// allow checks if a request from the given client should be allowed
func (rl *RateLimiter) allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	state, exists := rl.clients[clientID]

	if !exists {
		// New client, start with burst size tokens
		rl.clients[clientID] = &clientState{
			tokens:    float64(rl.config.BurstSize - 1), // -1 for current request
			lastCheck: now,
		}
		return true
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(state.lastCheck).Seconds()
	state.tokens += elapsed * float64(rl.config.RequestsPerSecond)

	// Cap at burst size
	if state.tokens > float64(rl.config.BurstSize) {
		state.tokens = float64(rl.config.BurstSize)
	}

	state.lastCheck = now

	// Check if we have tokens available
	if state.tokens < 1 {
		return false
	}

	state.tokens--
	return true
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (common for proxied requests)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	// Strip port if present
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// SetEnabled enables or disables rate limiting
func (rl *RateLimiter) SetEnabled(enabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.config.Enabled = enabled
}

// SetRPS sets the requests per second limit
func (rl *RateLimiter) SetRPS(rps int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.config.RequestsPerSecond = rps
}

// SetBurstSize sets the burst size
func (rl *RateLimiter) SetBurstSize(burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.config.BurstSize = burst
}
