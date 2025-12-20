// Package main is the entry point for the Prebid Server
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/appnexus"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/ortb"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/pubmatic"
	_ "github.com/StreetsDigital/thenexusengine/pbs/internal/adapters/rubicon"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/endpoints"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/exchange"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/metrics"
	"github.com/StreetsDigital/thenexusengine/pbs/internal/middleware"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
	"github.com/StreetsDigital/thenexusengine/pbs/pkg/redis"
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
	cors := middleware.NewCORS(middleware.DefaultCORSConfig())
	security := middleware.NewSecurity(nil) // Uses DefaultSecurityConfig()
	auth := middleware.NewAuth(middleware.DefaultAuthConfig())
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())
	sizeLimiter := middleware.NewSizeLimiter(middleware.DefaultSizeLimitConfig())
	gzipMiddleware := middleware.NewGzip(middleware.DefaultGzipConfig())

	log.Info().
		Bool("cors_enabled", true).
		Bool("security_headers_enabled", security.GetConfig().Enabled).
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

	// Initialize dynamic registry if Redis is available
	var dynamicRegistry *ortb.DynamicRegistry
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		redisClient, err := redis.New(redisURL)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to connect to Redis, dynamic bidders disabled")
		} else {
			dynamicRegistry = ortb.NewDynamicRegistry(redisClient, 30*time.Second)
			if err := dynamicRegistry.Start(context.Background()); err != nil {
				log.Warn().Err(err).Msg("Failed to start dynamic registry")
			} else {
				ex.SetDynamicRegistry(dynamicRegistry)
				log.Info().
					Int("dynamic_bidders", dynamicRegistry.Count()).
					Msg("Dynamic bidder registry initialized")
			}
		}
	} else {
		log.Info().Msg("REDIS_URL not set, dynamic bidders disabled")
	}

	// List registered bidders
	bidders := adapters.DefaultRegistry.ListBidders()
	log.Info().
		Int("count", len(bidders)).
		Strs("bidders", bidders).
		Msg("Static bidders registered")

	// Create handlers
	auctionHandler := endpoints.NewAuctionHandler(ex)
	statusHandler := endpoints.NewStatusHandler()
	// Use dynamic handler that queries registries at request time
	biddersHandler := endpoints.NewDynamicInfoBiddersHandler(adapters.DefaultRegistry, dynamicRegistry)

	// Cookie sync handlers
	hostURL := os.Getenv("PBS_HOST_URL")
	if hostURL == "" {
		hostURL = "https://nexus-pbs.fly.dev"
	}
	cookieSyncConfig := endpoints.DefaultCookieSyncConfig(hostURL)
	cookieSyncHandler := endpoints.NewCookieSyncHandler(cookieSyncConfig)
	setuidHandler := endpoints.NewSetUIDHandler(cookieSyncHandler.ListBidders())
	optoutHandler := endpoints.NewOptOutHandler()

	log.Info().
		Str("host_url", hostURL).
		Int("syncers", len(cookieSyncHandler.ListBidders())).
		Msg("Cookie sync initialized")

	// Setup routes
	mux := http.NewServeMux()
	mux.Handle("/openrtb2/auction", auctionHandler)
	mux.Handle("/status", statusHandler)
	mux.Handle("/health", healthHandler())
	mux.Handle("/info/bidders", biddersHandler)

	// Cookie sync endpoints
	mux.Handle("/cookie_sync", cookieSyncHandler)
	mux.Handle("/setuid", setuidHandler)
	mux.Handle("/optout", optoutHandler)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.Handler())

	// Admin endpoints for runtime configuration
	mux.HandleFunc("/admin/circuit-breaker", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if ex.GetIDRClient() != nil {
			stats := ex.GetIDRClient().CircuitBreakerStats()
			if err := json.NewEncoder(w).Encode(stats); err != nil {
				log.Error().Err(err).Msg("failed to encode circuit breaker stats")
			}
		} else {
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "IDR disabled"}); err != nil {
				log.Error().Err(err).Msg("failed to encode IDR disabled status")
			}
		}
	})

	// Build middleware chain: CORS -> Security -> Logging -> Size Limit -> Auth -> Rate Limit -> Metrics -> Gzip -> Handler
	// Note: CORS must be outermost to handle preflight OPTIONS requests
	// Note: Security headers applied early to ensure all responses have them
	// Note: Auth must run before Rate Limit so publisher ID is available for rate limiting
	// Note: Gzip is innermost so responses are compressed before being sent
	handler := http.Handler(mux)
	handler = gzipMiddleware.Middleware(handler) // Compress responses
	handler = m.Middleware(handler)
	handler = rateLimiter.Middleware(handler)
	handler = auth.Middleware(handler)
	handler = sizeLimiter.Middleware(handler)
	handler = loggingMiddleware(handler)
	handler = security.Middleware(handler)
	handler = cors.Middleware(handler)

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

	// Stop dynamic registry refresh goroutine
	if dynamicRegistry != nil {
		dynamicRegistry.Stop()
		log.Info().Msg("Dynamic registry stopped")
	}

	// Flush pending events from exchange
	if err := ex.Close(); err != nil {
		log.Warn().Err(err).Msg("Error flushing event recorder")
	} else {
		log.Info().Msg("Event recorder flushed")
	}

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
		if err := json.NewEncoder(w).Encode(health); err != nil {
			logger.Log.Error().Err(err).Msg("failed to encode health response")
		}
	})
}

// generateRequestID creates a unique request ID using cryptographically secure randomness
func generateRequestID() string {
	// Use 8 bytes (16 hex chars) for a good balance of uniqueness and brevity
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (should never happen)
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
