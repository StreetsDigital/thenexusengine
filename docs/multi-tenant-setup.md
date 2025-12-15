# Multi-Tenant Setup Guide

Run multiple publishers on a single Nexus Engine instance.

## Overview

The Nexus Engine supports multi-tenancy out of the box. Each publisher:
- Has their own configuration file
- Gets separate metrics tracking
- Can have different bidder credentials
- Can have custom IDR settings

## Directory Structure

```
config/
└── publishers/
    ├── example.yaml        # Template (not loaded)
    ├── publisher-abc.yaml  # Publisher ABC config
    ├── publisher-xyz.yaml  # Publisher XYZ config
    └── ...
```

## Creating a Publisher

### Step 1: Create Config File

Copy the example and customize:

```bash
cp config/publishers/example.yaml config/publishers/my-publisher.yaml
```

### Step 2: Configure Publisher Settings

```yaml
# config/publishers/my-publisher.yaml

publisher_id: "my-publisher"
name: "My Publisher Inc"
enabled: true

contact:
  email: "adops@mypublisher.com"
  name: "Ad Ops Team"

# Sites belonging to this publisher
sites:
  - site_id: "main-site"
    domain: "www.mypublisher.com"
    name: "Main Website"

  - site_id: "mobile-site"
    domain: "m.mypublisher.com"
    name: "Mobile Site"

# Bidder credentials (unique to this publisher)
bidders:
  appnexus:
    enabled: true
    params:
      placement_id: 98765432

  rubicon:
    enabled: true
    params:
      account_id: 5001
      site_id: 6002
      zone_id: 7003

  pubmatic:
    enabled: true
    params:
      publisher_id: "pub-my-publisher"

# IDR optimization settings
idr:
  max_bidders: 10      # Call up to 10 bidders per auction
  min_score: 0.05      # Include bidders scoring above 5%
  timeout_ms: 50       # 50ms for partner selection

# Rate limiting
rate_limits:
  requests_per_second: 2000
  burst: 200

# Privacy
privacy:
  gdpr_applies: true
  ccpa_applies: true
  coppa_applies: false
```

### Step 3: Restart Services

```bash
docker compose -f docker-compose.prod.yml restart idr
```

Or reload configs without restart:

```bash
curl -X POST http://localhost:5050/admin/reload-configs
```

## Publisher Integration

Publishers use their `publisher_id` as the `accountId` in Prebid.js:

```javascript
pbjs.setConfig({
  s2sConfig: {
    accountId: 'my-publisher',  // Matches config file name
    // ...
  }
});
```

## Managing Multiple Publishers

### List All Publishers

```bash
curl http://localhost:5050/admin/publishers
```

Response:
```json
{
  "publishers": [
    {"id": "publisher-abc", "name": "Publisher ABC", "enabled": true},
    {"id": "publisher-xyz", "name": "Publisher XYZ", "enabled": true}
  ]
}
```

### Get Publisher Config

```bash
curl http://localhost:5050/admin/publishers/my-publisher
```

### Disable a Publisher

Set `enabled: false` in their config file, then reload:

```bash
curl -X POST http://localhost:5050/admin/reload-configs
```

### View Publisher Metrics

```bash
curl http://localhost:5050/admin/publishers/my-publisher/metrics
```

## Per-Publisher Bidder Setup

Each publisher can have different credentials for the same bidder:

**Publisher A** (`config/publishers/publisher-a.yaml`):
```yaml
bidders:
  appnexus:
    enabled: true
    params:
      placement_id: 11111111
```

**Publisher B** (`config/publishers/publisher-b.yaml`):
```yaml
bidders:
  appnexus:
    enabled: true
    params:
      placement_id: 22222222
```

## IDR Optimization Per Publisher

Customize the IDR behavior for each publisher:

```yaml
idr:
  # Premium publisher - call more bidders
  max_bidders: 12
  min_score: 0.02
  timeout_ms: 75
```

```yaml
idr:
  # High-traffic publisher - optimize for speed
  max_bidders: 6
  min_score: 0.15
  timeout_ms: 30
```

## Rate Limiting

Protect your infrastructure with per-publisher limits:

```yaml
rate_limits:
  requests_per_second: 5000  # Large publisher
  burst: 500
```

```yaml
rate_limits:
  requests_per_second: 500   # Smaller publisher
  burst: 50
```

## Privacy Settings

Configure privacy compliance per publisher:

```yaml
privacy:
  gdpr_applies: true    # EU publisher
  ccpa_applies: false
  coppa_applies: false
```

```yaml
privacy:
  gdpr_applies: false
  ccpa_applies: true    # US publisher
  coppa_applies: true   # Kids content
```

## Environment Variables

Override config directory location:

```bash
PUBLISHER_CONFIG_DIR=/etc/nexus-engine/publishers
```

## Monitoring

### Metrics by Publisher

All metrics include a `publisher` label:

```
idr_requests_total{publisher="publisher-abc"} 125000
idr_requests_total{publisher="publisher-xyz"} 89000
```

### Logs by Publisher

Logs include publisher context:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "publisher_id": "publisher-abc",
  "site_id": "main-site",
  "message": "Auction completed",
  "bidders_called": 8,
  "bids_received": 5
}
```

## Scaling Considerations

### Single Instance (Recommended for < 50k QPS)

One server handles all publishers. Simple to manage.

### Multiple Instances (> 50k QPS)

Run multiple instances behind a load balancer:

```
                    ┌─────────────┐
                    │   Load      │
                    │  Balancer   │
                    └──────┬──────┘
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │  Nexus   │ │  Nexus   │ │  Nexus   │
        │ Engine 1 │ │ Engine 2 │ │ Engine 3 │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │            │            │
             └────────────┼────────────┘
                          ▼
                    ┌──────────┐
                    │  Shared  │
                    │  Redis   │
                    └──────────┘
```

All instances share the same Redis for consistent metrics and IDR learning.

## Troubleshooting

### Publisher Not Found

Check that:
1. Config file exists: `ls config/publishers/`
2. File name matches publisher_id
3. `enabled: true` is set
4. YAML syntax is valid

### Bidder Not Working for Publisher

1. Check bidder is enabled in config
2. Verify bidder params are correct
3. Check logs for errors:
   ```bash
   docker compose logs idr | grep "publisher-id"
   ```

### Config Changes Not Applied

Reload configs:
```bash
curl -X POST http://localhost:5050/admin/reload-configs
```

Or restart the IDR service:
```bash
docker compose -f docker-compose.prod.yml restart idr
```
