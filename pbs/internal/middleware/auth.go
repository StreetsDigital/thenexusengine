// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled       bool
	APIKeys       map[string]string // key -> publisher ID mapping
	HeaderName    string            // Header to check for API key (default: X-API-Key)
	BypassPaths   []string          // Paths that don't require auth (e.g., /health, /status)
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:     os.Getenv("AUTH_ENABLED") == "true",
		APIKeys:     parseAPIKeys(os.Getenv("API_KEYS")),
		HeaderName:  "X-API-Key",
		BypassPaths: []string{"/health", "/status", "/metrics", "/info/bidders"},
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
	config *AuthConfig
	mu     sync.RWMutex
}

// NewAuth creates a new Auth middleware
func NewAuth(config *AuthConfig) *Auth {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &Auth{config: config}
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
		publisherID, valid := a.validateKey(apiKey)
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
func (a *Auth) validateKey(key string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for validKey, pubID := range a.config.APIKeys {
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return pubID, true
		}
	}
	return "", false
}

// AddAPIKey adds a new API key at runtime
func (a *Auth) AddAPIKey(key, publisherID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config.APIKeys == nil {
		a.config.APIKeys = make(map[string]string)
	}
	a.config.APIKeys[key] = publisherID
}

// RemoveAPIKey removes an API key at runtime
func (a *Auth) RemoveAPIKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.config.APIKeys, key)
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
func (a *Auth) GetPublisherID(key string) (string, bool) {
	return a.validateKey(key)
}
