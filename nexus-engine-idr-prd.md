# Product Requirements Document
## The Nexus Engine: Intelligent Demand Router (IDR)

**Version:** 1.0  
**Author:** The Nexus Engine  
**Date:** December 2024  
**Status:** Draft

---

## Executive Summary

The Intelligent Demand Router (IDR) is the core optimisation layer of The Nexus Engine's Prebid Server implementation. It dynamically routes bid requests to demand partners based on real-time performance data, historical analysis, and predictive scoring—reducing latency, improving fill rates, and maximising yield.

This is The Nexus Engine's primary differentiator: instead of blindly calling all 200+ demand partners on every request (slow, wasteful), IDR intelligently selects the 10-20 partners most likely to respond with a competitive bid for *this specific impression*.

---

## Problem Statement

### Current State (Demand Manager / Vanilla PBS)
- Every bid request fans out to all configured demand partners
- Wastes QPS quotas with partners who rarely/never bid on certain inventory
- Increases latency (waiting for timeouts from irrelevant partners)
- No learning from historical performance
- Treats a UK mobile 300x250 the same as a US desktop 728x90

### Desired State (The Nexus Engine + IDR)
- Bid requests routed to partners with highest probability of responding
- Reduced average auction latency by 40-60%
- Improved effective CPMs through smarter partner selection
- Continuous learning from every auction
- QPS allocated to partners who actually compete

---

## Goals & Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Auction latency (p50) | <200ms | Server logs |
| Auction latency (p95) | <400ms | Server logs |
| Bid response rate | >60% of called partners respond | Auction logs |
| Effective CPM lift | +15-25% vs control | A/B test |
| QPS efficiency | 50% reduction in outbound requests | Partner logs |
| Fill rate | Maintain or improve | Ad server reports |

---

## User Stories

### Publisher (BizBudding)
- As a publisher, I want faster auctions so my page load times improve
- As a publisher, I want higher CPMs without adding more demand partners
- As a publisher, I want to understand which partners perform best on my inventory

### The Nexus Engine (Internal)
- As the platform, I want to optimise QPS usage across partners
- As the platform, I want to build a defensible data moat from auction intelligence
- As the platform, I want to demonstrate clear value vs Demand Manager

---

## Technical Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         PREBID.JS (Client)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  ID Modules  │  │   Consent    │  │  S2S Config  │               │
│  │  (UID2, ID5) │  │  (TCF, GPP)  │  │  (→ PBS)     │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    THE NEXUS ENGINE PBS                             │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │              INTELLIGENT DEMAND ROUTER (IDR)                │    │
│  │                                                             │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │    │
│  │  │  Request    │  │   Bidder    │  │   Partner   │        │    │
│  │  │  Classifier │→ │   Scorer    │→ │   Selector  │        │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │    │
│  │         │                │                │                │    │
│  │         ▼                ▼                ▼                │    │
│  │  ┌─────────────────────────────────────────────────────┐  │    │
│  │  │              PERFORMANCE DATABASE                    │  │    │
│  │  │  (Redis + TimescaleDB)                              │  │    │
│  │  └─────────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                │                                    │
│                                ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │                    PBS AUCTION ENGINE                       │    │
│  │         (Only selected partners receive requests)           │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                │                                    │
└────────────────────────────────┼────────────────────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
        ┌──────────┐      ┌──────────┐      ┌──────────┐
        │ AppNexus │      │ Rubicon  │      │ PubMatic │  ... (selected partners only)
        └──────────┘      └──────────┘      └──────────┘
```

---

## Component Specifications

### 1. Request Classifier

**Purpose:** Extract and normalise features from incoming bid requests for scoring.

**Input:** OpenRTB 2.x Bid Request

**Output:** Classified Request Object

```typescript
interface ClassifiedRequest {
  // Impression attributes
  impressionId: string;
  adFormat: 'banner' | 'video' | 'native' | 'audio';
  adSizes: string[];           // e.g., ['300x250', '320x50']
  position: 'atf' | 'btf' | 'unknown';
  
  // Environment
  deviceType: 'desktop' | 'mobile' | 'tablet' | 'ctv';
  os: string;                  // e.g., 'ios', 'android', 'windows'
  browser: string;             // e.g., 'chrome', 'safari'
  connectionType: string;      // e.g., 'wifi', '4g'
  
  // Geo
  country: string;             // ISO 3166-1 alpha-2
  region: string;              // ISO 3166-2
  dma: string | null;          // US only
  
  // Publisher
  publisherId: string;
  siteId: string;
  domain: string;
  pageType: string | null;     // e.g., 'article', 'homepage'
  categories: string[];        // IAB categories
  
  // User
  userIds: Record<string, string>;  // e.g., { uid2: '...', id5: '...' }
  hasConsent: boolean;
  consentString: string | null;
  
  // Context
  timestamp: Date;
  hourOfDay: number;           // 0-23
  dayOfWeek: number;           // 0-6
  
  // Floors
  floorPrice: number | null;
  floorCurrency: string;
}
```

**Classification Logic:**
```python
def classify_request(ortb_request: dict) -> ClassifiedRequest:
    imp = ortb_request['imp'][0]
    device = ortb_request.get('device', {})
    site = ortb_request.get('site', {})
    user = ortb_request.get('user', {})
    
    return ClassifiedRequest(
        impression_id=imp['id'],
        ad_format=determine_format(imp),
        ad_sizes=extract_sizes(imp),
        position=determine_position(imp),
        device_type=classify_device(device),
        os=device.get('os', '').lower(),
        browser=extract_browser(device.get('ua', '')),
        country=ortb_request.get('geo', {}).get('country', 'unknown'),
        # ... etc
    )
```

---

### 2. Bidder Scorer

**Purpose:** Score each potential bidder (0-100) based on likelihood of responding with a competitive bid.

**Scoring Dimensions:**

| Dimension | Weight | Description |
|-----------|--------|-------------|
| Historical Win Rate | 25% | How often this bidder wins for similar requests |
| Bid Rate | 20% | How often this bidder responds at all |
| Avg Bid CPM | 15% | Average bid value for similar inventory |
| Floor Clearance | 15% | Likelihood of clearing the floor price |
| Latency Score | 10% | Response time reliability |
| Recency | 10% | Recent performance (last 24h weighted higher) |
| ID Match Rate | 5% | User ID availability for this bidder |

**Scoring Algorithm:**

```python
class BidderScorer:
    def __init__(self, performance_db: PerformanceDB):
        self.db = performance_db
    
    def score_bidder(
        self, 
        bidder_code: str, 
        request: ClassifiedRequest
    ) -> BidderScore:
        
        # Build lookup key for historical data
        lookup_key = self._build_lookup_key(request)
        
        # Fetch historical metrics
        metrics = self.db.get_bidder_metrics(bidder_code, lookup_key)
        
        # Calculate component scores (0-100 each)
        win_rate_score = self._score_win_rate(metrics.win_rate)
        bid_rate_score = self._score_bid_rate(metrics.bid_rate)
        cpm_score = self._score_avg_cpm(metrics.avg_cpm, request.floor_price)
        floor_score = self._score_floor_clearance(metrics.floor_clearance_rate)
        latency_score = self._score_latency(metrics.p95_latency)
        recency_score = self._score_recency(metrics.last_24h_performance)
        id_match_score = self._score_id_match(bidder_code, request.user_ids)
        
        # Weighted composite
        total_score = (
            win_rate_score * 0.25 +
            bid_rate_score * 0.20 +
            cpm_score * 0.15 +
            floor_score * 0.15 +
            latency_score * 0.10 +
            recency_score * 0.10 +
            id_match_score * 0.05
        )
        
        return BidderScore(
            bidder_code=bidder_code,
            total_score=total_score,
            components={
                'win_rate': win_rate_score,
                'bid_rate': bid_rate_score,
                'cpm': cpm_score,
                'floor_clearance': floor_score,
                'latency': latency_score,
                'recency': recency_score,
                'id_match': id_match_score
            },
            confidence=metrics.sample_size_confidence
        )
    
    def _build_lookup_key(self, request: ClassifiedRequest) -> LookupKey:
        """
        Hierarchical lookup with fallback:
        1. Exact match: country + device + format + size
        2. Broader: country + device + format
        3. Broadest: country + device
        """
        return LookupKey(
            country=request.country,
            device_type=request.device_type,
            ad_format=request.ad_format,
            ad_size=request.ad_sizes[0] if request.ad_sizes else None,
            publisher_id=request.publisher_id
        )
    
    def _score_win_rate(self, win_rate: float) -> float:
        # 20%+ win rate = 100, 0% = 0, linear scale
        return min(100, win_rate * 500)
    
    def _score_bid_rate(self, bid_rate: float) -> float:
        # 80%+ bid rate = 100, 0% = 0
        return min(100, bid_rate * 125)
    
    def _score_avg_cpm(self, avg_cpm: float, floor: float | None) -> float:
        if floor is None:
            # No floor, score based on absolute CPM
            return min(100, avg_cpm * 20)  # £5 CPM = 100
        # Score based on headroom above floor
        headroom = (avg_cpm - floor) / floor if floor > 0 else 0
        return min(100, max(0, 50 + headroom * 100))
    
    def _score_floor_clearance(self, clearance_rate: float) -> float:
        return clearance_rate * 100
    
    def _score_latency(self, p95_latency_ms: float) -> float:
        # <100ms = 100, >500ms = 0
        if p95_latency_ms <= 100:
            return 100
        if p95_latency_ms >= 500:
            return 0
        return 100 - ((p95_latency_ms - 100) / 4)
    
    def _score_recency(self, last_24h: RecentMetrics) -> float:
        # Boost/penalise based on recent trend vs historical
        if last_24h.sample_size < 100:
            return 50  # Not enough data, neutral
        trend = last_24h.win_rate / last_24h.historical_win_rate
        return min(100, max(0, 50 * trend))
    
    def _score_id_match(
        self, 
        bidder_code: str, 
        user_ids: dict
    ) -> float:
        # Check if bidder's preferred ID types are present
        bidder_id_prefs = ID_PREFERENCES.get(bidder_code, ['uid2', 'id5'])
        matches = sum(1 for id_type in bidder_id_prefs if id_type in user_ids)
        return (matches / len(bidder_id_prefs)) * 100 if bidder_id_prefs else 50
```

---

### 3. Partner Selector

**Purpose:** Select which bidders to include in the auction based on scores.

**Selection Strategy:**

```python
class PartnerSelector:
    def __init__(self, config: SelectorConfig):
        self.config = config
    
    def select_partners(
        self,
        scores: list[BidderScore],
        request: ClassifiedRequest
    ) -> list[str]:
        
        # Sort by score descending
        ranked = sorted(scores, key=lambda x: x.total_score, reverse=True)
        
        selected = []
        
        # 1. Always include "anchor" bidders (top 3 by historical revenue)
        anchors = self._get_anchor_bidders(request.publisher_id)
        for bidder in anchors:
            if bidder in [s.bidder_code for s in ranked]:
                selected.append(bidder)
        
        # 2. Add high-confidence high-scorers
        for score in ranked:
            if score.bidder_code in selected:
                continue
            if len(selected) >= self.config.max_bidders:
                break
            
            # Must meet minimum score threshold
            if score.total_score < self.config.min_score_threshold:
                continue
            
            # Low confidence = exploration (include anyway sometimes)
            if score.confidence < 0.5:
                if random.random() > self.config.exploration_rate:
                    continue
            
            selected.append(score.bidder_code)
        
        # 3. Ensure minimum diversity (at least N different SSP types)
        selected = self._ensure_diversity(selected, ranked)
        
        # 4. Add exploration slots (random low-scorers to gather data)
        if len(selected) < self.config.max_bidders:
            exploration_candidates = [
                s.bidder_code for s in ranked 
                if s.bidder_code not in selected
                and s.confidence < 0.3
            ]
            exploration_count = min(
                self.config.exploration_slots,
                self.config.max_bidders - len(selected),
                len(exploration_candidates)
            )
            selected.extend(random.sample(exploration_candidates, exploration_count))
        
        return selected
    
    def _get_anchor_bidders(self, publisher_id: str) -> list[str]:
        """Top 3 revenue-generating bidders for this publisher."""
        # Query from performance DB
        return self.db.get_top_bidders_by_revenue(publisher_id, limit=3)
    
    def _ensure_diversity(
        self, 
        selected: list[str], 
        all_scores: list[BidderScore]
    ) -> list[str]:
        """Ensure we have representation from different SSP categories."""
        categories_present = {BIDDER_CATEGORIES.get(b) for b in selected}
        
        for category in ['premium', 'mid_tier', 'video_specialist', 'native']:
            if category not in categories_present:
                # Find best scorer in this category
                for score in all_scores:
                    if BIDDER_CATEGORIES.get(score.bidder_code) == category:
                        if score.bidder_code not in selected:
                            selected.append(score.bidder_code)
                            break
        
        return selected
```

**Configuration:**

```yaml
selector:
  max_bidders: 15              # Maximum partners per auction
  min_score_threshold: 25      # Minimum score to be considered
  exploration_rate: 0.1        # 10% chance to include low-confidence bidders
  exploration_slots: 2         # Reserved slots for data gathering
  anchor_bidder_count: 3       # Always-include top performers
```

---

### 4. Performance Database

**Purpose:** Store and query historical bidder performance metrics.

**Technology:** Redis (real-time) + TimescaleDB (historical analytics)

**Data Model:**

```sql
-- TimescaleDB hypertable for auction events
CREATE TABLE auction_events (
    timestamp TIMESTAMPTZ NOT NULL,
    auction_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL,
    
    -- Request attributes
    country TEXT,
    device_type TEXT,
    ad_format TEXT,
    ad_size TEXT,
    floor_price DECIMAL(10,4),
    
    -- Bidder response
    bidder_code TEXT NOT NULL,
    did_bid BOOLEAN,
    bid_cpm DECIMAL(10,4),
    response_time_ms INTEGER,
    did_win BOOLEAN,
    
    -- IDs present
    user_ids JSONB
);

SELECT create_hypertable('auction_events', 'timestamp');

-- Materialised view for quick lookups
CREATE MATERIALIZED VIEW bidder_performance_hourly AS
SELECT
    time_bucket('1 hour', timestamp) AS hour,
    publisher_id,
    country,
    device_type,
    ad_format,
    ad_size,
    bidder_code,
    
    COUNT(*) AS request_count,
    SUM(CASE WHEN did_bid THEN 1 ELSE 0 END) AS bid_count,
    SUM(CASE WHEN did_win THEN 1 ELSE 0 END) AS win_count,
    AVG(bid_cpm) FILTER (WHERE did_bid) AS avg_cpm,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms) AS p95_latency,
    SUM(CASE WHEN did_bid AND bid_cpm >= floor_price THEN 1 ELSE 0 END)::FLOAT / 
        NULLIF(SUM(CASE WHEN did_bid THEN 1 ELSE 0 END), 0) AS floor_clearance_rate

FROM auction_events
GROUP BY 1, 2, 3, 4, 5, 6, 7;

-- Index for fast lookups
CREATE INDEX idx_perf_lookup ON bidder_performance_hourly 
    (publisher_id, country, device_type, ad_format, bidder_code, hour DESC);
```

**Redis Schema (real-time counters):**

```
# Sliding window counters (last 1 hour)
bidder:{bidder_code}:pub:{pub_id}:requests    -> HyperLogLog
bidder:{bidder_code}:pub:{pub_id}:bids        -> HyperLogLog  
bidder:{bidder_code}:pub:{pub_id}:wins        -> HyperLogLog
bidder:{bidder_code}:pub:{pub_id}:cpm_sum     -> Float
bidder:{bidder_code}:pub:{pub_id}:latency     -> Sorted Set (for percentiles)

# Lookup cache (TTL 5 min)
score:{pub_id}:{country}:{device}:{format}:{bidder} -> JSON BidderScore
```

---

### 5. Event Pipeline

**Purpose:** Capture auction outcomes and feed the learning loop.

```python
class AuctionEventPipeline:
    def __init__(self, redis: Redis, timescale: Connection):
        self.redis = redis
        self.timescale = timescale
        self.buffer = []
        self.buffer_size = 1000
    
    async def record_auction(self, auction: AuctionResult):
        """Called after every auction completes."""
        
        for bidder_response in auction.bidder_responses:
            event = AuctionEvent(
                timestamp=auction.timestamp,
                auction_id=auction.id,
                publisher_id=auction.publisher_id,
                country=auction.request.country,
                device_type=auction.request.device_type,
                ad_format=auction.request.ad_format,
                ad_size=auction.request.ad_sizes[0],
                floor_price=auction.request.floor_price,
                bidder_code=bidder_response.bidder,
                did_bid=bidder_response.bid is not None,
                bid_cpm=bidder_response.bid.cpm if bidder_response.bid else None,
                response_time_ms=bidder_response.response_time_ms,
                did_win=bidder_response.bidder == auction.winner,
                user_ids=auction.request.user_ids
            )
            
            # Update real-time counters
            await self._update_redis_counters(event)
            
            # Buffer for batch insert
            self.buffer.append(event)
            
            if len(self.buffer) >= self.buffer_size:
                await self._flush_to_timescale()
    
    async def _update_redis_counters(self, event: AuctionEvent):
        key_prefix = f"bidder:{event.bidder_code}:pub:{event.publisher_id}"
        
        pipe = self.redis.pipeline()
        pipe.pfadd(f"{key_prefix}:requests", event.auction_id)
        
        if event.did_bid:
            pipe.pfadd(f"{key_prefix}:bids", event.auction_id)
            pipe.incrbyfloat(f"{key_prefix}:cpm_sum", event.bid_cpm)
            pipe.zadd(f"{key_prefix}:latency", {event.auction_id: event.response_time_ms})
        
        if event.did_win:
            pipe.pfadd(f"{key_prefix}:wins", event.auction_id)
        
        # Expire keys after 2 hours
        for key in [f"{key_prefix}:requests", f"{key_prefix}:bids", 
                    f"{key_prefix}:wins", f"{key_prefix}:cpm_sum", 
                    f"{key_prefix}:latency"]:
            pipe.expire(key, 7200)
        
        await pipe.execute()
    
    async def _flush_to_timescale(self):
        if not self.buffer:
            return
        
        # Batch insert
        await self.timescale.copy_records_to_table(
            'auction_events',
            records=[e.to_tuple() for e in self.buffer]
        )
        self.buffer = []
```

---

## Integration Points

### PBS Hook Integration

The IDR integrates with Prebid Server via the **processed auction request hook**:

```go
// In PBS: modules/nexus-idr/module.go

package nexusidr

import (
    "github.com/prebid/prebid-server/hooks/hookstage"
)

func (m *Module) HandleProcessedAuctionHook(
    ctx context.Context,
    payload hookstage.ProcessedAuctionRequestPayload,
) (hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload], error) {
    
    // 1. Classify the request
    classified := m.classifier.Classify(payload.BidRequest)
    
    // 2. Score all configured bidders
    scores := make([]BidderScore, 0)
    for _, bidder := range payload.BidderRequests {
        score := m.scorer.Score(bidder.BidderCode, classified)
        scores = append(scores, score)
    }
    
    // 3. Select partners
    selected := m.selector.Select(scores, classified)
    
    // 4. Filter bidder requests
    filteredRequests := make([]BidderRequest, 0)
    for _, req := range payload.BidderRequests {
        if contains(selected, req.BidderCode) {
            filteredRequests = append(filteredRequests, req)
        }
    }
    
    payload.BidderRequests = filteredRequests
    
    // 5. Log routing decision for analysis
    m.logger.LogRoutingDecision(classified, scores, selected)
    
    return hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{
        ModuleContext: ctx,
        Payload:       payload,
    }, nil
}
```

---

## API Endpoints

### Analytics Dashboard API

```yaml
GET /api/v1/analytics/routing
  description: Get routing performance metrics
  params:
    publisher_id: string (required)
    start_date: ISO date
    end_date: ISO date
    granularity: hour | day | week
  response:
    total_auctions: int
    avg_bidders_called: float
    avg_latency_ms: float
    fill_rate: float
    avg_cpm: float
    bidder_breakdown: [
      {
        bidder_code: string
        times_called: int
        bid_rate: float
        win_rate: float
        avg_cpm: float
      }
    ]

GET /api/v1/analytics/bidder/{bidder_code}
  description: Deep dive on specific bidder performance
  params:
    publisher_id: string
    dimensions: [country, device, format, size]
  response:
    overall_score: float
    score_components: {...}
    performance_by_dimension: {...}
    trend_7d: [...]

POST /api/v1/config/routing
  description: Update routing configuration
  body:
    publisher_id: string
    max_bidders: int
    min_score_threshold: float
    always_include: [string]  # Bidders to always call
    never_include: [string]   # Bidders to never call
```

---

## Rollout Plan

### Phase 1: Shadow Mode (Week 1-2)
- Deploy IDR alongside existing PBS
- Log routing decisions without actually filtering
- Compare "would have called" vs "actually called"
- Validate scoring accuracy

### Phase 2: Limited Traffic (Week 3-4)
- Route 10% of BizBudding traffic through IDR
- A/B test against control (full bidder calls)
- Monitor: latency, fill rate, CPM, errors

### Phase 3: Ramp Up (Week 5-6)
- Increase to 50% traffic
- Fine-tune scoring weights based on learnings
- Add publisher-specific overrides

### Phase 4: Full Deployment (Week 7+)
- 100% traffic through IDR
- Continuous optimisation
- Expand to additional publishers

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Cold start (no data) | Poor routing decisions | Default to calling all partners until 10K auctions |
| Scoring model bias | Miss good bidders | Exploration slots + regular model review |
| Data pipeline failure | Stale scores | Fallback to last-known-good, alert on staleness |
| Latency added by IDR | Slower auctions | Target <5ms for routing decision, cache aggressively |
| Partner relationships | Upset SSPs getting less traffic | Transparent reporting, maintain minimum call rates |

---

## Success Criteria for BizBudding Trial

After 2-week A/B test:

1. **Latency**: p50 auction time reduced by >30%
2. **CPM**: Effective CPM equal or higher than Demand Manager
3. **Fill Rate**: No degradation vs control
4. **QPS**: 40%+ reduction in outbound bid requests

---

## Appendix

### A. Bidder Categories

```python
BIDDER_CATEGORIES = {
    # Premium
    'appnexus': 'premium',
    'rubicon': 'premium', 
    'pubmatic': 'premium',
    'openx': 'premium',
    'ix': 'premium',
    
    # Mid-tier
    'triplelift': 'mid_tier',
    'sovrn': 'mid_tier',
    'sharethrough': 'mid_tier',
    'gumgum': 'mid_tier',
    '33across': 'mid_tier',
    
    # Video specialists
    'spotx': 'video_specialist',
    'beachfront': 'video_specialist',
    'unruly': 'video_specialist',
    
    # Native specialists  
    'teads': 'native',
    'outbrain': 'native',
    'taboola': 'native',
    
    # Regional
    'adform': 'emea',
    'smartadserver': 'emea',
    'improvedigital': 'emea',
}
```

### B. ID Type Preferences by Bidder

```python
ID_PREFERENCES = {
    'appnexus': ['uid2', 'liveramp', 'id5'],
    'rubicon': ['liveramp', 'uid2', 'id5'],
    'pubmatic': ['uid2', 'id5', 'sharedid'],
    'ix': ['id5', 'uid2', 'liveramp'],
    'triplelift': ['uid2', 'id5'],
    'criteo': ['criteo'],  # Criteo prefers their own
    # ... etc
}
```

### C. Sample Scoring Output

```json
{
  "auction_id": "abc123",
  "request_classification": {
    "country": "GB",
    "device_type": "mobile",
    "ad_format": "banner",
    "ad_size": "300x250",
    "floor_price": 0.50
  },
  "bidder_scores": [
    {
      "bidder_code": "rubicon",
      "total_score": 87.3,
      "components": {
        "win_rate": 92,
        "bid_rate": 85,
        "cpm": 78,
        "floor_clearance": 95,
        "latency": 88,
        "recency": 82,
        "id_match": 100
      },
      "confidence": 0.95
    },
    {
      "bidder_code": "appnexus", 
      "total_score": 81.2,
      "components": {...}
    }
  ],
  "selected_bidders": ["rubicon", "appnexus", "pubmatic", "ix", "triplelift"],
  "excluded_bidders": ["spotx", "beachfront"],  // Video specialists, not relevant
  "routing_time_ms": 2.3
}
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | Dec 2024 | The Nexus Engine | Initial draft |
