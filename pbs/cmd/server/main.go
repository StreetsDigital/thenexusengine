// Package main is the entry point for the Prebid Server
package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/appnexus"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/pubmatic"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/rubicon"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/endpoints"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/exchange"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/metrics"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/middleware"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
	"github.com/rs/zerolog"
)

func main() {
	// Parse flags
	port := flag.String("port", "8000", "Server port")
	idrURL := flag.String("idr-url", "http://localhost:5050", "IDR service URL")
	idrEnabled := flag.Bool("idr-enabled", true, "Enable IDR integration")
	timeout := flag.Duration("timeout", 1000*time.Millisecond, "Default auction timeout")
	flag.Parse()

	// Initialize structured logger
	logger.Init(logger.DefaultConfig())
	log := logger.Log

	log.Info().
		Str("port", *port).
		Str("idr_url", *idrURL).
		Bool("idr_enabled", *idrEnabled).
		Dur("timeout", *timeout).
		Msg("Starting The Nexus Engine PBS Server")

	// Initialize Prometheus metrics
	m := metrics.NewMetrics("pbs")
	log.Info().Msg("Prometheus metrics enabled")

	// Initialize middleware
	auth := middleware.NewAuth(middleware.DefaultAuthConfig())
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())
	sizeLimiter := middleware.NewSizeLimiter(middleware.DefaultSizeLimitConfig())

	log.Info().
		Bool("auth_enabled", auth.IsEnabled()).
		Bool("rate_limiting_enabled", rateLimiter != nil).
		Msg("Middleware initialized")

	// Configure exchange
	config := &exchange.Config{
		DefaultTimeout:     *timeout,
		MaxBidders:         50,
		IDREnabled:         *idrEnabled,
		IDRServiceURL:      *idrURL,
		EventRecordEnabled: true,
		EventBufferSize:    100,
		CurrencyConv:       false,
		DefaultCurrency:    "USD",
	}

	// Create exchange with default registry
	ex := exchange.New(adapters.DefaultRegistry, config)

	// List registered bidders
	bidders := adapters.DefaultRegistry.ListBidders()
	log.Info().
		Int("count", len(bidders)).
		Strs("bidders", bidders).
		Msg("Bidders registered")

	// Create handlers
	auctionHandler := endpoints.NewAuctionHandler(ex)
	statusHandler := endpoints.NewStatusHandler()
	biddersHandler := endpoints.NewInfoBiddersHandler(bidders)

	// Setup routes
	mux := http.NewServeMux()
	mux.Handle("/openrtb2/auction", auctionHandler)
	mux.Handle("/status", statusHandler)
	mux.Handle("/health", healthHandler())
	mux.Handle("/info/bidders", biddersHandler)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.Handler())

	// Admin endpoints for runtime configuration
	mux.HandleFunc("/admin/circuit-breaker", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if ex.GetIDRClient() != nil {
			stats := ex.GetIDRClient().CircuitBreakerStats()
			json.NewEncoder(w).Encode(stats)
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "IDR disabled"})
		}
	})

	// Build middleware chain: Size Limit -> Rate Limit -> Auth -> Metrics -> Handler
	handler := http.Handler(mux)
	handler = m.Middleware(handler)
	handler = auth.Middleware(handler)
	handler = rateLimiter.Middleware(handler)
	handler = sizeLimiter.Middleware(handler)
	handler = loggingMiddleware(handler)

	// Create server
	server := &http.Server{
		Addr:         ":" + *port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().Str("addr", ":"+*port).Msg("Server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server error")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")

	// Stop rate limiter cleanup goroutine
	rateLimiter.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped gracefully")
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs HTTP requests with structured logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add request ID to response
		w.Header().Set("X-Request-ID", requestID)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request completion
		duration := time.Since(start)

		event := logger.Log.Info()
		if wrapped.statusCode >= 400 {
			event = logger.Log.Warn()
		}
		if wrapped.statusCode >= 500 {
			event = logger.Log.Error()
		}

		event.
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("duration_ms", duration).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("HTTP request")
	})
}

// healthHandler returns a comprehensive health check
func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	return zerolog.TimestampFunc().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
