// Package main is the entry point for the Prebid Server
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
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
)

func main() {
	// Parse flags
	port := flag.String("port", "8000", "Server port")
	idrURL := flag.String("idr-url", "http://localhost:5050", "IDR service URL")
	idrEnabled := flag.Bool("idr-enabled", true, "Enable IDR integration")
	timeout := flag.Duration("timeout", 1000*time.Millisecond, "Default auction timeout")
	flag.Parse()

	log.Printf("Starting The Nexus Engine PBS Server")
	log.Printf("  Port: %s", *port)
	log.Printf("  IDR URL: %s", *idrURL)
	log.Printf("  IDR Enabled: %v", *idrEnabled)
	log.Printf("  Timeout: %v", *timeout)

	// Initialize Prometheus metrics
	m := metrics.NewMetrics("pbs")
	log.Printf("  Metrics: enabled")

	// Initialize middleware
	auth := middleware.NewAuth(middleware.DefaultAuthConfig())
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())
	sizeLimiter := middleware.NewSizeLimiter(middleware.DefaultSizeLimitConfig())

	log.Printf("  Auth: %v", auth.IsEnabled())
	log.Printf("  Rate Limiting: %v", rateLimiter != nil)

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
	log.Printf("Registered bidders: %v", bidders)

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
		log.Printf("Server listening on :%s", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop rate limiter cleanup goroutine
	rateLimiter.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
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
