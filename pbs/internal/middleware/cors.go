// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string // Empty means allow all (*)
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds
}

// DefaultCORSConfig returns production-ready CORS configuration for Prebid.js
func DefaultCORSConfig() *CORSConfig {
	allowedOrigins := parseCommaSeparated(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return &CORSConfig{
		Enabled:        os.Getenv("CORS_ENABLED") != "false", // Enabled by default
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{
			"Content-Type",
			"X-API-Key",
			"X-Request-ID",
			"Authorization",
			"Accept",
			"Origin",
			"X-Prebid", // Prebid.js header
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-Prebid-Server-Version",
		},
		AllowCredentials: os.Getenv("CORS_ALLOW_CREDENTIALS") == "true",
		MaxAge:           86400, // 24 hours preflight cache
	}
}

// parseCommaSeparated splits a comma-separated string into a slice
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// CORS provides Cross-Origin Resource Sharing middleware
type CORS struct {
	config *CORSConfig
	mu     sync.RWMutex
}

// NewCORS creates a new CORS middleware
func NewCORS(config *CORSConfig) *CORS {
	if config == nil {
		config = DefaultCORSConfig()
	}
	return &CORS{config: config}
}

// Middleware returns the CORS middleware handler
func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Copy all needed config fields while holding the lock to prevent data race
		c.mu.RLock()
		enabled := c.config.Enabled
		allowedOrigins := c.config.AllowedOrigins
		exposedHeaders := c.config.ExposedHeaders
		allowCredentials := c.config.AllowCredentials
		allowedMethods := c.config.AllowedMethods
		allowedHeaders := c.config.AllowedHeaders
		maxAge := c.config.MaxAge
		c.mu.RUnlock()

		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Set CORS headers
		if c.isOriginAllowedWithList(origin, allowedOrigins) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(allowedOrigins) == 0 {
				// No specific origins configured - allow all
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		// Set Vary header to ensure proper caching
		w.Header().Add("Vary", "Origin")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			c.handlePreflightWithConfig(w, r, allowedMethods, allowedHeaders, allowCredentials, maxAge)
			return
		}

		// Set exposed headers for non-preflight requests
		if len(exposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposedHeaders, ", "))
		}

		// Set credentials header if allowed
		if allowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		next.ServeHTTP(w, r)
	})
}

// handlePreflightWithConfig handles OPTIONS preflight requests with pre-copied config values
func (c *CORS) handlePreflightWithConfig(w http.ResponseWriter, r *http.Request, allowedMethods, allowedHeaders []string, allowCredentials bool, maxAge int) {
	// Set allowed methods
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))

	// Set allowed headers
	requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestedHeaders != "" {
		// Echo back requested headers if they're in our allowed list, or allow all configured
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
	}

	// Set credentials header if allowed
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set max age for preflight cache
	if maxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
	}

	// Respond with 204 No Content for preflight
	w.WriteHeader(http.StatusNoContent)
}

// isOriginAllowedWithList checks if the origin is in the provided allowed list
func (c *CORS) isOriginAllowedWithList(origin string, allowedOrigins []string) bool {
	// If no specific origins configured, allow all
	if len(allowedOrigins) == 0 {
		return true
	}

	// Check against allowed origins
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains (e.g., "*.example.com")
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // ".example.com"
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}

	return false
}

// isOriginAllowed checks if the origin is in the allowed list (thread-safe)
func (c *CORS) isOriginAllowed(origin string) bool {
	c.mu.RLock()
	allowedOrigins := c.config.AllowedOrigins
	c.mu.RUnlock()
	return c.isOriginAllowedWithList(origin, allowedOrigins)
}

// SetEnabled enables or disables CORS
func (c *CORS) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Enabled = enabled
}

// SetAllowedOrigins updates the allowed origins list
func (c *CORS) SetAllowedOrigins(origins []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.AllowedOrigins = origins
}
