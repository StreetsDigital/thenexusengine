// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"net/http"
	"os"
	"strings"
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
		if !c.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Set CORS headers
		if c.isOriginAllowed(origin) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(c.config.AllowedOrigins) == 0 {
				// No specific origins configured - allow all
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		// Set Vary header to ensure proper caching
		w.Header().Add("Vary", "Origin")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			c.handlePreflight(w, r)
			return
		}

		// Set exposed headers for non-preflight requests
		if len(c.config.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.config.ExposedHeaders, ", "))
		}

		// Set credentials header if allowed
		if c.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		next.ServeHTTP(w, r)
	})
}

// handlePreflight handles OPTIONS preflight requests
func (c *CORS) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// Set allowed methods
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.config.AllowedMethods, ", "))

	// Set allowed headers
	requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestedHeaders != "" {
		// Echo back requested headers if they're in our allowed list, or allow all configured
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.AllowedHeaders, ", "))
	}

	// Set credentials header if allowed
	if c.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set max age for preflight cache
	if c.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", formatInt(c.config.MaxAge))
	}

	// Respond with 204 No Content for preflight
	w.WriteHeader(http.StatusNoContent)
}

// isOriginAllowed checks if the origin is in the allowed list
func (c *CORS) isOriginAllowed(origin string) bool {
	// If no specific origins configured, allow all
	if len(c.config.AllowedOrigins) == 0 {
		return true
	}

	// Check against allowed origins
	for _, allowed := range c.config.AllowedOrigins {
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

// formatInt converts int to string without importing strconv
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// SetEnabled enables or disables CORS
func (c *CORS) SetEnabled(enabled bool) {
	c.config.Enabled = enabled
}

// SetAllowedOrigins updates the allowed origins list
func (c *CORS) SetAllowedOrigins(origins []string) {
	c.config.AllowedOrigins = origins
}
