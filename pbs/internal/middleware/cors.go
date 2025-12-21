// Package middleware provides HTTP middleware components
package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
)

// CORSConfig configures CORS behavior
type CORSConfig struct {
	// AllowedOrigins is a list of origins that are allowed to make cross-origin requests.
	// Use "*" to allow all origins (not recommended for production).
	// Use specific domains like "https://publisher1.com" for security.
	AllowedOrigins []string
	// AllowCredentials indicates whether the request can include credentials.
	AllowCredentials bool
	// AllowedMethods specifies the methods allowed for cross-origin requests.
	AllowedMethods []string
	// AllowedHeaders specifies the headers allowed in cross-origin requests.
	AllowedHeaders []string
	// ExposedHeaders specifies headers that browsers are allowed to access.
	ExposedHeaders []string
	// MaxAge indicates how long preflight results can be cached (in seconds).
	MaxAge int
}

// DefaultCORSConfig returns a default CORS config for Prebid.js integration
func DefaultCORSConfig() CORSConfig {
	// Read allowed origins from environment
	originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	var origins []string
	if originsEnv != "" {
		origins = strings.Split(originsEnv, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
	}

	return CORSConfig{
		AllowedOrigins:   origins,
		AllowCredentials: false, // Prebid.js doesn't need credentials
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{
			"Content-Type",
			"Accept",
			"Origin",
			"X-Requested-With",
			"X-Prebid", // Prebid.js header
		},
		ExposedHeaders: []string{
			"X-Request-ID",
		},
		MaxAge: 86400, // 24 hours - preflight cache
	}
}

// CORS middleware handles Cross-Origin Resource Sharing
type CORS struct {
	config      CORSConfig
	originSet   map[string]bool
	allowAll    bool
	next        http.Handler
}

// NewCORS creates a new CORS middleware
func NewCORS(config CORSConfig) func(http.Handler) http.Handler {
	// Build origin lookup set for O(1) checks
	originSet := make(map[string]bool)
	allowAll := false
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			allowAll = true
		} else {
			originSet[origin] = true
		}
	}

	if len(config.AllowedOrigins) == 0 {
		logger.Log.Warn().Msg("CORS_ALLOWED_ORIGINS not set - CORS disabled")
	} else if allowAll {
		logger.Log.Warn().Msg("CORS configured with wildcard origin (*) - not recommended for production")
	} else {
		logger.Log.Info().
			Strs("origins", config.AllowedOrigins).
			Msg("CORS configured for specific origins")
	}

	return func(next http.Handler) http.Handler {
		return &CORS{
			config:    config,
			originSet: originSet,
			allowAll:  allowAll,
			next:      next,
		}
	}
}

// ServeHTTP implements http.Handler
func (c *CORS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// No Origin header = not a CORS request
	if origin == "" {
		c.next.ServeHTTP(w, r)
		return
	}

	// Check if origin is allowed
	if !c.isOriginAllowed(origin) {
		// Origin not allowed - don't set CORS headers
		// The browser will block the response
		logger.Log.Debug().
			Str("origin", origin).
			Str("path", r.URL.Path).
			Msg("CORS request from non-allowed origin")
		c.next.ServeHTTP(w, r)
		return
	}

	// Set CORS headers
	c.setCORSHeaders(w, origin)

	// Handle preflight (OPTIONS) request
	if r.Method == http.MethodOptions {
		c.handlePreflight(w, r)
		return
	}

	c.next.ServeHTTP(w, r)
}

// isOriginAllowed checks if the origin is in the allowed list
func (c *CORS) isOriginAllowed(origin string) bool {
	if c.allowAll {
		return true
	}
	return c.originSet[origin]
}

// setCORSHeaders sets the appropriate CORS response headers
func (c *CORS) setCORSHeaders(w http.ResponseWriter, origin string) {
	// Use the actual origin, not "*", for security
	w.Header().Set("Access-Control-Allow-Origin", origin)

	if c.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(c.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers",
			strings.Join(c.config.ExposedHeaders, ", "))
	}

	// Vary header is important for caching
	w.Header().Add("Vary", "Origin")
}

// handlePreflight handles OPTIONS preflight requests
func (c *CORS) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// Set preflight-specific headers
	if len(c.config.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods",
			strings.Join(c.config.AllowedMethods, ", "))
	}

	if len(c.config.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers",
			strings.Join(c.config.AllowedHeaders, ", "))
	}

	if c.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", itoa(c.config.MaxAge))
	}

	// Preflight requests should return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
