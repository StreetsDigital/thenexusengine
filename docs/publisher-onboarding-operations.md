# Publisher Onboarding Operations Guide

Complete operational guide for onboarding publishers, sites, and ad units, and connecting them to demand sources.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Step 1: Create the Publisher](#step-1-create-the-publisher)
4. [Step 2: Add Sites](#step-2-add-sites)
5. [Step 3: Configure Ad Units](#step-3-configure-ad-units)
6. [Step 4: Connect Demand Sources](#step-4-connect-demand-sources)
7. [Step 5: Verify and Test](#step-5-verify-and-test)
8. [Step 6: Go Live](#step-6-go-live)
9. [Advanced Configuration](#advanced-configuration)
10. [Quick Reference](#quick-reference)

---

## Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        INVENTORY (Supply)                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   Publisher                                                           │
│       └── Site (domain)                                              │
│             └── Ad Unit (placement with sizes/format)                │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                   IDR (Intelligent Demand Router)                    │
│                 ML-powered bidder selection per auction              │
└─────────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         DEMAND (Bidders)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   Static Bidders (22 built-in)     Dynamic Bidders (oRTB)           │
│   ├── AppNexus                     ├── Custom DSPs                  │
│   ├── Rubicon                      ├── Regional partners            │
│   ├── PubMatic                     └── Any OpenRTB 2.5/2.6 endpoint │
│   ├── OpenX                                                          │
│   ├── Index Exchange                                                 │
│   ├── TripleLift                                                     │
│   ├── SpotX                                                          │
│   └── ... (15 more)                                                  │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Configuration Methods

| Method | Best For | Location |
|--------|----------|----------|
| **YAML Files** | Initial setup, version control, bulk config | `config/publishers/*.yaml` |
| **REST API** | Runtime changes, automation, UI integration | `http://localhost:5050/api/v2/config/*` |
| **Database Direct** | Migrations, bulk imports | `config_publishers`, `config_sites`, `config_ad_units` tables |

---

## Prerequisites

Before onboarding a publisher, ensure you have:

- [ ] Nexus Engine running (PBS + IDR services)
- [ ] Publisher's domain(s) and site information
- [ ] Ad unit specifications (sizes, formats, positions)
- [ ] Bidder credentials from demand partners (placement IDs, account IDs, etc.)

---

## Step 1: Create the Publisher

### Option A: YAML Configuration (Recommended for New Publishers)

Create a new file: `config/publishers/{publisher-id}.yaml`

```yaml
# config/publishers/acme-media.yaml
# =============================================
# PUBLISHER INFORMATION
# =============================================
publisher_id: "acme-media"           # Unique ID (lowercase, alphanumeric, hyphens)
name: "ACME Media Group"             # Display name
enabled: true                        # Set to false to pause all traffic

contact:
  email: "adops@acmemedia.com"       # AdOps contact for alerts
  name: "ACME Ad Operations"

# Sites and ad units will be added below...
sites: []

# Bidders will be configured below...
bidders: {}
```

**Required Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `publisher_id` | string | Unique identifier (lowercase, alphanumeric, hyphens only) |
| `name` | string | Human-readable display name |

**Optional Fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable/disable publisher |
| `contact.email` | string | | AdOps contact email |
| `contact.name` | string | | AdOps contact name |
| `api_key` | string | | API key for authentication (auto-generated if blank) |

### Option B: REST API

```bash
# Create publisher via API
curl -X PUT "http://localhost:5050/api/v2/config/publishers/acme-media" \
  -H "Content-Type: application/json" \
  -d '{
    "publisher_id": "acme-media",
    "name": "ACME Media Group",
    "enabled": true,
    "contact_email": "adops@acmemedia.com",
    "contact_name": "ACME Ad Operations"
  }'
```

### Option C: Database Direct

```sql
INSERT INTO config_publishers (publisher_id, name, enabled, contact_email, contact_name)
VALUES ('acme-media', 'ACME Media Group', true, 'adops@acmemedia.com', 'ACME Ad Operations');
```

---

## Step 2: Add Sites

Each publisher can have multiple sites (domains). Add sites to the publisher configuration.

### YAML Configuration

```yaml
# In config/publishers/acme-media.yaml
sites:
  # ---------------------------------------------
  # Main Website
  # ---------------------------------------------
  - site_id: "main-site"             # Unique within this publisher
    domain: "www.acmemedia.com"      # Actual domain (used for validation)
    name: "ACME Main Site"           # Display name
    enabled: true
    ad_units: []                     # Will add in Step 3

  # ---------------------------------------------
  # Mobile Website
  # ---------------------------------------------
  - site_id: "mobile"
    domain: "m.acmemedia.com"
    name: "ACME Mobile"
    enabled: true
    ad_units: []

  # ---------------------------------------------
  # News Section (subdomain)
  # ---------------------------------------------
  - site_id: "news"
    domain: "news.acmemedia.com"
    name: "ACME News"
    enabled: true
    ad_units: []
```

**Required Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `site_id` | string | Unique identifier within publisher |
| `domain` | string | Website domain (without protocol) |

**Optional Fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | | Display name |
| `enabled` | bool | `true` | Enable/disable site |
| `features` | object | | Site-level feature overrides (see Advanced Configuration) |

### REST API

```bash
# Create site via API
curl -X PUT "http://localhost:5050/api/v2/config/publishers/acme-media/sites/main-site" \
  -H "Content-Type: application/json" \
  -d '{
    "site_id": "main-site",
    "domain": "www.acmemedia.com",
    "name": "ACME Main Site",
    "enabled": true
  }'
```

---

## Step 3: Configure Ad Units

Ad units define the actual placements where ads will appear.

### YAML Configuration

```yaml
# In config/publishers/acme-media.yaml, under each site
sites:
  - site_id: "main-site"
    domain: "www.acmemedia.com"
    name: "ACME Main Site"
    enabled: true

    ad_units:
      # ---------------------------------------------
      # Banner: Leaderboard (Top of page)
      # ---------------------------------------------
      - unit_id: "leaderboard-top"
        name: "Top Leaderboard"
        sizes: [[728, 90], [970, 90], [970, 250]]
        media_type: "banner"
        position: "atf"              # above-the-fold
        floor_price: 1.50
        floor_currency: "USD"

      # ---------------------------------------------
      # Banner: Medium Rectangle (Sidebar)
      # ---------------------------------------------
      - unit_id: "sidebar-mrec"
        name: "Sidebar MREC"
        sizes: [[300, 250], [300, 600]]
        media_type: "banner"
        position: "atf"
        floor_price: 0.75
        floor_currency: "USD"

      # ---------------------------------------------
      # Banner: In-Article
      # ---------------------------------------------
      - unit_id: "in-article"
        name: "In-Article Banner"
        sizes: [[300, 250], [336, 280]]
        media_type: "banner"
        position: "btf"              # below-the-fold
        floor_price: 0.50
        floor_currency: "USD"

      # ---------------------------------------------
      # Video: Outstream
      # ---------------------------------------------
      - unit_id: "outstream-video"
        name: "Outstream Video"
        sizes: [[640, 480]]
        media_type: "video"
        position: "btf"
        floor_price: 5.00
        floor_currency: "USD"
        video:
          context: "outstream"       # or "instream"
          mimes: ["video/mp4", "video/webm"]
          protocols: [2, 3, 5, 6]    # VAST 2.0, 3.0 wrapper/inline
          playback_method: [1, 2]    # 1=autoplay, 2=click
          max_duration: 30

      # ---------------------------------------------
      # Native: In-Feed
      # ---------------------------------------------
      - unit_id: "native-infeed"
        name: "Native In-Feed"
        sizes: [[1, 1]]              # Native always uses 1x1
        media_type: "native"
        position: "btf"
        floor_price: 0.60
        floor_currency: "USD"
```

### Ad Unit Reference

**Required Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `unit_id` | string | Unique within site |
| `sizes` | array | List of `[width, height]` pairs |
| `media_type` | string | `banner`, `video`, `native`, `audio` |

**Optional Fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | | Display name |
| `enabled` | bool | `true` | Enable/disable unit |
| `position` | string | `unknown` | `atf`, `btf`, `interstitial`, `sidebar`, etc. |
| `floor_price` | float | | Minimum bid price |
| `floor_currency` | string | `USD` | Currency for floor |
| `video` | object | | Video-specific settings (required for video) |
| `features` | object | | Unit-level feature overrides |

**Video Configuration:**
```yaml
video:
  context: "outstream"           # outstream, instream
  mimes: ["video/mp4", "video/webm"]
  protocols: [2, 3, 5, 6]        # VAST versions (2=2.0, 3=3.0, 5=2.0 wrapper, 6=3.0 wrapper)
  playback_method: [1, 2]        # 1=autoplay sound on, 2=autoplay muted, 3=click, 4=mouse-over
  max_duration: 30               # Max video length in seconds
  min_duration: 5                # Min video length (optional)
  skip: 1                        # Allow skip (0=no, 1=yes)
  skipafter: 5                   # Skip after N seconds
```

**Common Ad Sizes:**
| Placement | Sizes |
|-----------|-------|
| Leaderboard | `[[728, 90], [970, 90]]` |
| Billboard | `[[970, 250]]` |
| Medium Rectangle | `[[300, 250], [336, 280]]` |
| Half Page | `[[300, 600]]` |
| Mobile Banner | `[[320, 50], [300, 50]]` |
| Mobile Interstitial | `[[320, 480], [300, 250]]` |
| Native | `[[1, 1]]` |
| Video | `[[640, 480], [640, 360]]` |

### REST API

```bash
# Create ad unit via API
curl -X PUT "http://localhost:5050/api/v2/config/publishers/acme-media/sites/main-site/ad-units/leaderboard-top" \
  -H "Content-Type: application/json" \
  -d '{
    "unit_id": "leaderboard-top",
    "name": "Top Leaderboard",
    "sizes": [[728, 90], [970, 90], [970, 250]],
    "media_type": "banner",
    "position": "atf",
    "floor_price": 1.50,
    "floor_currency": "USD"
  }'
```

---

## Step 4: Connect Demand Sources

### A. Static Bidders (Built-in PBS Adapters)

Configure credentials for the 22 built-in bidder adapters.

```yaml
# In config/publishers/acme-media.yaml
bidders:
  # ---------------------------------------------
  # AppNexus / Xandr
  # ---------------------------------------------
  appnexus:
    enabled: true
    params:
      placement_id: 12345678       # Get from AppNexus
    # Optional: per-site overrides
    site_overrides:
      mobile:
        placement_id: 87654321

  # ---------------------------------------------
  # Rubicon / Magnite
  # ---------------------------------------------
  rubicon:
    enabled: true
    params:
      account_id: 1001             # Get from Rubicon
      site_id: 2002
      zone_id: 3003

  # ---------------------------------------------
  # PubMatic
  # ---------------------------------------------
  pubmatic:
    enabled: true
    params:
      publisher_id: "pub-12345"    # Get from PubMatic
      ad_slot: "header_slot"       # Optional slot name

  # ---------------------------------------------
  # OpenX
  # ---------------------------------------------
  openx:
    enabled: true
    params:
      unit: "540123456"            # Get from OpenX
      del_domain: "acme.openx.net"

  # ---------------------------------------------
  # Index Exchange
  # ---------------------------------------------
  ix:
    enabled: true
    params:
      site_id: "123456"            # Get from Index Exchange

  # ---------------------------------------------
  # TripleLift (Native specialist)
  # ---------------------------------------------
  triplelift:
    enabled: true
    params:
      inventory_code: "acme_native"

  # ---------------------------------------------
  # SpotX (Video specialist)
  # ---------------------------------------------
  spotx:
    enabled: true
    params:
      channel_id: "123456"         # Get from SpotX

  # ---------------------------------------------
  # Sharethrough (Native)
  # ---------------------------------------------
  sharethrough:
    enabled: true
    params:
      pkey: "your-pkey"

  # ---------------------------------------------
  # Criteo
  # ---------------------------------------------
  criteo:
    enabled: true
    params:
      zone_id: 123456
      network_id: 789
```

**Available Static Bidders (22):**

| Bidder | Media Types | Required Params |
|--------|-------------|-----------------|
| `appnexus` | banner, video, native | `placement_id` |
| `rubicon` | banner, video | `account_id`, `site_id`, `zone_id` |
| `pubmatic` | banner, video, native | `publisher_id` |
| `openx` | banner, video | `unit`, `del_domain` |
| `ix` | banner, video | `site_id` |
| `triplelift` | banner, native | `inventory_code` |
| `spotx` | video | `channel_id` |
| `conversant` | banner, video | `site_id` |
| `sharethrough` | banner, native | `pkey` |
| `outbrain` | native | `publisher_id`, `section_id` |
| `taboola` | native | `publisher_id` |
| `adform` | banner | `mid` |
| `improvedigital` | banner, video | `placement_id` |
| `sovrn` | banner | `tag_id` |
| `unruly` | video | `site_id` |
| `smartadserver` | banner, video | `site_id`, `page_id`, `format_id` |
| `gumgum` | banner, video | `zone`, `pub_id` |
| `beachfront` | banner, video | `app_id`, `video_response_type` |
| `criteo` | banner | `zone_id`, `network_id` |
| `medianet` | banner | `cid`, `crid` |

### B. Dynamic Bidders (Custom OpenRTB)

For demand partners not in the built-in list, create dynamic OpenRTB bidders.

**Via Admin UI:** Navigate to `http://localhost:5050/bidders` and click "Add Bidder"

**Via API:**

```bash
# Create a custom OpenRTB bidder
curl -X POST "http://localhost:5050/api/bidders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -d '{
    "bidder_code": "custom-dsp",
    "name": "Custom DSP Partner",
    "description": "Regional DSP for LATAM",
    "endpoint": {
      "url": "https://dsp.example.com/openrtb/bid",
      "method": "POST",
      "timeout_ms": 200,
      "protocol_version": "2.5",
      "auth_type": "bearer",
      "auth_token": "your-api-token"
    },
    "capabilities": {
      "media_types": ["banner", "video"],
      "currencies": ["USD", "BRL"],
      "site_enabled": true,
      "app_enabled": false,
      "supports_gdpr": true,
      "supports_ccpa": true,
      "supports_coppa": true
    },
    "status": "testing",
    "priority": 50,
    "allowed_publishers": ["acme-media"],
    "allowed_countries": ["BR", "MX", "AR"]
  }'
```

**Dynamic Bidder Configuration Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `bidder_code` | Yes | Unique identifier (lowercase, alphanumeric, underscores) |
| `name` | Yes | Display name |
| `endpoint.url` | Yes | OpenRTB endpoint URL |
| `endpoint.timeout_ms` | Yes | Request timeout (100-500ms recommended) |
| `capabilities.media_types` | Yes | Supported formats: `banner`, `video`, `native`, `audio` |
| `status` | Yes | `testing`, `active`, `paused`, `disabled` |

**Authentication Types:**
- `none` - No authentication
- `bearer` - Bearer token in Authorization header
- `basic` - HTTP Basic auth
- `header` - Custom header

### C. Bidder Targeting Rules

Control which bidders can bid on which inventory:

```yaml
# In config/publishers/acme-media.yaml
targeting:
  # Block specific bidders from specific ad units
  blocked:
    - bidder: "spotx"
      ad_units: ["sidebar-mrec", "leaderboard-top"]  # Video bidder shouldn't get banner
    - bidder: "triplelift"
      ad_units: ["outstream-video"]                   # Native bidder shouldn't get video

  # Only allow specific bidders for specific ad units
  allowed:
    - bidder: "spotx"
      ad_units: ["outstream-video"]                   # Only allow on video
    - bidder: "triplelift"
      ad_units: ["native-infeed"]                     # Only allow on native
```

---

## Step 5: Verify and Test

### 1. Sync Configuration to Database

If using YAML files, sync to the database:

```bash
# Via API
curl -X POST "http://localhost:5050/api/v2/config/sync/to-db"
```

### 2. Verify Publisher Configuration

```bash
# Get resolved config for a specific context
curl "http://localhost:5050/api/v2/config/resolve?publisher_id=acme-media&site_id=main-site&unit_id=leaderboard-top"
```

### 3. Test PBS Endpoint

```bash
# Health check
curl "http://localhost:8000/health"

# List available bidders
curl "http://localhost:8000/info/bidders"
```

### 4. Send Test Auction Request

```bash
curl -X POST "http://localhost:8000/openrtb2/auction" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-auction-001",
    "imp": [{
      "id": "imp-1",
      "banner": {
        "w": 728,
        "h": 90,
        "format": [
          {"w": 728, "h": 90},
          {"w": 970, "h": 90}
        ]
      },
      "bidfloor": 0.50,
      "bidfloorcur": "USD",
      "tagid": "leaderboard-top"
    }],
    "site": {
      "domain": "www.acmemedia.com",
      "page": "https://www.acmemedia.com/article/123",
      "publisher": {
        "id": "acme-media"
      }
    },
    "device": {
      "ua": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
      "ip": "192.168.1.1"
    },
    "user": {},
    "at": 1,
    "tmax": 1000,
    "cur": ["USD"]
  }'
```

### 5. Check IDR Selection

```bash
# Check which bidders IDR would select
curl -X POST "http://localhost:5050/api/select" \
  -H "Content-Type: application/json" \
  -d '{
    "publisher_id": "acme-media",
    "site_id": "main-site",
    "unit_id": "leaderboard-top",
    "bidder_list": ["appnexus", "rubicon", "pubmatic", "openx", "ix"]
  }'
```

### 6. Test Dynamic Bidder Endpoint

```bash
curl -X POST "http://localhost:5050/api/bidders/custom-dsp/test"
```

---

## Step 6: Go Live

### 1. Enable the Publisher

```yaml
# In YAML
enabled: true
```

Or via API:

```bash
curl -X PATCH "http://localhost:5050/api/v2/config/publishers/acme-media" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### 2. Activate Dynamic Bidders

```bash
curl -X POST "http://localhost:5050/api/bidders/custom-dsp/enable"
```

### 3. Provide Publisher with Integration Code

Give the publisher their credentials:

| Setting | Value |
|---------|-------|
| Publisher ID | `acme-media` |
| PBS Endpoint | `https://your-pbs-domain.com/openrtb2/auction` |

And the integration code from [Publisher Integration Guide](publisher-integration.md).

### 4. Monitor Traffic

Check the metrics and logs:

```bash
# Prometheus metrics
curl "http://localhost:8000/metrics"

# Check IDR circuit breaker status
curl "http://localhost:8000/admin/circuit-breaker"
```

---

## Advanced Configuration

### Hierarchical Feature Settings

Configure settings at different levels: Global → Publisher → Site → Ad Unit

```yaml
# Publisher-level features (applies to all sites/units)
features:
  config_id: "publisher:acme-media"
  config_level: "publisher"

  # IDR Settings
  idr:
    enabled: true
    max_bidders: 12
    min_score_threshold: 20.0
    exploration_enabled: true
    exploration_rate: 0.15
    anchor_bidders_enabled: true
    custom_anchor_bidders: ["rubicon", "appnexus", "pubmatic"]

  # Privacy Settings
  privacy:
    gdpr_applies: true
    ccpa_applies: true
    coppa_applies: false

  # Floor Settings
  floors:
    floor_enabled: true
    default_floor_price: 0.50
    banner_floor: 0.50
    video_floor: 2.00
    native_floor: 0.75

  # Rate Limits
  rate_limits:
    requests_per_second: 2000
    burst: 200

sites:
  - site_id: "mobile"
    domain: "m.acmemedia.com"
    # Site-level overrides (inherits from publisher, then overrides)
    features:
      config_id: "site:mobile"
      config_level: "site"
      idr:
        max_bidders: 6            # Override: fewer bidders for mobile
        selection_timeout_ms: 30  # Override: faster timeout
      rate_limits:
        requests_per_second: 5000 # Override: higher for mobile traffic

    ad_units:
      - unit_id: "mobile-interstitial"
        name: "Mobile Interstitial"
        sizes: [[320, 480]]
        media_type: "banner"
        position: "interstitial"
        floor_price: 3.00
        # Ad unit-level overrides
        features:
          config_id: "ad_unit:mobile-interstitial"
          config_level: "ad_unit"
          idr:
            max_bidders: 10       # Override: more bidders for high-value
```

### IDR Configuration Options

```yaml
idr:
  enabled: true                   # Enable/disable IDR
  bypass_enabled: false           # If true, select ALL bidders (bypass IDR)
  shadow_mode: false              # Log decisions without filtering
  max_bidders: 12                 # Max bidders per auction
  min_score_threshold: 20.0       # Minimum score to participate
  exploration_enabled: true       # Enable exploration
  exploration_rate: 0.15          # 15% exploration rate
  exploration_slots: 2            # Reserved slots for exploration
  anchor_bidders_enabled: true    # Always include anchor bidders
  anchor_bidder_count: 3          # Number of anchor bidders
  custom_anchor_bidders:          # Specific anchor bidders
    - rubicon
    - appnexus
    - pubmatic
  diversity_enabled: true         # Enable bidder diversity
  diversity_categories:           # Categories for diversity
    - premium
    - mid_tier
    - video_specialist
  scoring_weights:                # ML scoring weights
    win_rate: 0.25
    bid_rate: 0.20
    cpm: 0.15
    floor_clearance: 0.15
    latency: 0.10
    recency: 0.10
    id_match: 0.05
  latency_excellent_ms: 100       # Latency score: 100 if <= this
  latency_poor_ms: 500            # Latency score: 0 if >= this
  selection_timeout_ms: 50        # IDR selection timeout
```

---

## Quick Reference

### API Endpoints Summary

| Operation | Method | Endpoint |
|-----------|--------|----------|
| List publishers | GET | `/api/v2/config/publishers` |
| Get publisher | GET | `/api/v2/config/publishers/{id}` |
| Create/update publisher | PUT | `/api/v2/config/publishers/{id}` |
| Delete publisher | DELETE | `/api/v2/config/publishers/{id}` |
| List sites | GET | `/api/v2/config/publishers/{id}/sites` |
| Get site | GET | `/api/v2/config/publishers/{id}/sites/{site_id}` |
| Create/update site | PUT | `/api/v2/config/publishers/{id}/sites/{site_id}` |
| Delete site | DELETE | `/api/v2/config/publishers/{id}/sites/{site_id}` |
| List ad units | GET | `/api/v2/config/publishers/{id}/sites/{site_id}/ad-units` |
| Get ad unit | GET | `/api/v2/config/publishers/{id}/sites/{site_id}/ad-units/{unit_id}` |
| Create/update ad unit | PUT | `/api/v2/config/publishers/{id}/sites/{site_id}/ad-units/{unit_id}` |
| Delete ad unit | DELETE | `/api/v2/config/publishers/{id}/sites/{site_id}/ad-units/{unit_id}` |
| Resolve config | GET | `/api/v2/config/resolve?publisher_id=X&site_id=Y&unit_id=Z` |
| Sync to DB | POST | `/api/v2/config/sync/to-db` |
| Sync from DB | POST | `/api/v2/config/sync/from-db` |
| List bidders | GET | `/api/bidders` |
| Create bidder | POST | `/api/bidders` |
| Update bidder | PUT | `/api/bidders/{code}` |
| Test bidder | POST | `/api/bidders/{code}/test` |
| Enable bidder | POST | `/api/bidders/{code}/enable` |
| Disable bidder | POST | `/api/bidders/{code}/disable` |

### Complete YAML Template

```yaml
# config/publishers/{publisher-id}.yaml
publisher_id: "publisher-id"
name: "Publisher Name"
enabled: true

contact:
  email: "adops@publisher.com"
  name: "Ad Operations"

features:
  config_id: "publisher:publisher-id"
  config_level: "publisher"
  idr:
    enabled: true
    max_bidders: 12
    exploration_rate: 0.15
  privacy:
    gdpr_applies: true
    ccpa_applies: true
  floors:
    default_floor_price: 0.50
  rate_limits:
    requests_per_second: 1000

sites:
  - site_id: "main"
    domain: "www.publisher.com"
    name: "Main Site"
    enabled: true
    ad_units:
      - unit_id: "leaderboard"
        name: "Leaderboard"
        sizes: [[728, 90], [970, 90]]
        media_type: "banner"
        position: "atf"
        floor_price: 1.00

bidders:
  appnexus:
    enabled: true
    params:
      placement_id: 12345678
  rubicon:
    enabled: true
    params:
      account_id: 1001
      site_id: 2002
      zone_id: 3003

targeting:
  blocked: []
  allowed: []
```

### Checklist

- [ ] Publisher created with unique ID
- [ ] Contact information added
- [ ] Sites added with correct domains
- [ ] Ad units configured with correct sizes and formats
- [ ] Floor prices set appropriately
- [ ] Bidder credentials configured
- [ ] Video settings configured (if using video)
- [ ] Targeting rules set (if needed)
- [ ] Configuration synced to database
- [ ] Test auction request successful
- [ ] Publisher enabled
- [ ] Integration code provided to publisher

---

## Related Documentation

- [Publisher Integration Guide](publisher-integration.md) - Client-side integration code
- [Publisher Quick Start](publisher-quickstart.md) - 5-minute publisher setup
- [OpenRTB Bidder Integration](ortb-bidder-integration.md) - Creating custom bidders
- [Docker Setup](docker-setup.md) - Running the platform
