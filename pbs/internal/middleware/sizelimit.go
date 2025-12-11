package middleware

import (
	"net/http"
	"os"
	"strconv"
)

// SizeLimitConfig holds request size limit configuration
type SizeLimitConfig struct {
	Enabled      bool
	MaxBodySize  int64 // Max request body size in bytes
	MaxURLLength int   // Max URL length
}

// DefaultSizeLimitConfig returns default size limit configuration
func DefaultSizeLimitConfig() *SizeLimitConfig {
	maxBody, _ := strconv.ParseInt(os.Getenv("MAX_REQUEST_SIZE"), 10, 64)
	if maxBody <= 0 {
		maxBody = 1024 * 1024 // Default: 1MB
	}

	maxURL, _ := strconv.Atoi(os.Getenv("MAX_URL_LENGTH"))
	if maxURL <= 0 {
		maxURL = 8192 // Default: 8KB
	}

	return &SizeLimitConfig{
		Enabled:      true, // Enabled by default for security
		MaxBodySize:  maxBody,
		MaxURLLength: maxURL,
	}
}

// SizeLimiter provides request size limiting middleware
type SizeLimiter struct {
	config *SizeLimitConfig
}

// NewSizeLimiter creates a new size limiter
func NewSizeLimiter(config *SizeLimitConfig) *SizeLimiter {
	if config == nil {
		config = DefaultSizeLimitConfig()
	}
	return &SizeLimiter{config: config}
}

// Middleware returns the size limiting middleware handler
func (sl *SizeLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sl.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check URL length
		if len(r.URL.String()) > sl.config.MaxURLLength {
			http.Error(w, `{"error":"URL too long"}`, http.StatusRequestURITooLong)
			return
		}

		// Check Content-Length header if present
		if r.ContentLength > sl.config.MaxBodySize {
			http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}

		// Wrap body with size limit reader
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, sl.config.MaxBodySize)
		}

		next.ServeHTTP(w, r)
	})
}

// SetMaxBodySize sets the max body size
func (sl *SizeLimiter) SetMaxBodySize(size int64) {
	sl.config.MaxBodySize = size
}

// SetMaxURLLength sets the max URL length
func (sl *SizeLimiter) SetMaxURLLength(length int) {
	sl.config.MaxURLLength = length
}

// SetEnabled enables or disables size limiting
func (sl *SizeLimiter) SetEnabled(enabled bool) {
	sl.config.Enabled = enabled
}
