package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSizeLimiterDisabled(t *testing.T) {
	sl := NewSizeLimiter(&SizeLimitConfig{
		Enabled:     false,
		MaxBodySize: 10,
	})

	handler := sl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Large body should be allowed when disabled
	body := bytes.NewReader(make([]byte, 1000))
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = 1000
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when disabled, got %d", rec.Code)
	}
}

func TestSizeLimiterMaxBodySize(t *testing.T) {
	sl := NewSizeLimiter(&SizeLimitConfig{
		Enabled:      true,
		MaxBodySize:  100,
		MaxURLLength: 1000, // Set a reasonable URL length limit
	})

	handler := sl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Small body should pass
	smallBody := bytes.NewReader(make([]byte, 50))
	req := httptest.NewRequest("POST", "/test", smallBody)
	req.ContentLength = 50
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("small body: expected 200, got %d", rec.Code)
	}

	// Large body should fail (Content-Length check)
	largeBody := bytes.NewReader(make([]byte, 200))
	req = httptest.NewRequest("POST", "/test", largeBody)
	req.ContentLength = 200
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large body: expected 413, got %d", rec.Code)
	}
}

func TestSizeLimiterMaxURLLength(t *testing.T) {
	sl := NewSizeLimiter(&SizeLimitConfig{
		Enabled:      true,
		MaxBodySize:  1024,
		MaxURLLength: 50,
	})

	handler := sl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Short URL should pass
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("short URL: expected 200, got %d", rec.Code)
	}

	// Long URL should fail
	longURL := "/test?" + strings.Repeat("a", 100)
	req = httptest.NewRequest("GET", longURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestURITooLong {
		t.Errorf("long URL: expected 414, got %d", rec.Code)
	}
}

func TestSizeLimiterBodyReader(t *testing.T) {
	sl := NewSizeLimiter(&SizeLimitConfig{
		Enabled:     true,
		MaxBodySize: 50,
	})

	var bodySize int
	handler := sl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		bodySize = n
		w.WriteHeader(http.StatusOK)
	}))

	// Body slightly over limit (without Content-Length header)
	body := bytes.NewReader(make([]byte, 100))
	req := httptest.NewRequest("POST", "/test", body)
	// Don't set Content-Length to trigger MaxBytesReader
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// MaxBytesReader should limit the read
	if bodySize > 50 {
		t.Errorf("expected body read to be limited to 50, got %d", bodySize)
	}
}

func TestSizeLimiterSetters(t *testing.T) {
	sl := NewSizeLimiter(&SizeLimitConfig{
		Enabled:      false,
		MaxBodySize:  100,
		MaxURLLength: 100,
	})

	sl.SetEnabled(true)
	sl.SetMaxBodySize(200)
	sl.SetMaxURLLength(500)

	if !sl.config.Enabled {
		t.Error("expected enabled to be true")
	}
	if sl.config.MaxBodySize != 200 {
		t.Errorf("expected max body size 200, got %d", sl.config.MaxBodySize)
	}
	if sl.config.MaxURLLength != 500 {
		t.Errorf("expected max URL length 500, got %d", sl.config.MaxURLLength)
	}
}

func TestDefaultSizeLimitConfig(t *testing.T) {
	config := DefaultSizeLimitConfig()

	if !config.Enabled {
		t.Error("expected size limiting to be enabled by default")
	}
	if config.MaxBodySize <= 0 {
		t.Error("expected positive max body size")
	}
	if config.MaxURLLength <= 0 {
		t.Error("expected positive max URL length")
	}
}
