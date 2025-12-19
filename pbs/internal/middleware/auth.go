// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Redis key patterns (must match IDR's api_keys.py)
const (
	RedisAPIKeysHash = "nexus:api_keys" // hash: api_key -> publisher_id // #nosec G101 -- This is a Redis key name, not a credential
)

// RedisClient interface for API key validation
type RedisClient interface {
	HGet(ctx context.Context, key, field string) (string, error)
	Ping(ctx context.Context) error
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled       bool
	APIKeys       map[string]string // key -> publisher ID mapping (local fallback)
	HeaderName    string            // Header to check for API key (default: X-API-Key)
	BypassPaths   []string          // Paths that don't require auth (e.g., /health, /status)
	RedisURL      string            // Redis URL for shared API keys
	UseRedis      bool              // Whether to use Redis for API key validation
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() *AuthConfig {
	redisURL := os.Getenv("REDIS_URL")
	return &AuthConfig{
		Enabled:     os.Getenv("AUTH_ENABLED") == "true",
		APIKeys:     parseAPIKeys(os.Getenv("API_KEYS")),
		HeaderName:  "X-API-Key",
		BypassPaths: []string{"/health", "/status", "/metrics", "/info/bidders"},
		RedisURL:    redisURL,
		UseRedis:    redisURL != "" && os.Getenv("AUTH_USE_REDIS") != "false",
	}
}

// parseAPIKeys parses API keys from env var format: "key1:pub1,key2:pub2"
func parseAPIKeys(envValue string) map[string]string {
	keys := make(map[string]string)
	if envValue == "" {
		return keys
	}

	pairs := strings.Split(envValue, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			keys[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 && parts[0] != "" {
			// Key without publisher ID mapping
			keys[strings.TrimSpace(parts[0])] = "default"
		}
	}
	return keys
}

// Auth provides API key authentication middleware
type Auth struct {
	config      *AuthConfig
	redisClient RedisClient
	mu          sync.RWMutex
	// Cache for Redis lookups (reduces latency)
	keyCache     map[string]cachedKey
	cacheMu      sync.RWMutex
	cacheTimeout time.Duration
}

type cachedKey struct {
	publisherID string
	expiresAt   time.Time
}

// NewAuth creates a new Auth middleware
func NewAuth(config *AuthConfig) *Auth {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &Auth{
		config:       config,
		keyCache:     make(map[string]cachedKey),
		cacheTimeout: 60 * time.Second, // Cache keys for 60 seconds
	}
}

// NewAuthWithRedis creates Auth middleware with a Redis client
func NewAuthWithRedis(config *AuthConfig, redisClient RedisClient) *Auth {
	auth := NewAuth(config)
	auth.redisClient = redisClient
	return auth
}

// SetRedisClient sets the Redis client for API key validation
func (a *Auth) SetRedisClient(client RedisClient) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.redisClient = client
}

// Middleware returns the authentication middleware handler
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		config := a.config
		a.mu.RUnlock()

		// Skip auth if disabled
		if !config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check bypass paths
		for _, path := range config.BypassPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Get API key from header
		apiKey := r.Header.Get(config.HeaderName)
		if apiKey == "" {
			// Also check Authorization header with Bearer scheme
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
			return
		}

		// Validate API key
		publisherID, valid := a.validateKey(r.Context(), apiKey)
		if !valid {
			http.Error(w, `{"error":"invalid API key"}`, http.StatusForbidden)
			return
		}

		// Add publisher ID to request context via header (can be used downstream)
		r.Header.Set("X-Publisher-ID", publisherID)

		next.ServeHTTP(w, r)
	})
}

// validateKey checks if an API key is valid and returns the associated publisher ID
func (a *Auth) validateKey(ctx context.Context, key string) (string, bool) {
	// Check local cache first
	if pubID, found := a.checkCache(key); found {
		return pubID, pubID != ""
	}

	// Try Redis if available
	a.mu.RLock()
	redisClient := a.redisClient
	useRedis := a.config.UseRedis
	a.mu.RUnlock()

	if useRedis && redisClient != nil {
		pubID, err := redisClient.HGet(ctx, RedisAPIKeysHash, key)
		if err == nil && pubID != "" {
			a.updateCache(key, pubID)
			return pubID, true
		}
		if err != nil {
			log.Debug().Err(err).Msg("Redis API key lookup failed, falling back to local")
		}
	}

	// Fall back to local config
	a.mu.RLock()
	defer a.mu.RUnlock()

	for validKey, pubID := range a.config.APIKeys {
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			a.updateCache(key, pubID)
			return pubID, true
		}
	}

	// Cache negative result briefly to avoid hammering Redis
	a.updateCache(key, "")
	return "", false
}

// checkCache checks if a key is in the cache and still valid
func (a *Auth) checkCache(key string) (string, bool) {
	a.cacheMu.RLock()
	defer a.cacheMu.RUnlock()

	cached, exists := a.keyCache[key]
	if !exists {
		return "", false
	}

	if time.Now().After(cached.expiresAt) {
		return "", false
	}

	return cached.publisherID, true
}

// updateCache adds or updates a key in the cache
func (a *Auth) updateCache(key, publisherID string) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	// Use shorter timeout for negative results
	timeout := a.cacheTimeout
	if publisherID == "" {
		timeout = 10 * time.Second
	}

	a.keyCache[key] = cachedKey{
		publisherID: publisherID,
		expiresAt:   time.Now().Add(timeout),
	}
}

// ClearCache clears the API key cache
func (a *Auth) ClearCache() {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	a.keyCache = make(map[string]cachedKey)
}

// AddAPIKey adds a new API key at runtime
func (a *Auth) AddAPIKey(key, publisherID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config.APIKeys == nil {
		a.config.APIKeys = make(map[string]string)
	}
	a.config.APIKeys[key] = publisherID

	// Also update cache
	a.updateCache(key, publisherID)
}

// RemoveAPIKey removes an API key at runtime
func (a *Auth) RemoveAPIKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.config.APIKeys, key)

	// Clear from cache
	a.cacheMu.Lock()
	delete(a.keyCache, key)
	a.cacheMu.Unlock()
}

// SetEnabled enables or disables authentication
func (a *Auth) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.Enabled = enabled
}

// IsEnabled returns whether authentication is enabled
func (a *Auth) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.config.Enabled
}

// GetPublisherID returns the publisher ID for a given API key
func (a *Auth) GetPublisherID(ctx context.Context, key string) (string, bool) {
	return a.validateKey(ctx, key)
}
