// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PublisherAuthConfig holds publisher authentication configuration
type PublisherAuthConfig struct {
	Enabled            bool              // Enable publisher validation
	AllowUnregistered  bool              // Allow requests without publisher ID (for testing)
	RegisteredPubs     map[string]string // publisher_id -> allowed domains (comma-separated, empty = any)
	ValidateDomain     bool              // Validate request domain matches registered domains
	RateLimitPerPub    int               // Requests per second per publisher (0 = unlimited)
	UseRedis           bool              // Use Redis for publisher validation
}

// DefaultPublisherAuthConfig returns default config
func DefaultPublisherAuthConfig() *PublisherAuthConfig {
	return &PublisherAuthConfig{
		Enabled:           os.Getenv("PUBLISHER_AUTH_ENABLED") == "true",
		AllowUnregistered: os.Getenv("PUBLISHER_ALLOW_UNREGISTERED") == "true",
		RegisteredPubs:    parsePublishers(os.Getenv("REGISTERED_PUBLISHERS")),
		ValidateDomain:    os.Getenv("PUBLISHER_VALIDATE_DOMAIN") == "true",
		RateLimitPerPub:   100, // Default 100 RPS per publisher
		UseRedis:          os.Getenv("PUBLISHER_AUTH_USE_REDIS") != "false",
	}
}

// parsePublishers parses "pub1:domain1.com,pub2:domain2.com" format
func parsePublishers(envValue string) map[string]string {
	pubs := make(map[string]string)
	if envValue == "" {
		return pubs
	}

	pairs := strings.Split(envValue, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			pubs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 && parts[0] != "" {
			// Publisher without domain restriction
			pubs[strings.TrimSpace(parts[0])] = ""
		}
	}
	return pubs
}

// minimalBidRequest is a minimal struct for extracting publisher info
type minimalBidRequest struct {
	Site *struct {
		Domain    string `json:"domain"`
		Publisher *struct {
			ID string `json:"id"`
		} `json:"publisher"`
	} `json:"site"`
	App *struct {
		Bundle    string `json:"bundle"`
		Publisher *struct {
			ID string `json:"id"`
		} `json:"publisher"`
	} `json:"app"`
}

// PublisherAuth provides publisher authentication for auction endpoints
type PublisherAuth struct {
	config      *PublisherAuthConfig
	redisClient RedisClient
	mu          sync.RWMutex

	// Rate limiting per publisher
	rateLimits   map[string]*rateLimitEntry
	rateLimitsMu sync.RWMutex
}

type rateLimitEntry struct {
	tokens    float64
	lastCheck time.Time
}

// Redis key for registered publishers
const RedisPublishersHash = "nexus:publishers" // hash: publisher_id -> allowed_domains

// NewPublisherAuth creates a new publisher auth middleware
func NewPublisherAuth(config *PublisherAuthConfig) *PublisherAuth {
	if config == nil {
		config = DefaultPublisherAuthConfig()
	}
	return &PublisherAuth{
		config:     config,
		rateLimits: make(map[string]*rateLimitEntry),
	}
}

// SetRedisClient sets the Redis client for publisher validation
func (p *PublisherAuth) SetRedisClient(client RedisClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.redisClient = client
}

// Middleware returns the publisher authentication middleware handler
func (p *PublisherAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.RLock()
		enabled := p.config.Enabled
		p.mu.RUnlock()

		// Skip if disabled
		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Only apply to POST requests to auction endpoints
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/openrtb2/auction") {
			next.ServeHTTP(w, r)
			return
		}

		// Read and buffer the body so it can be re-read by the handler
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
			return
		}

		// Parse minimal request to extract publisher info
		var minReq minimalBidRequest
		if err := json.Unmarshal(body, &minReq); err != nil {
			// Let the main handler deal with invalid JSON
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Extract publisher ID
		publisherID, domain := p.extractPublisherInfo(&minReq)

		// Validate publisher
		if err := p.validatePublisher(r.Context(), publisherID, domain); err != nil {
			log.Warn().
				Str("publisher_id", publisherID).
				Str("domain", domain).
				Str("error", err.Error()).
				Msg("Publisher validation failed")
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusForbidden)
			return
		}

		// Apply rate limiting per publisher
		if publisherID != "" && !p.checkRateLimit(publisherID) {
			log.Warn().
				Str("publisher_id", publisherID).
				Msg("Publisher rate limit exceeded")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Add publisher ID to request context via header
		r.Header.Set("X-Publisher-ID", publisherID)

		// Restore body for handler
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// extractPublisherInfo extracts publisher ID and domain from request
func (p *PublisherAuth) extractPublisherInfo(req *minimalBidRequest) (publisherID, domain string) {
	if req.Site != nil {
		domain = req.Site.Domain
		if req.Site.Publisher != nil {
			publisherID = req.Site.Publisher.ID
		}
	} else if req.App != nil {
		domain = req.App.Bundle
		if req.App.Publisher != nil {
			publisherID = req.App.Publisher.ID
		}
	}
	return
}

// validatePublisher validates the publisher ID and domain
func (p *PublisherAuth) validatePublisher(ctx context.Context, publisherID, domain string) error {
	p.mu.RLock()
	allowUnregistered := p.config.AllowUnregistered
	validateDomain := p.config.ValidateDomain
	registeredPubs := p.config.RegisteredPubs
	useRedis := p.config.UseRedis
	redisClient := p.redisClient
	p.mu.RUnlock()

	// No publisher ID
	if publisherID == "" {
		if allowUnregistered {
			return nil
		}
		return &PublisherAuthError{Code: "missing_publisher", Message: "publisher ID required"}
	}

	// Check Redis first if available
	if useRedis && redisClient != nil {
		allowedDomains, err := redisClient.HGet(ctx, RedisPublishersHash, publisherID)
		if err == nil && allowedDomains != "" {
			if !validateDomain || allowedDomains == "*" || p.domainMatches(domain, allowedDomains) {
				return nil
			}
			return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
		}
		// Fall through to local config
	}

	// Check local config
	allowedDomains, registered := registeredPubs[publisherID]
	if !registered {
		if allowUnregistered {
			return nil
		}
		return &PublisherAuthError{Code: "unknown_publisher", Message: "publisher not registered"}
	}

	// Validate domain if required
	if validateDomain && allowedDomains != "" && allowedDomains != "*" {
		if !p.domainMatches(domain, allowedDomains) {
			return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
		}
	}

	return nil
}

// domainMatches checks if domain matches allowed domains (comma-separated)
func (p *PublisherAuth) domainMatches(domain, allowedDomains string) bool {
	if domain == "" {
		return false
	}

	for _, allowed := range strings.Split(allowedDomains, "|") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // ".example.com"
			if strings.HasSuffix(domain, suffix) || domain == allowed[2:] {
				return true
			}
		} else if domain == allowed {
			return true
		}
	}
	return false
}

// checkRateLimit implements token bucket rate limiting per publisher
func (p *PublisherAuth) checkRateLimit(publisherID string) bool {
	p.mu.RLock()
	rateLimit := p.config.RateLimitPerPub
	p.mu.RUnlock()

	if rateLimit <= 0 {
		return true // Unlimited
	}

	p.rateLimitsMu.Lock()
	defer p.rateLimitsMu.Unlock()

	entry, exists := p.rateLimits[publisherID]
	now := time.Now()

	if !exists {
		p.rateLimits[publisherID] = &rateLimitEntry{
			tokens:    float64(rateLimit) - 1,
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(entry.lastCheck).Seconds()
	entry.tokens += elapsed * float64(rateLimit)
	if entry.tokens > float64(rateLimit) {
		entry.tokens = float64(rateLimit)
	}
	entry.lastCheck = now

	// Try to consume a token
	if entry.tokens >= 1 {
		entry.tokens--
		return true
	}

	return false
}

// RegisterPublisher adds a publisher at runtime
func (p *PublisherAuth) RegisterPublisher(publisherID, allowedDomains string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.config.RegisteredPubs == nil {
		p.config.RegisteredPubs = make(map[string]string)
	}
	p.config.RegisteredPubs[publisherID] = allowedDomains
}

// UnregisterPublisher removes a publisher at runtime
func (p *PublisherAuth) UnregisterPublisher(publisherID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.config.RegisteredPubs, publisherID)
}

// SetEnabled enables or disables publisher authentication
func (p *PublisherAuth) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Enabled = enabled
}

// PublisherAuthError represents a publisher auth error
type PublisherAuthError struct {
	Code    string
	Message string
}

func (e *PublisherAuthError) Error() string {
	return e.Message
}
