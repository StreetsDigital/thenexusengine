# IDR Performance Optimization: A/B Testing & Timeout Monitoring Plan

## Problem Statement

The IDR (Intelligent Demand Router) processing layer adds latency to the auction flow. If IDR takes too long:
- We miss bid opportunities → Lost revenue
- We timeout on overall auction → Degraded user experience

Current state:
- IDR timeout: 50ms (hardcoded)
- Shadow mode: All-or-nothing (no percentage-based sampling)
- No specific timeout tracking or revenue impact metrics
- No A/B testing infrastructure for traffic sampling

## Goals

1. **A/B test IDR traffic** - Apply IDR to a configurable percentage of traffic
2. **Track timeouts and performance** - Detailed metrics on IDR latency, timeouts, and their impact
3. **Measure revenue impact** - Compare win rates and CPMs between IDR-enabled vs control groups
4. **Create optimal solution** - Use data to determine optimal IDR timeout and traffic percentage

---

## Implementation Plan

### Phase 1: Traffic Sampling & A/B Testing Infrastructure

#### 1.1 Add Sample Rate Configuration

**Files to modify:**
- `pbs/internal/exchange/exchange.go` - Add IDR sample rate config
- `config/idr_config.yaml` - Add sample rate setting
- `.env.example` - Document new environment variables

**New configuration options:**
```yaml
# config/idr_config.yaml
selector:
  sample_rate: 0.5          # Apply IDR to 50% of traffic (0.0 to 1.0)
  control_group_enabled: true # Track control group for comparison
```

**Environment variables:**
```bash
IDR_SAMPLE_RATE=0.5         # 0.0 to 1.0 (default: 1.0 = all traffic)
IDR_CONTROL_GROUP=true       # Enable control group tracking
```

#### 1.2 Implement Request-Level A/B Assignment

**Files to modify:**
- `pbs/pkg/idr/client.go` - Add ShouldApplyIDR() method
- `pbs/internal/exchange/exchange.go` - Use sample rate for IDR decision

**Logic:**
```go
// Deterministic assignment based on request ID for consistency
func (c *Client) ShouldApplyIDR(requestID string, sampleRate float64) (bool, string) {
    // Hash request ID to get deterministic bucket
    hash := fnv.New32a()
    hash.Write([]byte(requestID))
    bucket := float64(hash.Sum32()) / float64(math.MaxUint32)

    applyIDR := bucket < sampleRate
    group := "control"
    if applyIDR {
        group = "experiment"
    }
    return applyIDR, group
}
```

#### 1.3 Add Experiment Group to Response/Logs

**New fields in DebugInfo:**
```go
type DebugInfo struct {
    // ... existing fields
    ExperimentGroup   string  // "experiment" or "control"
    IDRApplied        bool    // Whether IDR was actually applied
    IDRSkipReason     string  // "sample_rate", "circuit_open", "timeout", "disabled"
}
```

---

### Phase 2: Enhanced Timeout & Performance Metrics

#### 2.1 New Prometheus Metrics

**File to modify:** `pbs/internal/metrics/prometheus.go`

Add these new metrics:

```go
// IDR A/B Testing Metrics
IDRExperimentGroup *prometheus.CounterVec   // Tracks requests by group

// IDR Timeout Metrics
IDRTimeouts        *prometheus.CounterVec   // Total IDR timeouts
IDRTimeoutLatency  *prometheus.HistogramVec // Latency when timeout occurs

// Revenue Impact Metrics (per experiment group)
WinRate           *prometheus.GaugeVec     // Win rate by experiment group
AverageCPM        *prometheus.GaugeVec     // Avg winning CPM by group
RevenuePerRequest *prometheus.GaugeVec     // Estimated revenue per request

// IDR Decision Metrics
IDRDecisionLatency *prometheus.HistogramVec // Time taken to make IDR decision
IDRBiddersReduced  *prometheus.HistogramVec // Number of bidders filtered out
IDRFallbackCount   *prometheus.CounterVec   // Times fell back to all bidders
```

**Metric labels:**
```go
[]string{"experiment_group"}  // "experiment" or "control"
[]string{"timeout_reason"}    // "idr_service", "context_deadline", "circuit_open"
[]string{"skip_reason"}       // "sample_rate", "disabled", "circuit_open"
```

#### 2.2 Timeout Tracking in IDR Client

**File to modify:** `pbs/pkg/idr/client.go`

Add timeout detection and categorization:

```go
type SelectPartnersResult struct {
    Response     *SelectPartnersResponse
    Error        error
    TimedOut     bool
    TimeoutType  string  // "http_timeout", "context_deadline", "circuit_open"
    Latency      time.Duration
}

func (c *Client) SelectPartnersWithMetrics(ctx context.Context, ...) *SelectPartnersResult {
    start := time.Now()
    result := &SelectPartnersResult{}

    // Check if context already timed out
    select {
    case <-ctx.Done():
        result.TimedOut = true
        result.TimeoutType = "context_deadline"
        result.Latency = time.Since(start)
        return result
    default:
    }

    // ... existing logic with timeout detection
}
```

#### 2.3 IDR Latency Budget Awareness

**New configuration:**
```yaml
# config/idr_config.yaml
timeout:
  idr_timeout_ms: 50          # Timeout for IDR service call
  idr_budget_pct: 5           # Max % of auction time IDR can use
  adaptive_timeout: true       # Adjust timeout based on tmax
```

**Adaptive timeout logic:**
```go
// Calculate IDR timeout as percentage of remaining auction time
func calculateIDRTimeout(remainingTime time.Duration, budgetPct float64, maxTimeout time.Duration) time.Duration {
    budget := time.Duration(float64(remainingTime) * budgetPct / 100)
    if budget > maxTimeout {
        return maxTimeout
    }
    if budget < 10*time.Millisecond {
        return 10 * time.Millisecond // Minimum viable timeout
    }
    return budget
}
```

---

### Phase 3: Revenue Impact Tracking

#### 3.1 Event Recording Enhancement

**File to modify:** `pbs/pkg/idr/events.go`

Add experiment group to events:
```go
type AuctionEvent struct {
    // ... existing fields
    ExperimentGroup string  `json:"experiment_group"`
    IDRApplied      bool    `json:"idr_applied"`
    IDRLatencyMs    float64 `json:"idr_latency_ms"`
    IDRTimedOut     bool    `json:"idr_timed_out"`
    TimeoutReason   string  `json:"timeout_reason,omitempty"`
}
```

#### 3.2 Revenue Attribution Logging

**New structured log fields:**
```go
logger.Log.Info().
    Str("requestID", requestID).
    Str("experimentGroup", group).
    Bool("idrApplied", applied).
    Float64("idrLatencyMs", latency.Milliseconds()).
    Bool("idrTimedOut", timedOut).
    Int("biddersSelected", len(selected)).
    Int("biddersAvailable", len(available)).
    Float64("winningCPM", winningBid.Price).
    Str("winningBidder", winningBid.Seat).
    Msg("auction_result")
```

#### 3.3 Aggregated Metrics for Analysis

**File to modify:** `pbs/internal/metrics/prometheus.go`

```go
// Track aggregate performance by experiment group
AuctionRevenueTotal *prometheus.CounterVec  // Total revenue by group
AuctionWinsTotal    *prometheus.CounterVec  // Total wins by group
AuctionBidsTotal    *prometheus.CounterVec  // Total bids received by group
```

---

### Phase 4: Admin & Monitoring Endpoints

#### 4.1 A/B Test Control Endpoints

**File to create:** `pbs/internal/endpoints/admin_idr.go`

```go
// GET /admin/idr/experiment - Get current experiment config
// POST /admin/idr/experiment - Update experiment config
// GET /admin/idr/stats - Get experiment statistics

type ExperimentConfig struct {
    SampleRate       float64 `json:"sample_rate"`
    ControlEnabled   bool    `json:"control_group_enabled"`
    IDRTimeoutMs     int     `json:"idr_timeout_ms"`
    AdaptiveTimeout  bool    `json:"adaptive_timeout"`
}

type ExperimentStats struct {
    ExperimentGroup struct {
        Requests    int64   `json:"requests"`
        Timeouts    int64   `json:"timeouts"`
        AvgLatency  float64 `json:"avg_latency_ms"`
        WinRate     float64 `json:"win_rate"`
        AvgCPM      float64 `json:"avg_cpm"`
    } `json:"experiment"`
    ControlGroup struct {
        // Same fields
    } `json:"control"`
}
```

#### 4.2 Real-time Dashboard Data

**Grafana query examples:**
```promql
# Compare win rates between groups
sum(rate(pbs_auction_wins_total{experiment_group="experiment"}[5m]))
/
sum(rate(pbs_auctions_total{experiment_group="experiment"}[5m]))

# IDR timeout rate
sum(rate(pbs_idr_timeouts_total[5m]))
/
sum(rate(pbs_idr_requests_total[5m]))

# Revenue comparison
sum(rate(pbs_auction_revenue_total{experiment_group="experiment"}[5m]))
-
sum(rate(pbs_auction_revenue_total{experiment_group="control"}[5m]))
```

---

## Files to Create/Modify Summary

### New Files
| File | Purpose |
|------|---------|
| `pbs/pkg/idr/experiment.go` | A/B test assignment logic |
| `pbs/internal/endpoints/admin_idr.go` | Admin endpoints for experiment control |

### Modified Files
| File | Changes |
|------|---------|
| `pbs/pkg/idr/client.go` | Add timeout tracking, metrics, sample rate |
| `pbs/pkg/idr/events.go` | Add experiment group to events |
| `pbs/internal/exchange/exchange.go` | Integrate A/B testing, enhanced logging |
| `pbs/internal/metrics/prometheus.go` | Add new metrics |
| `config/idr_config.yaml` | Add sample rate, timeout config |
| `.env.example` | Document new env vars |

---

## Testing Strategy

### Unit Tests
- Test deterministic A/B assignment (same request ID → same group)
- Test timeout detection and categorization
- Test adaptive timeout calculation
- Test sample rate edge cases (0.0, 1.0, negative, >1.0)

### Integration Tests
- Test full auction flow with IDR sampling
- Test fallback behavior when IDR times out
- Test metrics recording in both groups

### Load Tests
- Measure latency impact of A/B assignment
- Verify metrics don't impact throughput
- Test under various sample rates

---

## Rollout Strategy

1. **Phase 1 (Week 1)**: Deploy with `sample_rate: 0.0` (IDR disabled, control baseline)
2. **Phase 2 (Week 2)**: Enable shadow mode with `sample_rate: 0.1` (10% traffic)
3. **Phase 3 (Week 3)**: Increase to `sample_rate: 0.25` if metrics look good
4. **Phase 4 (Week 4+)**: Gradually increase based on timeout/revenue data
5. **Final**: Set optimal sample rate based on analysis

---

## Success Criteria

1. **Timeout rate < 1%** - IDR should rarely timeout
2. **Latency p99 < 50ms** - IDR should be fast
3. **Revenue neutral or positive** - IDR group should not lose revenue vs control
4. **Clear metrics** - Dashboard shows actionable data for optimization

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| IDR timeout impacts auction | Adaptive timeout + circuit breaker |
| A/B assignment adds latency | Use fast hash function (FNV-1a) |
| Metrics cardinality explosion | Limit labels, use fixed experiment groups |
| Lost revenue during testing | Start with low sample rate, monitor closely |
