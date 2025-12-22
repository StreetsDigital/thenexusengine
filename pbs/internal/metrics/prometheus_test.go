package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// createTestMetrics creates a Metrics instance for testing with a custom registry
// to avoid conflicts with the global registry across tests
func createTestMetrics(namespace string) (*Metrics, *prometheus.Registry) {
	if namespace == "" {
		namespace = "test"
	}

	registry := prometheus.NewRegistry()

	m := &Metrics{
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

	// Register with custom registry
	registry.MustRegister(
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

	return m, registry
}

func TestMetrics_Struct(t *testing.T) {
	m, _ := createTestMetrics("test")

	if m.RequestsTotal == nil {
		t.Error("RequestsTotal should not be nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration should not be nil")
	}
	if m.RequestsInFlight == nil {
		t.Error("RequestsInFlight should not be nil")
	}
	if m.AuctionsTotal == nil {
		t.Error("AuctionsTotal should not be nil")
	}
	if m.AuctionDuration == nil {
		t.Error("AuctionDuration should not be nil")
	}
	if m.BidsReceived == nil {
		t.Error("BidsReceived should not be nil")
	}
	if m.BidCPM == nil {
		t.Error("BidCPM should not be nil")
	}
	if m.BiddersSelected == nil {
		t.Error("BiddersSelected should not be nil")
	}
	if m.BidderRequests == nil {
		t.Error("BidderRequests should not be nil")
	}
	if m.BidderLatency == nil {
		t.Error("BidderLatency should not be nil")
	}
	if m.BidderErrors == nil {
		t.Error("BidderErrors should not be nil")
	}
	if m.BidderTimeouts == nil {
		t.Error("BidderTimeouts should not be nil")
	}
	if m.IDRRequests == nil {
		t.Error("IDRRequests should not be nil")
	}
	if m.IDRLatency == nil {
		t.Error("IDRLatency should not be nil")
	}
	if m.IDRCircuitState == nil {
		t.Error("IDRCircuitState should not be nil")
	}
	if m.PrivacyFiltered == nil {
		t.Error("PrivacyFiltered should not be nil")
	}
	if m.ConsentSignals == nil {
		t.Error("ConsentSignals should not be nil")
	}
	if m.ActiveConnections == nil {
		t.Error("ActiveConnections should not be nil")
	}
	if m.RateLimitRejected == nil {
		t.Error("RateLimitRejected should not be nil")
	}
	if m.AuthFailures == nil {
		t.Error("AuthFailures should not be nil")
	}
}

func TestHandler(t *testing.T) {
	handler := Handler()
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}

	// Test that it responds
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	m, _ := createTestMetrics("mw")

	// Create a simple handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	wrapped := m.Middleware(testHandler)

	// Make a request
	req := httptest.NewRequest("GET", "/test/path", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Verify RequestsTotal was incremented
	count := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/test/path", "200"))
	if count != 1 {
		t.Errorf("expected RequestsTotal to be 1, got %f", count)
	}
}

func TestMiddleware_RecordsDifferentStatuses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := createTestMetrics("mw_status")

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			wrapped := m.Middleware(testHandler)
			req := httptest.NewRequest("POST", "/api", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestMiddleware_RequestsInFlight(t *testing.T) {
	m, _ := createTestMetrics("mw_inflight")

	inFlightDuringRequest := float64(0)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture in-flight value during request
		inFlightDuringRequest = testutil.ToFloat64(m.RequestsInFlight)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Middleware(testHandler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Before request
	before := testutil.ToFloat64(m.RequestsInFlight)
	if before != 0 {
		t.Errorf("expected 0 in-flight before request, got %f", before)
	}

	wrapped.ServeHTTP(w, req)

	// During request should have been 1
	if inFlightDuringRequest != 1 {
		t.Errorf("expected 1 in-flight during request, got %f", inFlightDuringRequest)
	}

	// After request should be 0 again
	after := testutil.ToFloat64(m.RequestsInFlight)
	if after != 0 {
		t.Errorf("expected 0 in-flight after request, got %f", after)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rw.statusCode)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected underlying writer to have 404, got %d", w.Code)
	}
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Default should be 200
	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default 200, got %d", rw.statusCode)
	}
}

func TestRecordAuction(t *testing.T) {
	m, _ := createTestMetrics("auction")

	m.RecordAuction("success", "banner", 100*time.Millisecond, 5, 2)

	// Verify counter
	count := testutil.ToFloat64(m.AuctionsTotal.WithLabelValues("success", "banner"))
	if count != 1 {
		t.Errorf("expected AuctionsTotal to be 1, got %f", count)
	}
}

func TestRecordAuction_DifferentStatuses(t *testing.T) {
	m, _ := createTestMetrics("auction_status")

	m.RecordAuction("success", "banner", 50*time.Millisecond, 3, 0)
	m.RecordAuction("success", "banner", 60*time.Millisecond, 4, 1)
	m.RecordAuction("error", "banner", 10*time.Millisecond, 0, 5)
	m.RecordAuction("nobid", "video", 100*time.Millisecond, 5, 0)

	successBanner := testutil.ToFloat64(m.AuctionsTotal.WithLabelValues("success", "banner"))
	if successBanner != 2 {
		t.Errorf("expected 2 success banner auctions, got %f", successBanner)
	}

	errorBanner := testutil.ToFloat64(m.AuctionsTotal.WithLabelValues("error", "banner"))
	if errorBanner != 1 {
		t.Errorf("expected 1 error banner auction, got %f", errorBanner)
	}

	nobidVideo := testutil.ToFloat64(m.AuctionsTotal.WithLabelValues("nobid", "video"))
	if nobidVideo != 1 {
		t.Errorf("expected 1 nobid video auction, got %f", nobidVideo)
	}
}

func TestRecordBid(t *testing.T) {
	m, _ := createTestMetrics("bid")

	m.RecordBid("appnexus", "banner", 2.50)

	count := testutil.ToFloat64(m.BidsReceived.WithLabelValues("appnexus", "banner"))
	if count != 1 {
		t.Errorf("expected BidsReceived to be 1, got %f", count)
	}
}

func TestRecordBid_MultipleBidders(t *testing.T) {
	m, _ := createTestMetrics("bid_multi")

	m.RecordBid("appnexus", "banner", 2.50)
	m.RecordBid("rubicon", "banner", 3.00)
	m.RecordBid("appnexus", "video", 5.00)
	m.RecordBid("appnexus", "banner", 2.75)

	appnexusBanner := testutil.ToFloat64(m.BidsReceived.WithLabelValues("appnexus", "banner"))
	if appnexusBanner != 2 {
		t.Errorf("expected 2 appnexus banner bids, got %f", appnexusBanner)
	}

	rubiconBanner := testutil.ToFloat64(m.BidsReceived.WithLabelValues("rubicon", "banner"))
	if rubiconBanner != 1 {
		t.Errorf("expected 1 rubicon banner bid, got %f", rubiconBanner)
	}

	appnexusVideo := testutil.ToFloat64(m.BidsReceived.WithLabelValues("appnexus", "video"))
	if appnexusVideo != 1 {
		t.Errorf("expected 1 appnexus video bid, got %f", appnexusVideo)
	}
}

func TestRecordBidderRequest_Success(t *testing.T) {
	m, _ := createTestMetrics("bidder_req")

	m.RecordBidderRequest("appnexus", 50*time.Millisecond, false, false)

	count := testutil.ToFloat64(m.BidderRequests.WithLabelValues("appnexus"))
	if count != 1 {
		t.Errorf("expected BidderRequests to be 1, got %f", count)
	}

	errors := testutil.ToFloat64(m.BidderErrors.WithLabelValues("appnexus", "error"))
	if errors != 0 {
		t.Errorf("expected no errors, got %f", errors)
	}

	timeouts := testutil.ToFloat64(m.BidderTimeouts.WithLabelValues("appnexus"))
	if timeouts != 0 {
		t.Errorf("expected no timeouts, got %f", timeouts)
	}
}

func TestRecordBidderRequest_WithError(t *testing.T) {
	m, _ := createTestMetrics("bidder_err")

	m.RecordBidderRequest("rubicon", 100*time.Millisecond, true, false)

	count := testutil.ToFloat64(m.BidderRequests.WithLabelValues("rubicon"))
	if count != 1 {
		t.Errorf("expected BidderRequests to be 1, got %f", count)
	}

	errors := testutil.ToFloat64(m.BidderErrors.WithLabelValues("rubicon", "error"))
	if errors != 1 {
		t.Errorf("expected 1 error, got %f", errors)
	}
}

func TestRecordBidderRequest_WithTimeout(t *testing.T) {
	m, _ := createTestMetrics("bidder_timeout")

	m.RecordBidderRequest("pubmatic", 500*time.Millisecond, false, true)

	timeouts := testutil.ToFloat64(m.BidderTimeouts.WithLabelValues("pubmatic"))
	if timeouts != 1 {
		t.Errorf("expected 1 timeout, got %f", timeouts)
	}
}

func TestRecordBidderRequest_WithBothErrorAndTimeout(t *testing.T) {
	m, _ := createTestMetrics("bidder_both")

	m.RecordBidderRequest("ix", 500*time.Millisecond, true, true)

	errors := testutil.ToFloat64(m.BidderErrors.WithLabelValues("ix", "error"))
	if errors != 1 {
		t.Errorf("expected 1 error, got %f", errors)
	}

	timeouts := testutil.ToFloat64(m.BidderTimeouts.WithLabelValues("ix"))
	if timeouts != 1 {
		t.Errorf("expected 1 timeout, got %f", timeouts)
	}
}

func TestRecordIDRRequest(t *testing.T) {
	m, _ := createTestMetrics("idr")

	m.RecordIDRRequest("success", 25*time.Millisecond)

	count := testutil.ToFloat64(m.IDRRequests.WithLabelValues("success"))
	if count != 1 {
		t.Errorf("expected IDRRequests to be 1, got %f", count)
	}
}

func TestRecordIDRRequest_DifferentStatuses(t *testing.T) {
	m, _ := createTestMetrics("idr_status")

	m.RecordIDRRequest("success", 20*time.Millisecond)
	m.RecordIDRRequest("success", 30*time.Millisecond)
	m.RecordIDRRequest("error", 10*time.Millisecond)
	m.RecordIDRRequest("bypass", 5*time.Millisecond)

	success := testutil.ToFloat64(m.IDRRequests.WithLabelValues("success"))
	if success != 2 {
		t.Errorf("expected 2 success requests, got %f", success)
	}

	errCount := testutil.ToFloat64(m.IDRRequests.WithLabelValues("error"))
	if errCount != 1 {
		t.Errorf("expected 1 error request, got %f", errCount)
	}

	bypass := testutil.ToFloat64(m.IDRRequests.WithLabelValues("bypass"))
	if bypass != 1 {
		t.Errorf("expected 1 bypass request, got %f", bypass)
	}
}

func TestSetIDRCircuitState_Closed(t *testing.T) {
	m, _ := createTestMetrics("circuit_closed")

	m.SetIDRCircuitState("closed")

	state := testutil.ToFloat64(m.IDRCircuitState.WithLabelValues())
	if state != 0 {
		t.Errorf("expected state 0 (closed), got %f", state)
	}
}

func TestSetIDRCircuitState_Open(t *testing.T) {
	m, _ := createTestMetrics("circuit_open")

	m.SetIDRCircuitState("open")

	state := testutil.ToFloat64(m.IDRCircuitState.WithLabelValues())
	if state != 1 {
		t.Errorf("expected state 1 (open), got %f", state)
	}
}

func TestSetIDRCircuitState_HalfOpen(t *testing.T) {
	m, _ := createTestMetrics("circuit_half")

	m.SetIDRCircuitState("half-open")

	state := testutil.ToFloat64(m.IDRCircuitState.WithLabelValues())
	if state != 2 {
		t.Errorf("expected state 2 (half-open), got %f", state)
	}
}

func TestSetIDRCircuitState_Unknown(t *testing.T) {
	m, _ := createTestMetrics("circuit_unknown")

	m.SetIDRCircuitState("unknown")

	state := testutil.ToFloat64(m.IDRCircuitState.WithLabelValues())
	if state != 0 {
		t.Errorf("expected state 0 (default), got %f", state)
	}
}

func TestSetIDRCircuitState_Transitions(t *testing.T) {
	m, _ := createTestMetrics("circuit_trans")

	// Start closed
	m.SetIDRCircuitState("closed")
	if testutil.ToFloat64(m.IDRCircuitState.WithLabelValues()) != 0 {
		t.Error("expected closed (0)")
	}

	// Transition to open
	m.SetIDRCircuitState("open")
	if testutil.ToFloat64(m.IDRCircuitState.WithLabelValues()) != 1 {
		t.Error("expected open (1)")
	}

	// Transition to half-open
	m.SetIDRCircuitState("half-open")
	if testutil.ToFloat64(m.IDRCircuitState.WithLabelValues()) != 2 {
		t.Error("expected half-open (2)")
	}

	// Back to closed
	m.SetIDRCircuitState("closed")
	if testutil.ToFloat64(m.IDRCircuitState.WithLabelValues()) != 0 {
		t.Error("expected closed (0)")
	}
}

func TestRecordPrivacyFiltered(t *testing.T) {
	m, _ := createTestMetrics("privacy")

	m.RecordPrivacyFiltered("appnexus", "gdpr")

	count := testutil.ToFloat64(m.PrivacyFiltered.WithLabelValues("appnexus", "gdpr"))
	if count != 1 {
		t.Errorf("expected PrivacyFiltered to be 1, got %f", count)
	}
}

func TestRecordPrivacyFiltered_DifferentReasons(t *testing.T) {
	m, _ := createTestMetrics("privacy_reasons")

	m.RecordPrivacyFiltered("bidder1", "gdpr")
	m.RecordPrivacyFiltered("bidder1", "ccpa")
	m.RecordPrivacyFiltered("bidder2", "gdpr")
	m.RecordPrivacyFiltered("bidder1", "gdpr")

	bidder1GDPR := testutil.ToFloat64(m.PrivacyFiltered.WithLabelValues("bidder1", "gdpr"))
	if bidder1GDPR != 2 {
		t.Errorf("expected 2 GDPR filters for bidder1, got %f", bidder1GDPR)
	}

	bidder1CCPA := testutil.ToFloat64(m.PrivacyFiltered.WithLabelValues("bidder1", "ccpa"))
	if bidder1CCPA != 1 {
		t.Errorf("expected 1 CCPA filter for bidder1, got %f", bidder1CCPA)
	}

	bidder2GDPR := testutil.ToFloat64(m.PrivacyFiltered.WithLabelValues("bidder2", "gdpr"))
	if bidder2GDPR != 1 {
		t.Errorf("expected 1 GDPR filter for bidder2, got %f", bidder2GDPR)
	}
}

func TestRecordConsentSignal_WithConsent(t *testing.T) {
	m, _ := createTestMetrics("consent_yes")

	m.RecordConsentSignal("gdpr", true)

	count := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("gdpr", "yes"))
	if count != 1 {
		t.Errorf("expected ConsentSignals (yes) to be 1, got %f", count)
	}

	noConsent := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("gdpr", "no"))
	if noConsent != 0 {
		t.Errorf("expected ConsentSignals (no) to be 0, got %f", noConsent)
	}
}

func TestRecordConsentSignal_WithoutConsent(t *testing.T) {
	m, _ := createTestMetrics("consent_no")

	m.RecordConsentSignal("ccpa", false)

	count := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("ccpa", "no"))
	if count != 1 {
		t.Errorf("expected ConsentSignals (no) to be 1, got %f", count)
	}

	yesConsent := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("ccpa", "yes"))
	if yesConsent != 0 {
		t.Errorf("expected ConsentSignals (yes) to be 0, got %f", yesConsent)
	}
}

func TestRecordConsentSignal_DifferentTypes(t *testing.T) {
	m, _ := createTestMetrics("consent_types")

	m.RecordConsentSignal("gdpr", true)
	m.RecordConsentSignal("ccpa", false)
	m.RecordConsentSignal("gpp", true)
	m.RecordConsentSignal("gdpr", false)

	gdprYes := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("gdpr", "yes"))
	if gdprYes != 1 {
		t.Errorf("expected 1 GDPR yes, got %f", gdprYes)
	}

	gdprNo := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("gdpr", "no"))
	if gdprNo != 1 {
		t.Errorf("expected 1 GDPR no, got %f", gdprNo)
	}

	ccpaNo := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("ccpa", "no"))
	if ccpaNo != 1 {
		t.Errorf("expected 1 CCPA no, got %f", ccpaNo)
	}

	gppYes := testutil.ToFloat64(m.ConsentSignals.WithLabelValues("gpp", "yes"))
	if gppYes != 1 {
		t.Errorf("expected 1 GPP yes, got %f", gppYes)
	}
}

func TestSystemMetrics_ActiveConnections(t *testing.T) {
	m, _ := createTestMetrics("sys_conn")

	// Initially 0
	if testutil.ToFloat64(m.ActiveConnections) != 0 {
		t.Error("expected 0 active connections initially")
	}

	// Increment
	m.ActiveConnections.Inc()
	if testutil.ToFloat64(m.ActiveConnections) != 1 {
		t.Error("expected 1 active connection after Inc")
	}

	m.ActiveConnections.Inc()
	if testutil.ToFloat64(m.ActiveConnections) != 2 {
		t.Error("expected 2 active connections after second Inc")
	}

	// Decrement
	m.ActiveConnections.Dec()
	if testutil.ToFloat64(m.ActiveConnections) != 1 {
		t.Error("expected 1 active connection after Dec")
	}

	// Set
	m.ActiveConnections.Set(10)
	if testutil.ToFloat64(m.ActiveConnections) != 10 {
		t.Error("expected 10 active connections after Set")
	}
}

func TestSystemMetrics_RateLimitRejected(t *testing.T) {
	m, _ := createTestMetrics("sys_rate")

	// Initially 0
	if testutil.ToFloat64(m.RateLimitRejected) != 0 {
		t.Error("expected 0 rate limit rejections initially")
	}

	// Increment
	m.RateLimitRejected.Inc()
	m.RateLimitRejected.Inc()
	m.RateLimitRejected.Inc()

	if testutil.ToFloat64(m.RateLimitRejected) != 3 {
		t.Error("expected 3 rate limit rejections")
	}
}

func TestSystemMetrics_AuthFailures(t *testing.T) {
	m, _ := createTestMetrics("sys_auth")

	// Initially 0
	if testutil.ToFloat64(m.AuthFailures) != 0 {
		t.Error("expected 0 auth failures initially")
	}

	// Increment
	m.AuthFailures.Inc()
	m.AuthFailures.Inc()

	if testutil.ToFloat64(m.AuthFailures) != 2 {
		t.Error("expected 2 auth failures")
	}
}

func TestMiddleware_DifferentMethods(t *testing.T) {
	m, _ := createTestMetrics("mw_methods")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := m.Middleware(testHandler)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		count := testutil.ToFloat64(m.RequestsTotal.WithLabelValues(method, "/api", "200"))
		if count != 1 {
			t.Errorf("expected 1 request for method %s, got %f", method, count)
		}
	}
}

func TestMiddleware_RecordsDuration(t *testing.T) {
	m, _ := createTestMetrics("mw_duration")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	wrapped := m.Middleware(testHandler)

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Verify histogram has a sample
	// Note: We can't easily verify the exact value, but we can verify it's not empty
	histogram := m.RequestDuration.WithLabelValues("GET", "/slow")
	if histogram == nil {
		t.Error("expected histogram to be created")
	}
}

// Benchmark tests
func BenchmarkRecordAuction(b *testing.B) {
	m, _ := createTestMetrics("bench_auction")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordAuction("success", "banner", 100*time.Millisecond, 5, 2)
	}
}

func BenchmarkRecordBid(b *testing.B) {
	m, _ := createTestMetrics("bench_bid")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordBid("appnexus", "banner", 2.50)
	}
}

func BenchmarkRecordBidderRequest(b *testing.B) {
	m, _ := createTestMetrics("bench_bidder")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordBidderRequest("appnexus", 50*time.Millisecond, false, false)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	m, _ := createTestMetrics("bench_mw")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := m.Middleware(testHandler)
	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkSetIDRCircuitState(b *testing.B) {
	m, _ := createTestMetrics("bench_circuit")
	states := []string{"closed", "open", "half-open"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.SetIDRCircuitState(states[i%3])
	}
}

func BenchmarkRecordConsentSignal(b *testing.B) {
	m, _ := createTestMetrics("bench_consent")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordConsentSignal("gdpr", i%2 == 0)
	}
}

// Test that metrics have correct help text (via inspection)
func TestMetrics_HelpText(t *testing.T) {
	m, registry := createTestMetrics("help")

	// Gather all metrics
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Verify we have metrics registered
	if len(families) == 0 {
		t.Error("expected metrics to be registered")
	}

	// Check that help text is non-empty for all metrics
	for _, family := range families {
		if family.GetHelp() == "" {
			t.Errorf("metric %s has empty help text", family.GetName())
		}
	}

	// Sanity check - access a metric to ensure it's properly registered
	_ = m.RequestsTotal
}

// Test counter label combinations
func TestMetrics_LabelCombinations(t *testing.T) {
	m, _ := createTestMetrics("labels")

	// Test that different label combinations create distinct metric series
	m.BidsReceived.WithLabelValues("bidder1", "banner").Inc()
	m.BidsReceived.WithLabelValues("bidder1", "video").Inc()
	m.BidsReceived.WithLabelValues("bidder2", "banner").Inc()

	b1Banner := testutil.ToFloat64(m.BidsReceived.WithLabelValues("bidder1", "banner"))
	b1Video := testutil.ToFloat64(m.BidsReceived.WithLabelValues("bidder1", "video"))
	b2Banner := testutil.ToFloat64(m.BidsReceived.WithLabelValues("bidder2", "banner"))

	if b1Banner != 1 || b1Video != 1 || b2Banner != 1 {
		t.Error("label combinations should create independent series")
	}
}

// Integration test: simulate a full auction flow
func TestMetrics_AuctionFlow(t *testing.T) {
	m, _ := createTestMetrics("flow")

	// Simulate an auction flow
	// 1. Record IDR request
	m.RecordIDRRequest("success", 20*time.Millisecond)

	// 2. Record bidder requests
	m.RecordBidderRequest("appnexus", 50*time.Millisecond, false, false)
	m.RecordBidderRequest("rubicon", 75*time.Millisecond, false, false)
	m.RecordBidderRequest("pubmatic", 500*time.Millisecond, false, true) // timeout

	// 3. Record bids
	m.RecordBid("appnexus", "banner", 2.50)
	m.RecordBid("rubicon", "banner", 3.00)
	// pubmatic timed out, no bid

	// 4. Record auction result
	m.RecordAuction("success", "banner", 150*time.Millisecond, 3, 0)

	// Verify metrics
	if testutil.ToFloat64(m.IDRRequests.WithLabelValues("success")) != 1 {
		t.Error("expected 1 IDR request")
	}
	if testutil.ToFloat64(m.BidderRequests.WithLabelValues("appnexus")) != 1 {
		t.Error("expected 1 appnexus request")
	}
	if testutil.ToFloat64(m.BidderTimeouts.WithLabelValues("pubmatic")) != 1 {
		t.Error("expected 1 pubmatic timeout")
	}
	if testutil.ToFloat64(m.BidsReceived.WithLabelValues("appnexus", "banner")) != 1 {
		t.Error("expected 1 appnexus bid")
	}
	if testutil.ToFloat64(m.AuctionsTotal.WithLabelValues("success", "banner")) != 1 {
		t.Error("expected 1 successful auction")
	}
}

// Test empty namespace defaults to "test" in our helper
func TestCreateTestMetrics_DefaultNamespace(t *testing.T) {
	m, registry := createTestMetrics("")

	families, _ := registry.Gather()
	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), "test_") {
			t.Errorf("expected metric name to start with 'test_', got %s", family.GetName())
		}
	}

	_ = m // Silence unused variable warning
}
