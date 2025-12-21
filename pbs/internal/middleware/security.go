// Package middleware provides HTTP middleware components
package middleware

import (
	"net/http"
)

// SecurityConfig configures security headers
type SecurityConfig struct {
	// EnableHSTS enables HTTP Strict Transport Security
	EnableHSTS bool
	// HSTSMaxAge is the max-age value for HSTS in seconds (default: 1 year)
	HSTSMaxAge int
	// FrameOptions controls X-Frame-Options (DENY, SAMEORIGIN, or empty to disable)
	FrameOptions string
	// ContentTypeNosniff enables X-Content-Type-Options: nosniff
	ContentTypeNosniff bool
	// XSSProtection enables X-XSS-Protection header
	XSSProtection bool
	// ReferrerPolicy sets the Referrer-Policy header
	ReferrerPolicy string
	// CSPPolicy sets Content-Security-Policy (empty to disable)
	CSPPolicy string
	// PermissionsPolicy sets Permissions-Policy header
	PermissionsPolicy string
}

// DefaultSecurityConfig returns secure defaults for an API server
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableHSTS:         true,
		HSTSMaxAge:         31536000, // 1 year
		FrameOptions:       "DENY",
		ContentTypeNosniff: true,
		XSSProtection:      true,
		ReferrerPolicy:     "strict-origin-when-cross-origin",
		// CSP for API responses - very restrictive
		CSPPolicy: "default-src 'none'; frame-ancestors 'none'",
		// Disable sensitive browser features
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders adds security headers to HTTP responses
type SecurityHeaders struct {
	config SecurityConfig
	next   http.Handler
}

// NewSecurityHeaders creates security headers middleware
func NewSecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &SecurityHeaders{
			config: config,
			next:   next,
		}
	}
}

// ServeHTTP implements http.Handler
func (s *SecurityHeaders) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set security headers before passing to next handler
	s.setSecurityHeaders(w)
	s.next.ServeHTTP(w, r)
}

// setSecurityHeaders adds all configured security headers
func (s *SecurityHeaders) setSecurityHeaders(w http.ResponseWriter) {
	// HSTS - only enable in production (when using HTTPS)
	if s.config.EnableHSTS && s.config.HSTSMaxAge > 0 {
		// Note: This header is ignored over HTTP, only effective over HTTPS
		w.Header().Set("Strict-Transport-Security",
			"max-age="+itoa(s.config.HSTSMaxAge)+"; includeSubDomains")
	}

	// X-Frame-Options - prevent clickjacking
	if s.config.FrameOptions != "" {
		w.Header().Set("X-Frame-Options", s.config.FrameOptions)
	}

	// X-Content-Type-Options - prevent MIME sniffing
	if s.config.ContentTypeNosniff {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}

	// X-XSS-Protection - legacy but still useful for older browsers
	if s.config.XSSProtection {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	}

	// Referrer-Policy - control referrer information
	if s.config.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", s.config.ReferrerPolicy)
	}

	// Content-Security-Policy - restrict resource loading
	if s.config.CSPPolicy != "" {
		w.Header().Set("Content-Security-Policy", s.config.CSPPolicy)
	}

	// Permissions-Policy - restrict browser features
	if s.config.PermissionsPolicy != "" {
		w.Header().Set("Permissions-Policy", s.config.PermissionsPolicy)
	}

	// Cache-Control for API responses - prevent caching of sensitive data
	// Individual handlers can override this for cacheable responses
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
