package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterDisabled(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled: false,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should allow unlimited requests when disabled
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

func TestRateLimiterBasic(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 5,
		BurstSize:         5,
		WindowSize:        time.Second,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 requests should succeed (burst)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("burst request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst exhausted, got %d", rec.Code)
	}

	// Check headers
	if rec.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After: 1, got %s", rec.Header().Get("Retry-After"))
	}
}

func TestRateLimiterDifferentClients(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 2,
		BurstSize:         2,
		WindowSize:        time.Second,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Client 1 exhausts burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Client 1 should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Error("client 1 should be rate limited")
	}

	// Client 2 should still work
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("client 2 should not be rate limited, got %d", rec.Code)
	}
}

func TestRateLimiterUsesPublisherID(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 2,
		BurstSize:         2,
		WindowSize:        time.Second,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust publisher1's limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Publisher-ID", "publisher1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// publisher1 should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Publisher-ID", "publisher1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Error("publisher1 should be rate limited")
	}

	// publisher2 should work
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Publisher-ID", "publisher2")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("publisher2 should not be rate limited, got %d", rec.Code)
	}
}

func TestRateLimiterTokenRefill(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         2,
		WindowSize:        time.Second,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Wait for tokens to refill (100ms should give ~1 token at 10 RPS)
	time.Sleep(150 * time.Millisecond)

	// Should be able to make another request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected token refill to allow request, got %d", rec.Code)
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 1000,
		BurstSize:         100,
		WindowSize:        time.Second,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	// Run 100 concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
		}()
	}

	wg.Wait()

	// Should have at most burst size successes
	if successCount > 100 {
		t.Errorf("expected at most 100 successes, got %d", successCount)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		trustXFF   bool
		expected   string
	}{
		// Without trusted proxies, always use RemoteAddr
		{"RemoteAddr with port", "192.168.1.1:12345", "", "", false, "192.168.1.1"},
		{"RemoteAddr without port", "192.168.1.1", "", "", false, "192.168.1.1"},
		{"XFF ignored without trust", "192.168.1.1:12345", "10.0.0.1", "", false, "192.168.1.1"},
		// With trusted proxies configured, XFF is used
		{"X-Forwarded-For with trust", "127.0.0.1:12345", "10.0.0.1", "", true, "10.0.0.1"},
		{"X-Real-IP with trust", "127.0.0.1:12345", "", "10.0.0.1", true, "10.0.0.1"},
		{"XFF takes precedence with trust", "127.0.0.1:12345", "10.0.0.1", "10.0.0.2", true, "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config *RateLimitConfig
			if tt.trustXFF {
				// Trust localhost as proxy
				_, loopback, _ := net.ParseCIDR("127.0.0.1/32")
				config = &RateLimitConfig{
					TrustedProxies: []*net.IPNet{loopback},
					TrustXFF:       true,
				}
			} else {
				config = &RateLimitConfig{
					TrustXFF: false,
				}
			}

			rl := &RateLimiter{config: config}

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			result := rl.getClientIP(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRateLimiterSetters(t *testing.T) {
	rl := NewRateLimiter(&RateLimitConfig{
		Enabled:           false,
		RequestsPerSecond: 10,
		BurstSize:         5,
	})
	defer rl.Stop()

	rl.SetEnabled(true)
	rl.SetRPS(100)
	rl.SetBurstSize(50)

	if rl.config.Enabled != true {
		t.Error("expected enabled to be true")
	}
	if rl.config.RequestsPerSecond != 100 {
		t.Errorf("expected RPS 100, got %d", rl.config.RequestsPerSecond)
	}
	if rl.config.BurstSize != 50 {
		t.Errorf("expected burst 50, got %d", rl.config.BurstSize)
	}
}
