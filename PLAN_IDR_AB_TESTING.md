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
- No ability to target specific traffic segments for testing

## Goals

1. **A/B test IDR traffic** - Apply IDR to configurable traffic segments
2. **Target specific segments** - Test IDR on specific publishers, geos, devices, formats, etc.
3. **Track timeouts and performance** - Detailed metrics on IDR latency, timeouts, and their impact
4. **Measure revenue impact** - Compare win rates and CPMs between IDR-enabled vs control groups
5. **Create optimal solution** - Use data to determine optimal IDR timeout and traffic segments

---

## Implementation Plan

### Phase 0: Targeting Rules Engine (NEW)

This phase adds the ability to target specific traffic segments for IDR testing based on any bid request attribute.

#### 0.1 Targeting Rules Configuration

**File to create:** `config/idr_experiments.yaml`

```yaml
# IDR Experiment Configuration
# Supports multiple concurrent experiments with targeting rules

experiments:
  # Example: Test IDR on US mobile banner traffic for specific publisher
  - name: "us_mobile_banner_test"
    enabled: true
    sample_rate: 0.5          # Apply IDR to 50% of matching traffic
    priority: 10              # Higher priority = evaluated first

    # Targeting rules (ALL must match - AND logic)
    targeting:
      # Publisher targeting
      publisher_ids:
        - "pub123"
        - "pub456"

      # Site/App targeting
      site_domains:
        - "example.com"
        - "news.example.com"
      app_bundles: []

      # Geographic targeting
      countries:
        - "USA"
        - "CAN"
      regions: []             # e.g., ["CA", "NY", "TX"]

      # Device targeting
      device_types:
        - "mobile"            # mobile, desktop, ctv, tablet
      os:
        - "iOS"
        - "Android"
      browsers: []            # Chrome, Safari, Firefox, Edge

      # Format targeting
      media_types:
        - "banner"            # banner, video, native, audio
      ad_sizes:
        - "300x250"
        - "320x50"

      # Additional OpenRTB fields
      connection_types: []    # wifi, cellular, ethernet
      carriers: []            # Carrier names
      languages: []           # en, es, fr, etc.

    # Experiment-specific settings
    settings:
      idr_timeout_ms: 40      # Custom timeout for this experiment
      shadow_mode: false      # If true, track but don't filter

  # Example: Shadow mode test for video on CTV
  - name: "ctv_video_shadow"
    enabled: true
    sample_rate: 1.0          # Track all matching traffic
    priority: 5
    targeting:
      device_types: ["ctv"]
      media_types: ["video"]
    settings:
      shadow_mode: true       # Don't filter, just track what IDR would do

  # Default experiment (catch-all for unmatched traffic)
  - name: "default"
    enabled: true
    sample_rate: 0.0          # Control group (no IDR) by default
    priority: 0               # Lowest priority
    targeting: {}             # Match everything
    settings:
      idr_timeout_ms: 50
```

#### 0.2 Targeting Rules Data Structure

**File to create:** `pbs/pkg/idr/targeting.go`

```go
package idr

import (
    "github.com/mxmCherry/openrtb/v16/openrtb2"
)

// Experiment defines an IDR A/B test experiment
type Experiment struct {
    Name       string            `yaml:"name" json:"name"`
    Enabled    bool              `yaml:"enabled" json:"enabled"`
    SampleRate float64           `yaml:"sample_rate" json:"sample_rate"`
    Priority   int               `yaml:"priority" json:"priority"`
    Targeting  TargetingRules    `yaml:"targeting" json:"targeting"`
    Settings   ExperimentSettings `yaml:"settings" json:"settings"`
}

// TargetingRules defines which traffic an experiment applies to
type TargetingRules struct {
    // Publisher/Site/App
    PublisherIDs   []string `yaml:"publisher_ids" json:"publisher_ids"`
    SiteDomains    []string `yaml:"site_domains" json:"site_domains"`
    AppBundles     []string `yaml:"app_bundles" json:"app_bundles"`
    PlacementIDs   []string `yaml:"placement_ids" json:"placement_ids"`

    // Geographic
    Countries      []string `yaml:"countries" json:"countries"`
    Regions        []string `yaml:"regions" json:"regions"`
    Cities         []string `yaml:"cities" json:"cities"`
    DMAs           []string `yaml:"dmas" json:"dmas"`

    // Device
    DeviceTypes    []string `yaml:"device_types" json:"device_types"`     // mobile, desktop, ctv, tablet
    OS             []string `yaml:"os" json:"os"`                         // iOS, Android, Windows, macOS
    OSVersions     []string `yaml:"os_versions" json:"os_versions"`       // 14.0, 15.0
    Browsers       []string `yaml:"browsers" json:"browsers"`             // Chrome, Safari, Firefox
    BrowserVersions []string `yaml:"browser_versions" json:"browser_versions"`
    DeviceMakes    []string `yaml:"device_makes" json:"device_makes"`     // Apple, Samsung
    DeviceModels   []string `yaml:"device_models" json:"device_models"`

    // Format
    MediaTypes     []string `yaml:"media_types" json:"media_types"`       // banner, video, native, audio
    AdSizes        []string `yaml:"ad_sizes" json:"ad_sizes"`             // 300x250, 728x90

    // Network
    ConnectionTypes []string `yaml:"connection_types" json:"connection_types"` // wifi, cellular
    Carriers        []string `yaml:"carriers" json:"carriers"`

    // User
    Languages      []string `yaml:"languages" json:"languages"`
    HasUserID      *bool    `yaml:"has_user_id" json:"has_user_id"`       // nil = don't care
    HasGDPRConsent *bool    `yaml:"has_gdpr_consent" json:"has_gdpr_consent"`

    // Custom: Extension fields from bid request
    CustomRules    map[string][]string `yaml:"custom_rules" json:"custom_rules"`
}

// ExperimentSettings defines IDR behavior for this experiment
type ExperimentSettings struct {
    IDRTimeoutMs    int  `yaml:"idr_timeout_ms" json:"idr_timeout_ms"`
    ShadowMode      bool `yaml:"shadow_mode" json:"shadow_mode"`
    AdaptiveTimeout bool `yaml:"adaptive_timeout" json:"adaptive_timeout"`
    MaxBidders      int  `yaml:"max_bidders" json:"max_bidders"`
}

// ExperimentManager manages experiment matching and assignment
type ExperimentManager struct {
    experiments []Experiment
    configPath  string
}

// MatchResult contains the result of experiment matching
type MatchResult struct {
    Experiment      *Experiment
    ExperimentName  string
    ApplyIDR        bool    // Based on sample_rate
    ExperimentGroup string  // "experiment" or "control"
    MatchedRules    []string // Which targeting rules matched
}

// Match finds the matching experiment for a bid request
func (em *ExperimentManager) Match(req *openrtb2.BidRequest) *MatchResult {
    // Experiments are sorted by priority (highest first)
    for _, exp := range em.experiments {
        if !exp.Enabled {
            continue
        }
        if em.matchesTargeting(req, &exp.Targeting) {
            applyIDR, group := em.assignGroup(req.ID, exp.SampleRate)
            return &MatchResult{
                Experiment:      &exp,
                ExperimentName:  exp.Name,
                ApplyIDR:        applyIDR,
                ExperimentGroup: group,
            }
        }
    }
    return nil // No matching experiment
}

// matchesTargeting checks if a request matches all targeting rules
func (em *ExperimentManager) matchesTargeting(req *openrtb2.BidRequest, rules *TargetingRules) bool {
    // Empty rules = match everything
    if em.isEmptyRules(rules) {
        return true
    }

    // All specified rules must match (AND logic)
    // Empty slice for a rule = don't filter on that dimension

    if len(rules.PublisherIDs) > 0 && !em.matchPublisher(req, rules.PublisherIDs) {
        return false
    }
    if len(rules.SiteDomains) > 0 && !em.matchSiteDomain(req, rules.SiteDomains) {
        return false
    }
    if len(rules.Countries) > 0 && !em.matchCountry(req, rules.Countries) {
        return false
    }
    if len(rules.DeviceTypes) > 0 && !em.matchDeviceType(req, rules.DeviceTypes) {
        return false
    }
    if len(rules.MediaTypes) > 0 && !em.matchMediaType(req, rules.MediaTypes) {
        return false
    }
    if len(rules.AdSizes) > 0 && !em.matchAdSize(req, rules.AdSizes) {
        return false
    }
    if len(rules.OS) > 0 && !em.matchOS(req, rules.OS) {
        return false
    }
    if len(rules.Browsers) > 0 && !em.matchBrowser(req, rules.Browsers) {
        return false
    }
    // ... additional matching logic

    return true
}
```

#### 0.3 Extracting Request Attributes

**File to add:** `pbs/pkg/idr/request_attributes.go`

```go
package idr

import (
    "fmt"
    "strings"

    "github.com/mxmCherry/openrtb/v16/openrtb2"
)

// RequestAttributes extracts targetable attributes from a bid request
type RequestAttributes struct {
    // Publisher/Site/App
    PublisherID   string
    SiteDomain    string
    AppBundle     string
    PlacementIDs  []string

    // Geographic
    Country       string
    Region        string
    City          string
    DMA           string

    // Device
    DeviceType    string   // mobile, desktop, ctv, tablet
    OS            string   // iOS, Android, Windows, macOS, Linux
    OSVersion     string
    Browser       string   // Chrome, Safari, Firefox, Edge
    BrowserVersion string
    DeviceMake    string
    DeviceModel   string

    // Format
    MediaTypes    []string // banner, video, native, audio
    AdSizes       []string // 300x250, 728x90

    // Network
    ConnectionType string  // wifi, cellular, ethernet
    Carrier        string
    IP             string

    // User
    Language       string
    HasUserID      bool
    HasGDPRConsent bool

    // Raw request for custom rules
    RawRequest    *openrtb2.BidRequest
}

// ExtractAttributes extracts all targetable attributes from a bid request
func ExtractAttributes(req *openrtb2.BidRequest) *RequestAttributes {
    attrs := &RequestAttributes{
        RawRequest: req,
    }

    // Publisher/Site/App
    if req.Site != nil {
        attrs.SiteDomain = req.Site.Domain
        if req.Site.Publisher != nil {
            attrs.PublisherID = req.Site.Publisher.ID
        }
    }
    if req.App != nil {
        attrs.AppBundle = req.App.Bundle
        if req.App.Publisher != nil && attrs.PublisherID == "" {
            attrs.PublisherID = req.App.Publisher.ID
        }
    }

    // Device
    if req.Device != nil {
        attrs.DeviceType = getDeviceType(req.Device.DeviceType)
        attrs.OS = normalizeOS(req.Device.OS)
        attrs.OSVersion = req.Device.OSV
        attrs.DeviceMake = req.Device.Make
        attrs.DeviceModel = req.Device.Model
        attrs.Carrier = req.Device.Carrier
        attrs.IP = req.Device.IP
        attrs.Language = req.Device.Language

        // Browser from User-Agent
        attrs.Browser, attrs.BrowserVersion = parseBrowser(req.Device.UA)

        // Connection type
        attrs.ConnectionType = getConnectionType(req.Device.ConnectionType)

        // Geographic
        if req.Device.Geo != nil {
            attrs.Country = req.Device.Geo.Country
            attrs.Region = req.Device.Geo.Region
            attrs.City = req.Device.Geo.City
            attrs.DMA = req.Device.Geo.DMA
        }
    }

    // User
    if req.User != nil {
        attrs.HasUserID = req.User.ID != "" || req.User.BuyerUID != ""
    }

    // GDPR
    if req.Regs != nil && req.Regs.Ext != nil {
        // Parse GDPR consent from regs.ext
        attrs.HasGDPRConsent = hasGDPRConsent(req)
    }

    // Impressions - extract media types and sizes
    for _, imp := range req.Imp {
        attrs.PlacementIDs = append(attrs.PlacementIDs, imp.TagID)

        if imp.Banner != nil {
            attrs.MediaTypes = appendUnique(attrs.MediaTypes, "banner")
            if imp.Banner.W != nil && imp.Banner.H != nil {
                size := fmt.Sprintf("%dx%d", *imp.Banner.W, *imp.Banner.H)
                attrs.AdSizes = appendUnique(attrs.AdSizes, size)
            }
            for _, format := range imp.Banner.Format {
                size := fmt.Sprintf("%dx%d", format.W, format.H)
                attrs.AdSizes = appendUnique(attrs.AdSizes, size)
            }
        }
        if imp.Video != nil {
            attrs.MediaTypes = appendUnique(attrs.MediaTypes, "video")
        }
        if imp.Native != nil {
            attrs.MediaTypes = appendUnique(attrs.MediaTypes, "native")
        }
        if imp.Audio != nil {
            attrs.MediaTypes = appendUnique(attrs.MediaTypes, "audio")
        }
    }

    return attrs
}

func getDeviceType(dt openrtb2.DeviceType) string {
    switch dt {
    case 1: return "mobile"
    case 2: return "desktop"
    case 3: return "ctv"
    case 4: return "phone"
    case 5: return "tablet"
    case 6: return "connected_device"
    case 7: return "set_top_box"
    default: return "unknown"
    }
}

func normalizeOS(os string) string {
    os = strings.ToLower(os)
    switch {
    case strings.Contains(os, "ios"): return "iOS"
    case strings.Contains(os, "android"): return "Android"
    case strings.Contains(os, "windows"): return "Windows"
    case strings.Contains(os, "mac"): return "macOS"
    case strings.Contains(os, "linux"): return "Linux"
    case strings.Contains(os, "tizen"): return "Tizen"
    case strings.Contains(os, "roku"): return "Roku"
    case strings.Contains(os, "fire"): return "FireOS"
    default: return os
    }
}

func getConnectionType(ct openrtb2.ConnectionType) string {
    switch ct {
    case 1: return "ethernet"
    case 2: return "wifi"
    case 3: return "cellular_unknown"
    case 4: return "2g"
    case 5: return "3g"
    case 6: return "4g"
    default: return "unknown"
    }
}
```

#### 0.4 Configuration Hot-Reload

Support updating experiments without restart:

```go
// WatchConfig watches for config file changes and reloads experiments
func (em *ExperimentManager) WatchConfig(ctx context.Context) {
    watcher, _ := fsnotify.NewWatcher()
    defer watcher.Close()

    watcher.Add(em.configPath)

    for {
        select {
        case <-ctx.Done():
            return
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                if err := em.Reload(); err != nil {
                    log.Error().Err(err).Msg("failed to reload experiments")
                }
            }
        }
    }
}

// Reload reloads experiments from config file
func (em *ExperimentManager) Reload() error {
    experiments, err := LoadExperiments(em.configPath)
    if err != nil {
        return err
    }

    // Sort by priority (highest first)
    sort.Slice(experiments, func(i, j int) bool {
        return experiments[i].Priority > experiments[j].Priority
    })

    em.mu.Lock()
    em.experiments = experiments
    em.mu.Unlock()

    log.Info().Int("count", len(experiments)).Msg("reloaded IDR experiments")
    return nil
}
```

---

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
// Experiment Management
// GET  /admin/idr/experiments              - List all experiments
// GET  /admin/idr/experiments/:name        - Get specific experiment
// POST /admin/idr/experiments              - Create new experiment
// PUT  /admin/idr/experiments/:name        - Update experiment
// DELETE /admin/idr/experiments/:name      - Delete experiment

// Experiment Control
// POST /admin/idr/experiments/:name/enable  - Enable experiment
// POST /admin/idr/experiments/:name/disable - Disable experiment
// POST /admin/idr/experiments/:name/sample-rate - Update sample rate

// Stats & Monitoring
// GET /admin/idr/stats                      - Get all experiment stats
// GET /admin/idr/stats/:name                - Get specific experiment stats

type ExperimentAPI struct {
    Name        string          `json:"name"`
    Enabled     bool            `json:"enabled"`
    SampleRate  float64         `json:"sample_rate"`
    Priority    int             `json:"priority"`
    Targeting   TargetingRules  `json:"targeting"`
    Settings    ExperimentSettings `json:"settings"`
    Stats       *ExperimentStats `json:"stats,omitempty"`
}

type ExperimentStats struct {
    TotalRequests    int64   `json:"total_requests"`
    MatchedRequests  int64   `json:"matched_requests"`
    IDRApplied       int64   `json:"idr_applied"`
    ControlGroup     int64   `json:"control_group"`
    IDRTimeouts      int64   `json:"idr_timeouts"`
    AvgLatencyMs     float64 `json:"avg_latency_ms"`
    P95LatencyMs     float64 `json:"p95_latency_ms"`
    WinRate          float64 `json:"win_rate"`
    AvgWinningCPM    float64 `json:"avg_winning_cpm"`
    RevenuePerMille  float64 `json:"revenue_per_mille"`
}

// Example: Create a new experiment via API
// POST /admin/idr/experiments
// {
//   "name": "test_video_usa",
//   "enabled": true,
//   "sample_rate": 0.25,
//   "priority": 15,
//   "targeting": {
//     "countries": ["USA"],
//     "media_types": ["video"],
//     "device_types": ["mobile", "ctv"]
//   },
//   "settings": {
//     "idr_timeout_ms": 35,
//     "shadow_mode": false
//   }
// }

// Example: Update sample rate dynamically
// POST /admin/idr/experiments/test_video_usa/sample-rate
// { "sample_rate": 0.5 }
```

#### 4.2 Real-time Dashboard Data

**Grafana query examples:**
```promql
# Requests per experiment
sum by (experiment_name) (rate(pbs_idr_experiment_requests_total[5m]))

# Compare win rates between experiment and control within same experiment
sum(rate(pbs_auction_wins_total{experiment_name="us_mobile_banner_test", experiment_group="experiment"}[5m]))
/
sum(rate(pbs_auctions_total{experiment_name="us_mobile_banner_test", experiment_group="experiment"}[5m]))

# IDR timeout rate per experiment
sum by (experiment_name) (rate(pbs_idr_timeouts_total[5m]))
/
sum by (experiment_name) (rate(pbs_idr_requests_total[5m]))

# Revenue comparison within an experiment
sum(rate(pbs_auction_revenue_total{experiment_name="us_mobile_banner_test", experiment_group="experiment"}[5m]))
-
sum(rate(pbs_auction_revenue_total{experiment_name="us_mobile_banner_test", experiment_group="control"}[5m]))

# P95 latency per experiment
histogram_quantile(0.95, sum by (experiment_name, le) (rate(pbs_idr_latency_seconds_bucket[5m])))

# Top experiments by revenue impact
topk(5,
  sum by (experiment_name) (rate(pbs_auction_revenue_total{experiment_group="experiment"}[1h]))
  -
  sum by (experiment_name) (rate(pbs_auction_revenue_total{experiment_group="control"}[1h]))
)
```

#### 4.3 Targeting Rule Debugging

**Endpoint for testing targeting rules:**
```go
// POST /admin/idr/test-targeting
// Test which experiment would match for a given bid request
//
// Request body: OpenRTB bid request (or subset)
// Response:
// {
//   "matched_experiment": "us_mobile_banner_test",
//   "would_apply_idr": true,
//   "experiment_group": "experiment",
//   "matched_rules": [
//     "countries: USA",
//     "device_types: mobile",
//     "media_types: banner"
//   ],
//   "extracted_attributes": {
//     "publisher_id": "pub123",
//     "country": "USA",
//     "device_type": "mobile",
//     "os": "iOS",
//     "browser": "Safari",
//     "media_types": ["banner"],
//     "ad_sizes": ["300x250", "320x50"]
//   }
// }
```

---

## Files to Create/Modify Summary

### New Files
| File | Purpose |
|------|---------|
| `config/idr_experiments.yaml` | Experiment definitions with targeting rules |
| `pbs/pkg/idr/targeting.go` | Targeting rules engine and experiment matching |
| `pbs/pkg/idr/request_attributes.go` | Extract targetable attributes from bid requests |
| `pbs/pkg/idr/experiment.go` | A/B test assignment logic |
| `pbs/internal/endpoints/admin_idr.go` | Admin endpoints for experiment control |

### Modified Files
| File | Changes |
|------|---------|
| `pbs/pkg/idr/client.go` | Add timeout tracking, metrics, experiment integration |
| `pbs/pkg/idr/events.go` | Add experiment group/name to events |
| `pbs/internal/exchange/exchange.go` | Integrate targeting rules, enhanced logging |
| `pbs/internal/metrics/prometheus.go` | Add new metrics with experiment_name label |
| `config/idr_config.yaml` | Add timeout config |
| `.env.example` | Document new env vars |

---

## Testing Strategy

### Unit Tests
- Test deterministic A/B assignment (same request ID → same group)
- Test timeout detection and categorization
- Test adaptive timeout calculation
- Test sample rate edge cases (0.0, 1.0, negative, >1.0)
- **Test targeting rule matching:**
  - Publisher ID matching
  - Country/region/city matching
  - Device type matching (mobile, desktop, ctv)
  - OS and browser matching
  - Media type and ad size matching
  - Combination rules (AND logic)
  - Empty rules (match all)
  - Priority ordering

### Integration Tests
- Test full auction flow with IDR sampling
- Test fallback behavior when IDR times out
- Test metrics recording in both groups
- **Test experiment matching in production flow:**
  - Verify correct experiment matched
  - Verify sample rate applied correctly
  - Verify experiment-specific settings (timeout, shadow mode)
  - Test hot-reload of experiment config

### Load Tests
- Measure latency impact of A/B assignment
- Verify metrics don't impact throughput
- Test under various sample rates
- **Measure targeting rule evaluation latency**

---

## Rollout Strategy

### Recommended Approach: Start with Shadow Mode on Low-Risk Segments

1. **Phase 1: Baseline (1 week)**
   - Deploy with default experiment: `sample_rate: 0.0` (no IDR)
   - Collect baseline metrics for all traffic

2. **Phase 2: Shadow Test on Safe Segment (1-2 weeks)**
   - Create experiment for low-value, high-volume segment:
   ```yaml
   - name: "shadow_test_display_mobile"
     sample_rate: 1.0
     targeting:
       media_types: ["banner"]
       device_types: ["mobile"]
       countries: ["USA"]
     settings:
       shadow_mode: true  # Track but don't filter
   ```
   - Monitor: IDR latency, timeout rate, what would have been filtered

3. **Phase 3: Live Test on Single Segment (1-2 weeks)**
   - Convert shadow to live on 10% of matching traffic:
   ```yaml
   - name: "live_test_display_mobile"
     sample_rate: 0.1  # 10% experiment, 90% control
     targeting:
       media_types: ["banner"]
       device_types: ["mobile"]
       countries: ["USA"]
     settings:
       shadow_mode: false
   ```
   - Compare win rate and CPM: experiment vs control

4. **Phase 4: Expand Based on Results**
   - If positive: Increase sample rate, add more segments
   - If negative: Investigate, adjust IDR timeout/algorithm
   - Create segment-specific experiments based on learnings

5. **Phase 5: Production Rollout**
   - Create experiments for all traffic segments
   - Set optimal sample rates per segment
   - Keep default experiment as fallback

### Example Progressive Rollout:
```yaml
experiments:
  # High-value video - conservative testing
  - name: "video_ctv"
    sample_rate: 0.05
    targeting: { media_types: ["video"], device_types: ["ctv"] }

  # Medium-value mobile banner - moderate testing
  - name: "banner_mobile"
    sample_rate: 0.25
    targeting: { media_types: ["banner"], device_types: ["mobile"] }

  # Lower-value desktop banner - aggressive testing
  - name: "banner_desktop"
    sample_rate: 0.50
    targeting: { media_types: ["banner"], device_types: ["desktop"] }

  # Default: no IDR for unmatched traffic
  - name: "default"
    sample_rate: 0.0
    targeting: {}
```

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
| Metrics cardinality explosion | Limit experiment count, use fixed group labels |
| Lost revenue during testing | Start with low sample rate on low-value segments |
| Targeting rule evaluation too slow | Cache extracted attributes, optimize matching |
| Too many experiments overlap | Priority system, first-match wins |
| Config file corruption | Validation on load, keep last-known-good |
| Inconsistent experiment assignment | Deterministic hash on request ID |

---

## Appendix: Targetable OpenRTB Fields

All fields that can be used in targeting rules:

| Category | Field | Source in OpenRTB |
|----------|-------|-------------------|
| Publisher | `publisher_ids` | `site.publisher.id`, `app.publisher.id` |
| Site/App | `site_domains` | `site.domain` |
| Site/App | `app_bundles` | `app.bundle` |
| Site/App | `placement_ids` | `imp[].tagid` |
| Geographic | `countries` | `device.geo.country` |
| Geographic | `regions` | `device.geo.region` |
| Geographic | `cities` | `device.geo.city` |
| Geographic | `dmas` | `device.geo.dma` |
| Device | `device_types` | `device.devicetype` |
| Device | `os` | `device.os` |
| Device | `os_versions` | `device.osv` |
| Device | `browsers` | Parsed from `device.ua` |
| Device | `device_makes` | `device.make` |
| Device | `device_models` | `device.model` |
| Format | `media_types` | `imp[].banner`, `.video`, `.native`, `.audio` |
| Format | `ad_sizes` | `imp[].banner.w/h`, `.format[].w/h` |
| Network | `connection_types` | `device.connectiontype` |
| Network | `carriers` | `device.carrier` |
| User | `languages` | `device.language` |
| User | `has_user_id` | `user.id`, `user.buyeruid` |
| Privacy | `has_gdpr_consent` | `regs.ext.gdpr`, `user.ext.consent` |
