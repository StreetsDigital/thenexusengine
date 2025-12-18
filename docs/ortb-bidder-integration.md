# OpenRTB Bidder Integration Guide

The Nexus Engine supports dynamic OpenRTB bidder integration, allowing you to add custom demand sources without writing code. This guide covers creating, configuring, and managing custom bidders through the Admin UI or API.

## Overview

The OpenRTB bidder integration system enables:

- **Dynamic Bidder Creation** - Add new demand partners without code changes
- **OpenRTB 2.5/2.6 Support** - Full protocol compliance for standard integrations
- **Request/Response Transformation** - Map fields for bidder-specific formats
- **Publisher & Geo Targeting** - Control which bidders serve which inventory
- **Real-time Configuration** - Update bidder settings without restarts
- **Authentication Options** - Bearer tokens, basic auth, or custom headers

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Admin UI / API                           │
│                    (Create/Update Bidders)                       │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Redis Storage                               │
│           nexus:bidders (hash) + nexus:bidders:active (set)      │
└─────────────────────────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
┌─────────────────────────┐    ┌─────────────────────────┐
│   Python IDR Service    │    │   Go PBS Server         │
│   (Bidder Manager)      │    │   (Dynamic Registry)    │
│   - CRUD operations     │    │   - Runtime adapter     │
│   - Validation          │    │   - Refresh from Redis  │
│   - Test endpoint       │    │   - Auction integration │
└─────────────────────────┘    └─────────────────────────┘
                                        │
                                        ▼
                              ┌─────────────────────┐
                              │ OpenRTB Bid Request │
                              │ to Demand Partner   │
                              └─────────────────────┘
```

## Quick Start

### 1. Access the Admin UI

Navigate to the Admin dashboard:

```
http://localhost:5050/bidders
```

### 2. Create a New Bidder

Click "Add Bidder" and fill in the configuration:

**Basic Information:**
- **Bidder Code**: Unique identifier (e.g., `mydemand`)
- **Name**: Display name (e.g., "My Demand Partner")
- **Description**: Brief description of the integration

**Endpoint Configuration:**
- **URL**: The partner's OpenRTB endpoint
- **Method**: HTTP method (typically `POST`)
- **Timeout**: Request timeout in milliseconds
- **Protocol Version**: `2.5` or `2.6`

### 3. Test the Integration

Use the "Test Endpoint" button to verify connectivity before enabling.

### 4. Enable the Bidder

Set status to "Active" to include in auctions.

---

## Configuration Reference

### BidderConfig Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `bidder_code` | string | Yes | Unique identifier (lowercase, alphanumeric, underscores) |
| `name` | string | Yes | Human-readable name |
| `description` | string | No | Description of the bidder |
| `endpoint` | object | Yes | Endpoint configuration |
| `capabilities` | object | Yes | Supported features |
| `rate_limits` | object | No | Rate limiting settings |
| `request_transform` | object | No | Request modification rules |
| `response_transform` | object | No | Response modification rules |
| `status` | string | Yes | `active`, `testing`, `paused`, `disabled` |
| `gvl_vendor_id` | int | No | GDPR Global Vendor List ID |
| `priority` | int | No | Selection priority (higher = preferred) |
| `maintainer_email` | string | No | Contact email |
| `allowed_publishers` | array | No | Publisher whitelist (empty = all) |
| `blocked_publishers` | array | No | Publisher blacklist |
| `allowed_countries` | array | No | Country whitelist (empty = all) |
| `blocked_countries` | array | No | Country blacklist |

### Endpoint Configuration

```json
{
  "endpoint": {
    "url": "https://partner.example.com/openrtb/bid",
    "method": "POST",
    "timeout_ms": 200,
    "protocol_version": "2.5",
    "auth_type": "bearer",
    "auth_token": "your-api-token",
    "custom_headers": {
      "X-Partner-ID": "12345"
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `url` | string | Partner's bid endpoint URL |
| `method` | string | HTTP method (`POST`, `GET`) |
| `timeout_ms` | int | Request timeout in milliseconds |
| `protocol_version` | string | OpenRTB version (`2.5`, `2.6`) |
| `auth_type` | string | Authentication type (see below) |
| `auth_username` | string | Username for basic auth |
| `auth_password` | string | Password for basic auth |
| `auth_token` | string | Token for bearer auth |
| `auth_header_name` | string | Custom header name |
| `auth_header_value` | string | Custom header value |
| `custom_headers` | object | Additional headers to include |

**Authentication Types:**

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `none` | No authentication | None |
| `bearer` | Bearer token in Authorization header | `auth_token` |
| `basic` | HTTP Basic authentication | `auth_username`, `auth_password` |
| `header` | Custom header authentication | `auth_header_name`, `auth_header_value` |

### Capabilities Configuration

```json
{
  "capabilities": {
    "media_types": ["banner", "video", "native"],
    "currencies": ["USD", "EUR"],
    "site_enabled": true,
    "app_enabled": true,
    "video_protocols": [2, 3, 5, 6],
    "video_mimes": ["video/mp4", "video/webm"],
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true,
    "supports_gpp": false,
    "supports_schain": true,
    "supports_eids": true,
    "supports_first_party_data": true,
    "supports_ctv": false,
    "supports_ad_pods": false
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `media_types` | array | Supported ad formats: `banner`, `video`, `native`, `audio` |
| `currencies` | array | Accepted currencies (ISO 4217) |
| `site_enabled` | bool | Can bid on website inventory |
| `app_enabled` | bool | Can bid on app inventory |
| `video_protocols` | array | VAST protocol versions supported |
| `video_mimes` | array | Video MIME types supported |
| `supports_gdpr` | bool | Processes GDPR consent signals |
| `supports_ccpa` | bool | Processes CCPA privacy string |
| `supports_coppa` | bool | Respects COPPA flag |
| `supports_gpp` | bool | Supports Global Privacy Platform |
| `supports_schain` | bool | Processes supply chain info |
| `supports_eids` | bool | Accepts extended IDs |
| `supports_first_party_data` | bool | Processes first-party data |
| `supports_ctv` | bool | Supports Connected TV |
| `supports_ad_pods` | bool | Supports video ad pods |

### Rate Limits Configuration

```json
{
  "rate_limits": {
    "qps_limit": 1000,
    "daily_limit": 10000000,
    "concurrent_limit": 100
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `qps_limit` | int | Maximum queries per second |
| `daily_limit` | int | Maximum requests per day |
| `concurrent_limit` | int | Maximum concurrent requests |

### Request Transform Configuration

Transform the OpenRTB request before sending to the partner:

```json
{
  "request_transform": {
    "field_mappings": {
      "site.publisher.id": "site.publisher.ext.partner_pub_id"
    },
    "field_additions": {
      "ext.partner": "mydemand"
    },
    "field_removals": ["user.ext.consent"],
    "imp_ext_template": {
      "bidder": {
        "placement_id": "default_placement"
      }
    },
    "request_ext_template": {
      "prebid": {
        "channel": {"name": "nexus"}
      }
    },
    "site_ext_template": {
      "partner_site_id": "12345"
    },
    "user_ext_template": {
      "partner_user_segment": "premium"
    },
    "seat_id": "partner_seat_123"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `field_mappings` | object | Map source fields to destination fields |
| `field_additions` | object | Add static fields to the request |
| `field_removals` | array | Remove fields from the request |
| `imp_ext_template` | object | Template merged into each impression's ext |
| `request_ext_template` | object | Template merged into request ext |
| `site_ext_template` | object | Template merged into site ext |
| `user_ext_template` | object | Template merged into user ext |
| `seat_id` | string | Seat ID to include in request |
| `schain_augment` | object | Supply chain augmentation configuration |

### Supply Chain (SChain) Augmentation

The supply chain augmentation feature allows you to append supply chain nodes to bid requests on a per-bidder basis. This enables proper transparency documentation for programmatic transactions per [IAB OpenRTB Supply Chain spec](https://iabtechlab.com/standards/openrtb-supply-chain/).

**Use cases:**
- Document The Nexus Engine as an intermediary in the supply chain
- Add bidder-specific seller IDs for different partnerships
- Ensure ads.txt/sellers.json compliance
- Provide transparency for header bidding integrations

```json
{
  "request_transform": {
    "schain_augment": {
      "enabled": true,
      "nodes": [
        {
          "asi": "nexusengine.com",
          "sid": "nexus-publisher-123",
          "hp": 1,
          "name": "The Nexus Engine",
          "domain": "nexusengine.com"
        }
      ],
      "complete": 1,
      "version": "1.0"
    }
  }
}
```

**SChain Augment Configuration:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | bool | Yes | Enable/disable schain augmentation for this bidder |
| `nodes` | array | Yes | List of supply chain nodes to append |
| `complete` | int | No | Override the complete flag (1=complete, 0=incomplete, null=preserve original) |
| `version` | string | No | SChain version (default "1.0") |

**SChain Node Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `asi` | string | Yes | Canonical domain name of the SSP/Exchange (e.g., "nexusengine.com") |
| `sid` | string | Yes | Seller ID representing the bidder's relationship with this entity |
| `hp` | int | No | Header bidding partner flag: 1=direct/payment, 0=indirect (default 1) |
| `rid` | string | No | Request ID for tracking (optional, typically omitted) |
| `name` | string | No | Human-readable name of the entity |
| `domain` | string | No | Domain of the entity (may differ from ASI) |
| `ext` | object | No | Custom extension data |

**How It Works:**

1. When a bid request arrives, the existing supply chain (if any) is preserved
2. If `schain_augment.enabled` is true, configured nodes are appended to the existing chain
3. If no supply chain exists, one is created with the configured nodes
4. The modified request is sent to the bidder

```
Original Request:                    Augmented Request to Bidder:
┌────────────────────────┐          ┌────────────────────────────────────┐
│ source.schain.nodes:   │   ──►    │ source.schain.nodes:               │
│ [{asi: "publisher.com" │          │ [{asi: "publisher.com"             │
│   sid: "pub-123"}]     │          │   sid: "pub-123"},                 │
└────────────────────────┘          │  {asi: "nexusengine.com"           │
                                    │   sid: "nexus-seat-001",           │
                                    │   hp: 1,                           │
                                    │   name: "The Nexus Engine"}]       │
                                    └────────────────────────────────────┘
```

**Admin UI Configuration:**

You can configure schain augmentation through the Admin UI:

1. Open the bidder edit modal
2. Expand the "Supply Chain (SChain) Augmentation" section
3. Check "Enable SChain Augmentation"
4. Add one or more nodes with the required fields
5. Optionally set the chain completeness flag
6. Save the bidder configuration

### Response Transform Configuration

Transform bid responses from the partner:

```json
{
  "response_transform": {
    "bid_field_mappings": {
      "ext.deal_id": "dealid"
    },
    "price_adjustment": 0.95,
    "currency_conversion": true,
    "creative_type_mappings": {
      "display": "banner",
      "vast": "video"
    },
    "extract_duration_from_vast": true
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `bid_field_mappings` | object | Map partner response fields to OpenRTB |
| `price_adjustment` | float | Multiply bid prices (e.g., 0.95 for 5% reduction) |
| `currency_conversion` | bool | Enable automatic currency conversion |
| `creative_type_mappings` | object | Map partner creative types to standard types |
| `extract_duration_from_vast` | bool | Parse video duration from VAST |

---

## API Reference

### List All Bidders

```bash
GET /api/bidders
Authorization: Basic <credentials>
```

**Response:**
```json
{
  "bidders": [
    {
      "bidder_code": "mydemand",
      "name": "My Demand Partner",
      "status": "active",
      ...
    }
  ],
  "count": 1
}
```

### Get Single Bidder

```bash
GET /api/bidders/<bidder_code>
Authorization: Basic <credentials>
```

### Create Bidder

```bash
POST /api/bidders
Authorization: Basic <credentials>
Content-Type: application/json

{
  "bidder_code": "newpartner",
  "name": "New Partner",
  "description": "A new demand partner integration",
  "endpoint": {
    "url": "https://partner.com/bid",
    "method": "POST",
    "timeout_ms": 200,
    "protocol_version": "2.5",
    "auth_type": "bearer",
    "auth_token": "secret-token"
  },
  "capabilities": {
    "media_types": ["banner", "video"],
    "currencies": ["USD"],
    "site_enabled": true,
    "app_enabled": false,
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true
  },
  "status": "testing"
}
```

### Update Bidder

```bash
PUT /api/bidders/<bidder_code>
Authorization: Basic <credentials>
Content-Type: application/json

{
  "name": "Updated Partner Name",
  "status": "active"
}
```

### Delete Bidder

```bash
DELETE /api/bidders/<bidder_code>
Authorization: Basic <credentials>
```

### Test Bidder Endpoint

```bash
POST /api/bidders/<bidder_code>/test
Authorization: Basic <credentials>
```

**Response:**
```json
{
  "success": true,
  "status_code": 200,
  "latency_ms": 45.2,
  "message": "Endpoint reachable"
}
```

### Enable/Disable Bidder

```bash
POST /api/bidders/<bidder_code>/enable
POST /api/bidders/<bidder_code>/disable
Authorization: Basic <credentials>
```

### Get Active Bidders (No Auth)

For PBS server to fetch active bidders:

```bash
GET /api/bidders/active
```

---

## Example Configurations

### Simple Banner Partner

```json
{
  "bidder_code": "simplebanner",
  "name": "Simple Banner DSP",
  "endpoint": {
    "url": "https://dsp.example.com/rtb/bid",
    "method": "POST",
    "timeout_ms": 150,
    "protocol_version": "2.5",
    "auth_type": "bearer",
    "auth_token": "api-key-12345"
  },
  "capabilities": {
    "media_types": ["banner"],
    "currencies": ["USD"],
    "site_enabled": true,
    "app_enabled": false,
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true
  },
  "status": "active",
  "priority": 50
}
```

### Video Specialist with VAST

```json
{
  "bidder_code": "videopartner",
  "name": "Video Specialist DSP",
  "endpoint": {
    "url": "https://video-dsp.example.com/openrtb/v2/bid",
    "method": "POST",
    "timeout_ms": 300,
    "protocol_version": "2.6",
    "auth_type": "header",
    "auth_header_name": "X-API-Key",
    "auth_header_value": "video-api-key"
  },
  "capabilities": {
    "media_types": ["video"],
    "currencies": ["USD", "EUR"],
    "site_enabled": true,
    "app_enabled": true,
    "video_protocols": [2, 3, 5, 6, 7, 8],
    "video_mimes": ["video/mp4", "video/webm", "video/x-flv"],
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true,
    "supports_ctv": true,
    "supports_ad_pods": true
  },
  "response_transform": {
    "extract_duration_from_vast": true
  },
  "status": "active",
  "gvl_vendor_id": 999,
  "priority": 75
}
```

### Regional Partner with Publisher Restrictions

```json
{
  "bidder_code": "eupartner",
  "name": "EU Regional Partner",
  "endpoint": {
    "url": "https://eu.partner.com/bid",
    "method": "POST",
    "timeout_ms": 200,
    "protocol_version": "2.5",
    "auth_type": "basic",
    "auth_username": "nexus",
    "auth_password": "secret123"
  },
  "capabilities": {
    "media_types": ["banner", "native"],
    "currencies": ["EUR", "GBP"],
    "site_enabled": true,
    "app_enabled": true,
    "supports_gdpr": true,
    "supports_ccpa": false,
    "supports_coppa": true,
    "supports_gpp": true
  },
  "allowed_countries": ["DE", "FR", "IT", "ES", "NL", "BE", "AT", "CH"],
  "blocked_publishers": ["competitor_pub_1", "low_quality_pub"],
  "status": "active",
  "gvl_vendor_id": 888,
  "priority": 60,
  "maintainer_email": "partners@nexusengine.com"
}
```

### Partner with Request Transformation

```json
{
  "bidder_code": "custompartner",
  "name": "Custom Format Partner",
  "endpoint": {
    "url": "https://custom.partner.com/v3/rtb",
    "method": "POST",
    "timeout_ms": 250,
    "protocol_version": "2.5",
    "auth_type": "bearer",
    "auth_token": "custom-token",
    "custom_headers": {
      "X-Partner-Version": "3.0",
      "X-Request-Source": "nexus"
    }
  },
  "capabilities": {
    "media_types": ["banner", "video", "native"],
    "currencies": ["USD"],
    "site_enabled": true,
    "app_enabled": true,
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true,
    "supports_schain": true,
    "supports_eids": true,
    "supports_first_party_data": true
  },
  "request_transform": {
    "imp_ext_template": {
      "bidder": {
        "network_id": "nexus_network",
        "traffic_source": "direct"
      }
    },
    "request_ext_template": {
      "partner": {
        "version": "1.0",
        "source": "thenexusengine"
      }
    },
    "seat_id": "nexus_seat_001"
  },
  "response_transform": {
    "price_adjustment": 0.98
  },
  "rate_limits": {
    "qps_limit": 500,
    "daily_limit": 5000000,
    "concurrent_limit": 50
  },
  "status": "active",
  "priority": 80
}
```

### Partner with Supply Chain Augmentation

```json
{
  "bidder_code": "transparentpartner",
  "name": "Transparent Partner with SChain",
  "description": "Partner requiring supply chain transparency",
  "endpoint": {
    "url": "https://transparent.partner.com/bid",
    "method": "POST",
    "timeout_ms": 200,
    "protocol_version": "2.6",
    "auth_type": "bearer",
    "auth_token": "partner-token-xyz"
  },
  "capabilities": {
    "media_types": ["banner", "video"],
    "currencies": ["USD"],
    "site_enabled": true,
    "app_enabled": true,
    "supports_gdpr": true,
    "supports_ccpa": true,
    "supports_coppa": true,
    "supports_schain": true
  },
  "request_transform": {
    "schain_augment": {
      "enabled": true,
      "nodes": [
        {
          "asi": "nexusengine.com",
          "sid": "nexus-transparent-seat",
          "hp": 1,
          "name": "The Nexus Engine",
          "domain": "nexusengine.com"
        }
      ],
      "complete": 1,
      "version": "1.0"
    }
  },
  "status": "active",
  "gvl_vendor_id": 777,
  "priority": 70
}
```

### Multi-hop Reseller Configuration

For complex supply chain scenarios with multiple intermediaries:

```json
{
  "bidder_code": "resellerpartner",
  "name": "Reseller Partner",
  "endpoint": {
    "url": "https://reseller.partner.com/rtb/bid",
    "method": "POST",
    "timeout_ms": 250,
    "protocol_version": "2.5"
  },
  "capabilities": {
    "media_types": ["banner"],
    "currencies": ["USD"],
    "supports_schain": true
  },
  "request_transform": {
    "schain_augment": {
      "enabled": true,
      "nodes": [
        {
          "asi": "nexusengine.com",
          "sid": "nexus-main",
          "hp": 1,
          "name": "The Nexus Engine"
        },
        {
          "asi": "reseller.partner.com",
          "sid": "reseller-seat-001",
          "hp": 0,
          "name": "Reseller Partner",
          "rid": "resell-tracking-id"
        }
      ],
      "complete": 1
    }
  },
  "status": "active",
  "priority": 50
}
```

---

## Best Practices

### 1. Start in Testing Mode

Always create new bidders with `status: "testing"` first:

```json
{
  "status": "testing"
}
```

Testing mode allows the bidder to participate in auctions but flags responses for monitoring.

### 2. Set Appropriate Timeouts

Balance latency vs. bid opportunity:

| Partner Type | Recommended Timeout |
|--------------|---------------------|
| Tier 1 SSPs | 150-200ms |
| Regional partners | 200-250ms |
| Video specialists | 250-350ms |
| Secondary partners | 100-150ms |

### 3. Configure Rate Limits

Protect partners and your infrastructure:

```json
{
  "rate_limits": {
    "qps_limit": 1000,
    "concurrent_limit": 100
  }
}
```

### 4. Use Publisher/Geo Targeting

Avoid sending traffic to inappropriate partners:

```json
{
  "allowed_countries": ["US", "CA"],
  "blocked_publishers": ["adult_pub_1", "piracy_pub_2"]
}
```

### 5. Set GVL Vendor IDs

For GDPR compliance, always set the vendor ID:

```json
{
  "gvl_vendor_id": 123,
  "capabilities": {
    "supports_gdpr": true
  }
}
```

### 6. Monitor Performance

Use the IDR dashboard to track:
- Bid rates
- Win rates
- Average CPM
- Latency percentiles
- Error rates

### 7. Gradual Rollout

1. Create bidder in `testing` status
2. Verify with test endpoint
3. Enable for specific publishers first
4. Monitor for 24-48 hours
5. Expand to full traffic

---

## Troubleshooting

### Bidder Not Participating in Auctions

1. **Check status**: Must be `active` or `testing`
2. **Verify endpoint**: Use "Test Endpoint" button
3. **Check capabilities**: Ensure media types match inventory
4. **Review targeting**: Check publisher/country restrictions

### Low Bid Rates

1. **Check timeout**: May be too short for partner
2. **Review request transform**: Partner may need specific fields
3. **Verify capabilities**: Ensure media types are correctly configured
4. **Check authentication**: Invalid credentials cause rejections

### High Error Rates

1. **Test endpoint connectivity**: Use the test function
2. **Check response format**: Partner may use non-standard OpenRTB
3. **Review logs**: Check IDR and PBS logs for specific errors
4. **Contact partner**: May need to whitelist your IP/domain

### Invalid Bids

1. **Check price adjustment**: May be incorrectly configured
2. **Review response transform**: Field mappings may be wrong
3. **Verify currency**: Partner may return unexpected currency

---

## Redis Storage Schema

Bidder configurations are stored in Redis:

| Key | Type | Description |
|-----|------|-------------|
| `nexus:bidders` | Hash | Bidder code → JSON config |
| `nexus:bidders:active` | Set | Set of active bidder codes |
| `nexus:bidders:index` | Sorted Set | Bidder codes by priority |
| `nexus:bidders:{code}:stats` | Hash | Performance statistics |

---

## Integration with IDR

Dynamic bidders integrate with the Intelligent Demand Router:

1. **Scoring**: IDR scores dynamic bidders alongside static ones
2. **Selection**: Dynamic bidders compete based on predicted performance
3. **Learning**: IDR learns from dynamic bidder performance over time
4. **Metrics**: All standard IDR metrics apply to dynamic bidders

---

## Security Considerations

### Credential Storage

- Auth tokens are stored in Redis
- Use environment variables for sensitive values in production
- Rotate credentials regularly

### Network Security

- Use HTTPS endpoints only
- Whitelist partner IPs if possible
- Monitor for unusual traffic patterns

### Access Control

- Admin UI requires authentication
- API endpoints require Basic auth
- `/api/bidders/active` is unauthenticated (for PBS internal use)

---

## Related Documentation

- [Publisher Integration Guide](publisher-integration.md)
- [Docker Setup](docker-setup.md)
- [Multi-tenant Setup](multi-tenant-setup.md)
- [API Reference](api/openapi.yaml)
