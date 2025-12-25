// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"net/http"
	"os"
	"strings"
	"sync"
)

// SecurityConfig holds security headers configuration
type SecurityConfig struct {
	// Enabled toggles all security headers
	Enabled bool

	// XFrameOptions prevents clickjacking (DENY, SAMEORIGIN, or ALLOW-FROM uri)
	XFrameOptions string

	// XContentTypeOptions prevents MIME-type sniffing (nosniff)
	XContentTypeOptions string

	// XXSSProtection enables XSS filter in older browsers
	XXSSProtection string

	// ContentSecurityPolicy controls resource loading
	ContentSecurityPolicy string

	// ReferrerPolicy controls referrer information
	ReferrerPolicy string

	// StrictTransportSecurity enables HSTS (only set when behind TLS proxy)
	StrictTransportSecurity string

	// PermissionsPolicy controls browser features
	PermissionsPolicy string

	// CacheControl for API responses
	CacheControl string
}

// DefaultSecurityConfig returns production-ready security headers
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled: os.Getenv("SECURITY_HEADERS_ENABLED") != "false", // Enabled by default

		// Prevent clickjacking - deny framing entirely for API
		XFrameOptions: envOrDefault("SECURITY_X_FRAME_OPTIONS", "DENY"),

		// Prevent MIME-type sniffing attacks
		XContentTypeOptions: "nosniff",

		// Enable XSS filter (legacy, but still useful for older browsers)
		XXSSProtection: "1; mode=block",

		// CSP for API responses - restrictive since we only serve JSON
		ContentSecurityPolicy: envOrDefault("SECURITY_CSP",
			"default-src 'none'; frame-ancestors 'none'"),

		// Don't leak referrer data
		ReferrerPolicy: envOrDefault("SECURITY_REFERRER_POLICY", "strict-origin-when-cross-origin"),

		// P1-6: HSTS enabled by default for HTTPS enforcement
		// Set SECURITY_HSTS="" to disable if not using TLS (development only)
		StrictTransportSecurity: envOrDefault("SECURITY_HSTS",
			"max-age=31536000; includeSubDomains"),

		// Disable unnecessary browser features for API
		PermissionsPolicy: envOrDefault("SECURITY_PERMISSIONS_POLICY",
			"geolocation=(), microphone=(), camera=()"),

		// API responses should not be cached by browsers
		CacheControl: envOrDefault("SECURITY_CACHE_CONTROL",
			"no-store, no-cache, must-revalidate, private"),
	}
}

// envOrDefault returns environment variable value or default
func envOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// Security provides security headers middleware
type Security struct {
	config *SecurityConfig
	mu     sync.RWMutex
}

// NewSecurity creates a new Security middleware
func NewSecurity(config *SecurityConfig) *Security {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	return &Security{config: config}
}

// Middleware returns the security headers middleware handler
func (s *Security) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Copy all needed config fields while holding the lock to prevent data race
		s.mu.RLock()
		enabled := s.config.Enabled
		xFrameOptions := s.config.XFrameOptions
		xContentTypeOptions := s.config.XContentTypeOptions
		xXSSProtection := s.config.XXSSProtection
		csp := s.config.ContentSecurityPolicy
		referrerPolicy := s.config.ReferrerPolicy
		hsts := s.config.StrictTransportSecurity
		permissionsPolicy := s.config.PermissionsPolicy
		cacheControl := s.config.CacheControl
		s.mu.RUnlock()

		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Set security headers
		h := w.Header()

		if xFrameOptions != "" {
			h.Set("X-Frame-Options", xFrameOptions)
		}

		if xContentTypeOptions != "" {
			h.Set("X-Content-Type-Options", xContentTypeOptions)
		}

		if xXSSProtection != "" {
			h.Set("X-XSS-Protection", xXSSProtection)
		}

		if csp != "" {
			h.Set("Content-Security-Policy", csp)
		}

		if referrerPolicy != "" {
			h.Set("Referrer-Policy", referrerPolicy)
		}

		if hsts != "" {
			h.Set("Strict-Transport-Security", hsts)
		}

		if permissionsPolicy != "" {
			h.Set("Permissions-Policy", permissionsPolicy)
		}

		if cacheControl != "" {
			// Only set cache control for non-static paths
			if !isStaticPath(r.URL.Path) {
				h.Set("Cache-Control", cacheControl)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// isStaticPath checks if path is for static content that can be cached
func isStaticPath(path string) bool {
	// Metrics endpoint can be cached briefly
	staticPaths := []string{"/metrics"}
	for _, p := range staticPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// SetEnabled enables or disables security headers
func (s *Security) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Enabled = enabled
}

// SetHSTS sets the HSTS header value
// Example: "max-age=31536000; includeSubDomains"
func (s *Security) SetHSTS(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.StrictTransportSecurity = value
}

// SetCSP sets the Content-Security-Policy header
func (s *Security) SetCSP(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.ContentSecurityPolicy = value
}

// GetConfig returns a copy of the current configuration
func (s *Security) GetConfig() SecurityConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.config
}
