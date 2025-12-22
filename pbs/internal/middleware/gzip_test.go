package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddleware_CompressesJSON(t *testing.T) {
	gz := NewGzip(DefaultGzipConfig())

	// Create a handler that returns JSON larger than MinLength (256 bytes)
	// This response is ~350 bytes
	jsonResponse := `{"id":"test-auction-123","cur":"USD","seatbid":[{"bid":[{"id":"bid-1","impid":"imp-1","price":2.50,"adm":"<html><body>This is a test ad creative with enough content to exceed the minimum compression threshold of 256 bytes</body></html>","adomain":["example.com"],"crid":"creative-123"}]}]}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))
	})

	// Wrap with gzip middleware
	wrapped := gz.Middleware(handler)

	// Create request with Accept-Encoding: gzip
	req := httptest.NewRequest("GET", "/openrtb2/auction", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Verify Content-Encoding is gzip
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got: %s", rec.Header().Get("Content-Encoding"))
	}

	// Verify Vary header includes Accept-Encoding
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Errorf("Expected Vary to include Accept-Encoding, got: %s", rec.Header().Get("Vary"))
	}

	// Decompress and verify content
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if string(decompressed) != jsonResponse {
		t.Errorf("Decompressed content mismatch.\nExpected: %s\nGot: %s", jsonResponse, string(decompressed))
	}
}

func TestGzipMiddleware_SkipsWithoutAcceptEncoding(t *testing.T) {
	gz := NewGzip(DefaultGzipConfig())

	// Large enough response to normally be compressed (>256 bytes)
	jsonResponse := `{"id":"test-auction-123","cur":"USD","seatbid":[{"bid":[{"id":"bid-1","impid":"imp-1","price":2.50,"adm":"<html><body>This is a test ad creative with enough content to exceed the minimum compression threshold of 256 bytes</body></html>","adomain":["example.com"],"crid":"creative-123"}]}]}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))
	})

	wrapped := gz.Middleware(handler)

	// Request without Accept-Encoding
	req := httptest.NewRequest("GET", "/openrtb2/auction", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Should NOT be compressed
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Expected no gzip encoding when Accept-Encoding not set")
	}

	// Content should be plain
	if rec.Body.String() != jsonResponse {
		t.Errorf("Content mismatch.\nExpected: %s\nGot: %s", jsonResponse, rec.Body.String())
	}
}

func TestGzipMiddleware_SkipsExcludedPaths(t *testing.T) {
	gz := NewGzip(DefaultGzipConfig())

	response := `{"status":"healthy"}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	})

	wrapped := gz.Middleware(handler)

	excludedPaths := []string{"/metrics", "/health", "/status"}
	for _, path := range excludedPaths {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") == "gzip" {
			t.Errorf("Path %s should not be compressed", path)
		}
	}
}

func TestGzipMiddleware_SkipsSmallResponses(t *testing.T) {
	config := DefaultGzipConfig()
	config.MinLength = 256
	gz := NewGzip(config)

	// Response smaller than MinLength
	smallResponse := `{"ok":true}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(smallResponse))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/openrtb2/auction", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Small responses should not set Content-Encoding (though the header check happens before write)
	// The key is that the body should be readable as plain text
	body := rec.Body.String()
	if body != smallResponse {
		t.Errorf("Small response should not be compressed.\nExpected: %s\nGot: %s", smallResponse, body)
	}
}

func TestGzipMiddleware_Disabled(t *testing.T) {
	config := DefaultGzipConfig()
	config.Enabled = false
	gz := NewGzip(config)

	// Large enough response to normally be compressed (>256 bytes)
	jsonResponse := `{"id":"test-auction-123","cur":"USD","seatbid":[{"bid":[{"id":"bid-1","impid":"imp-1","price":2.50,"adm":"<html><body>This is a test ad creative with enough content to exceed the minimum compression threshold of 256 bytes</body></html>","adomain":["example.com"],"crid":"creative-123"}]}]}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/openrtb2/auction", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Gzip should not be applied when disabled")
	}

	if rec.Body.String() != jsonResponse {
		t.Errorf("Content mismatch.\nExpected: %s\nGot: %s", jsonResponse, rec.Body.String())
	}
}

func TestGzipMiddleware_SkipsNonCompressibleTypes(t *testing.T) {
	gz := NewGzip(DefaultGzipConfig())

	// Image-like content type should not be compressed
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 100))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Non-compressible content types should not be compressed")
	}
}

func TestGzipMiddleware_CompressionLevel(t *testing.T) {
	// Test with best compression
	config := DefaultGzipConfig()
	config.Level = 9
	gz := NewGzip(config)

	// Large response to see compression effect
	largeResponse := strings.Repeat(`{"id":"test","value":12345},`, 100)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeResponse))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/openrtb2/auction", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Compressed should be smaller than original
	if rec.Body.Len() >= len(largeResponse) {
		t.Errorf("Compressed size (%d) should be smaller than original (%d)", rec.Body.Len(), len(largeResponse))
	}
}

func TestGzipMiddleware_InvalidLevel(t *testing.T) {
	// Invalid level should default to 6
	config := &GzipConfig{
		Enabled:      true,
		MinLength:    256,
		Level:        15, // Invalid - should default to 6
		ContentTypes: []string{"application/json"},
	}
	gz := NewGzip(config)

	// Just verify it doesn't panic
	largeResponse := strings.Repeat(`{"test":true},`, 50)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeResponse))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Should still compress with defaulted level")
	}
}

func TestDefaultGzipConfig(t *testing.T) {
	config := DefaultGzipConfig()

	if !config.Enabled {
		t.Error("Default config should be enabled")
	}
	if config.MinLength != 256 {
		t.Errorf("Expected MinLength 256, got %d", config.MinLength)
	}
	if config.Level != 6 {
		t.Errorf("Expected Level 6, got %d", config.Level)
	}
	if len(config.ContentTypes) != 3 {
		t.Errorf("Expected 3 content types, got %d", len(config.ContentTypes))
	}
	if len(config.ExcludedPaths) != 3 {
		t.Errorf("Expected 3 excluded paths, got %d", len(config.ExcludedPaths))
	}
}

func TestGzipMiddleware_NilConfig(t *testing.T) {
	// Should use defaults when nil config passed
	gz := NewGzip(nil)

	largeResponse := strings.Repeat(`{"test":true},`, 50)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeResponse))
	})

	wrapped := gz.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Should compress with default config")
	}
}
