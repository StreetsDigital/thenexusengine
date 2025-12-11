// Package metrics provides Prometheus metrics for PBS
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Request metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Auction metrics
	AuctionsTotal       *prometheus.CounterVec
	AuctionDuration     *prometheus.HistogramVec
	BidsReceived        *prometheus.CounterVec
	BidCPM              *prometheus.HistogramVec
	BiddersSelected     *prometheus.HistogramVec
	BiddersExcluded     *prometheus.HistogramVec

	// Bidder metrics
	BidderRequests     *prometheus.CounterVec
	BidderLatency      *prometheus.HistogramVec
	BidderErrors       *prometheus.CounterVec
	BidderTimeouts     *prometheus.CounterVec

	// IDR metrics
	IDRRequests        *prometheus.CounterVec
	IDRLatency         *prometheus.HistogramVec
	IDRCircuitState    *prometheus.GaugeVec

	// Privacy metrics
	PrivacyFiltered    *prometheus.CounterVec
	ConsentSignals     *prometheus.CounterVec

	// System metrics
	ActiveConnections  prometheus.Gauge
	RateLimitRejected  prometheus.Counter
	AuthFailures       prometheus.Counter
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "pbs"
	}

	m := &Metrics{
		// Request metrics
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		RequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being served",
			},
		),

		// Auction metrics
		AuctionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auctions_total",
				Help:      "Total number of auctions",
			},
			[]string{"status", "media_type"},
		),
		AuctionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "auction_duration_seconds",
				Help:      "Auction duration in seconds",
				Buckets:   []float64{.01, .025, .05, .1, .25, .5, .75, 1, 1.5, 2},
			},
			[]string{"media_type"},
		),
		BidsReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bids_received_total",
				Help:      "Total number of bids received",
			},
			[]string{"bidder", "media_type"},
		),
		BidCPM: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bid_cpm",
				Help:      "Bid CPM distribution",
				Buckets:   []float64{0.1, 0.5, 1, 2, 3, 5, 10, 20, 50},
			},
			[]string{"bidder", "media_type"},
		),
		BiddersSelected: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bidders_selected",
				Help:      "Number of bidders selected per auction",
				Buckets:   []float64{1, 2, 3, 5, 7, 10, 15, 20, 30},
			},
			[]string{"media_type"},
		),
		BiddersExcluded: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bidders_excluded",
				Help:      "Number of bidders excluded per auction",
			},
			[]string{"reason"},
		),

		// Bidder metrics
		BidderRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bidder_requests_total",
				Help:      "Total requests to each bidder",
			},
			[]string{"bidder"},
		),
		BidderLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bidder_latency_seconds",
				Help:      "Bidder response latency in seconds",
				Buckets:   []float64{.01, .025, .05, .1, .15, .2, .3, .5, .75, 1},
			},
			[]string{"bidder"},
		),
		BidderErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bidder_errors_total",
				Help:      "Total errors from bidders",
			},
			[]string{"bidder", "error_type"},
		),
		BidderTimeouts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bidder_timeouts_total",
				Help:      "Total timeouts from bidders",
			},
			[]string{"bidder"},
		),

		// IDR metrics
		IDRRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "idr_requests_total",
				Help:      "Total requests to IDR service",
			},
			[]string{"status"},
		),
		IDRLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "idr_latency_seconds",
				Help:      "IDR service latency in seconds",
				Buckets:   []float64{.005, .01, .025, .05, .075, .1, .15, .2},
			},
			[]string{},
		),
		IDRCircuitState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "idr_circuit_breaker_state",
				Help:      "IDR circuit breaker state (0=closed, 1=open, 2=half-open)",
			},
			[]string{},
		),

		// Privacy metrics
		PrivacyFiltered: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "privacy_filtered_total",
				Help:      "Total bidders filtered due to privacy",
			},
			[]string{"bidder", "reason"},
		),
		ConsentSignals: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "consent_signals_total",
				Help:      "Consent signals received",
			},
			[]string{"type", "has_consent"},
		),

		// System metrics
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_connections",
				Help:      "Number of active connections",
			},
		),
		RateLimitRejected: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limit_rejected_total",
				Help:      "Total requests rejected due to rate limiting",
			},
		),
		AuthFailures: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_failures_total",
				Help:      "Total authentication failures",
			},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.RequestsInFlight,
		m.AuctionsTotal,
		m.AuctionDuration,
		m.BidsReceived,
		m.BidCPM,
		m.BiddersSelected,
		m.BiddersExcluded,
		m.BidderRequests,
		m.BidderLatency,
		m.BidderErrors,
		m.BidderTimeouts,
		m.IDRRequests,
		m.IDRLatency,
		m.IDRCircuitState,
		m.PrivacyFiltered,
		m.ConsentSignals,
		m.ActiveConnections,
		m.RateLimitRejected,
		m.AuthFailures,
	)

	return m
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware returns HTTP middleware that records request metrics
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.RequestsInFlight.Inc()
		defer m.RequestsInFlight.Dec()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)

		m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
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

// RecordAuction records auction metrics
func (m *Metrics) RecordAuction(status, mediaType string, duration time.Duration, biddersSelected, biddersExcluded int) {
	m.AuctionsTotal.WithLabelValues(status, mediaType).Inc()
	m.AuctionDuration.WithLabelValues(mediaType).Observe(duration.Seconds())
	m.BiddersSelected.WithLabelValues(mediaType).Observe(float64(biddersSelected))
}

// RecordBid records a bid received from a bidder
func (m *Metrics) RecordBid(bidder, mediaType string, cpm float64) {
	m.BidsReceived.WithLabelValues(bidder, mediaType).Inc()
	m.BidCPM.WithLabelValues(bidder, mediaType).Observe(cpm)
}

// RecordBidderRequest records a request to a bidder
func (m *Metrics) RecordBidderRequest(bidder string, latency time.Duration, hasError, timedOut bool) {
	m.BidderRequests.WithLabelValues(bidder).Inc()
	m.BidderLatency.WithLabelValues(bidder).Observe(latency.Seconds())

	if hasError {
		m.BidderErrors.WithLabelValues(bidder, "error").Inc()
	}
	if timedOut {
		m.BidderTimeouts.WithLabelValues(bidder).Inc()
	}
}

// RecordIDRRequest records an IDR service request
func (m *Metrics) RecordIDRRequest(status string, latency time.Duration) {
	m.IDRRequests.WithLabelValues(status).Inc()
	m.IDRLatency.WithLabelValues().Observe(latency.Seconds())
}

// SetIDRCircuitState sets the IDR circuit breaker state metric
func (m *Metrics) SetIDRCircuitState(state string) {
	var value float64
	switch state {
	case "closed":
		value = 0
	case "open":
		value = 1
	case "half-open":
		value = 2
	}
	m.IDRCircuitState.WithLabelValues().Set(value)
}

// RecordPrivacyFiltered records when a bidder is filtered for privacy reasons
func (m *Metrics) RecordPrivacyFiltered(bidder, reason string) {
	m.PrivacyFiltered.WithLabelValues(bidder, reason).Inc()
}

// RecordConsentSignal records a consent signal
func (m *Metrics) RecordConsentSignal(signalType string, hasConsent bool) {
	consent := "no"
	if hasConsent {
		consent = "yes"
	}
	m.ConsentSignals.WithLabelValues(signalType, consent).Inc()
}
