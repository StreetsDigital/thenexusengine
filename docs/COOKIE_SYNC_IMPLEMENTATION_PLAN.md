# Cookie Sync Implementation Plan

## Overview

This document outlines the implementation plan for adding `/cookie_sync` and `/setuid` endpoints to the Nexus Engine PBS. These endpoints are required for Prebid.js integration via `s2sConfig`.

---

## Current State

**Existing Infrastructure:**
- 22 bidder adapters (appnexus, rubicon, pubmatic, ix, criteo, etc.)
- Handler pattern in `internal/endpoints/` with `http.Handler` interface
- Standard `http.ServeMux` router in `cmd/server/main.go`
- Privacy structs (`Regs`) with GDPR, CCPA, GPP fields in `internal/openrtb/request.go`
- `SyncerInfo` stub exists in `internal/adapters/adapter.go` but only has `Supports []string`

**Missing Components:**
- No user sync URL storage per bidder
- No cookie sync endpoint
- No UID cookie handling
- No GDPR consent enforcement for syncing

---

## Architecture

```
┌─────────────────┐     POST /cookie_sync      ┌──────────────────────┐
│   Prebid.js     │ ──────────────────────────▶│  CookieSyncHandler   │
│   (Browser)     │                            │                      │
└─────────────────┘                            └──────────┬───────────┘
                                                          │
                                                          ▼
                                               ┌──────────────────────┐
                                               │      Syncer          │
                                               │  - Build sync URLs   │
                                               │  - Check GDPR        │
                                               │  - Filter bidders    │
                                               └──────────┬───────────┘
                                                          │
                                                          ▼
                                               ┌──────────────────────┐
                                               │   Bidder Configs     │
                                               │  - Sync URL templates│
                                               │  - GVL Vendor IDs    │
                                               └──────────────────────┘

┌─────────────────┐     GET /setuid?bidder=x   ┌──────────────────────┐
│  Bidder Sync    │ ──────────────────────────▶│   SetUIDHandler      │
│   (Redirect)    │                            │  - Update UID cookie │
└─────────────────┘                            └──────────────────────┘
```

---

## Files to Create

| File | Purpose |
|------|---------|
| `pbs/internal/usersync/types.go` | Request/response data structures |
| `pbs/internal/usersync/cookie.go` | UID cookie management |
| `pbs/internal/usersync/syncer.go` | Core syncer logic |
| `pbs/internal/usersync/gdpr.go` | GDPR/privacy filtering |
| `pbs/internal/usersync/config.go` | Bidder sync URL configurations |
| `pbs/internal/endpoints/cookiesync.go` | `/cookie_sync` HTTP handler |
| `pbs/internal/endpoints/setuid.go` | `/setuid` HTTP handler |

## Files to Modify

| File | Changes |
|------|---------|
| `pbs/internal/adapters/adapter.go` | Expand `SyncerInfo` struct |
| `pbs/cmd/server/main.go` | Add route registrations |

---

## Data Structures

### CookieSyncRequest

```go
type CookieSyncRequest struct {
    Bidders        []string        `json:"bidders,omitempty"`
    Limit          int             `json:"limit,omitempty"`
    GDPR           *int            `json:"gdpr,omitempty"`
    GDPRConsent    string          `json:"gdpr_consent,omitempty"`
    USPrivacy      string          `json:"us_privacy,omitempty"`
    GPP            string          `json:"gpp,omitempty"`
    GPPSID         []int           `json:"gpp_sid,omitempty"`
    CooperativeSync bool           `json:"coopSync,omitempty"`
    FilterSettings *FilterSettings `json:"filterSettings,omitempty"`
    Account        string          `json:"account,omitempty"`
    Debug          bool            `json:"debug,omitempty"`
}
```

### CookieSyncResponse

```go
type CookieSyncResponse struct {
    Status       string       `json:"status"`
    BidderStatus []BidderSync `json:"bidder_status"`
    Debug        *DebugInfo   `json:"debug,omitempty"`
}

type BidderSync struct {
    Bidder   string   `json:"bidder"`
    NoCookie bool     `json:"no_cookie,omitempty"`
    UserSync *SyncURL `json:"usersync,omitempty"`
    Error    string   `json:"error,omitempty"`
}

type SyncURL struct {
    URL         string `json:"url"`
    Type        string `json:"type"` // "iframe" or "redirect"
    SupportCORS bool   `json:"supportCORS,omitempty"`
}
```

### SyncerInfo (expanded)

```go
type SyncerInfo struct {
    Key         string           // Unique syncer key (may differ from bidder code)
    Supports    []SyncType       // iframe, redirect, or both
    Default     SyncType         // Default sync type
    IFrameURL   *SyncURLTemplate // iframe sync URL template
    RedirectURL *SyncURLTemplate // redirect/pixel sync URL template
    SupportCORS bool             // Whether the syncer endpoint supports CORS
}

type SyncType string

const (
    SyncTypeIFrame   SyncType = "iframe"
    SyncTypeRedirect SyncType = "redirect"
)

type SyncURLTemplate struct {
    URL       string // URL with macros: {{gdpr}}, {{gdpr_consent}}, {{us_privacy}}, {{redirect_url}}
    UserMacro string // Macro for user ID (default varies by bidder)
}
```

### PBSCookie (UID storage)

```go
type PBSCookie struct {
    UIDs     map[string]UIDEntry `json:"uids"`
    OptOut   bool                `json:"optout,omitempty"`
    Birthday time.Time           `json:"bday,omitempty"`
}

type UIDEntry struct {
    UID     string    `json:"uid"`
    Expires time.Time `json:"expires,omitempty"`
}
```

---

## Implementation Phases

### Phase 1: Foundation

1. **Create usersync package types** (`types.go`)
   - Define all request/response structures
   - Define `SyncType` constants
   - Define filter settings

2. **Expand SyncerInfo** (`adapter.go`)
   - Add URL template fields
   - Add `Key`, `Default`, `SupportCORS` fields

3. **Create cookie management** (`cookie.go`)
   - Implement `PBSCookie` struct
   - Add cookie read/write helpers
   - Handle cookie encoding/decoding

### Phase 2: Core Logic

4. **Create syncer core** (`syncer.go`)
   - Implement `Syncer` struct
   - Add `NewSyncer(registry)` constructor
   - Implement `BuildSyncURL(bidder, syncType, privacy)`
   - Implement `ChooseSyncs(request, existingUIDs, limit)`
   - Implement URL macro replacement

5. **Create GDPR enforcement** (`gdpr.go`)
   - Implement `GDPREnforcer`
   - Check vendor consent using GVL IDs
   - Implement `CanSync(bidder, gdprConsent)` method

### Phase 3: Bidder Configuration

6. **Configure sync URLs** (`config.go`)
   - Define sync URLs for all 22 bidders
   - Reference Prebid.org documentation for URLs

### Phase 4: Endpoints

7. **Create cookie_sync handler** (`cookiesync.go`)
   - Parse POST request body
   - Read existing UID cookie
   - Check GDPR consent per bidder
   - Filter by already-synced bidders
   - Apply limit and filter settings
   - Build and return sync URLs

8. **Create setuid handler** (`setuid.go`)
   - Handle `/setuid?bidder=xxx&uid=yyy`
   - Update UID cookie
   - Return appropriate response/redirect

### Phase 5: Integration

9. **Register routes** (`main.go`)
   - Add `/cookie_sync` route
   - Add `/setuid` route
   - Wire up handlers with dependencies

---

## Bidder Sync URL Reference

| Bidder | Sync Key | Types | GVL ID |
|--------|----------|-------|--------|
| appnexus | adnxs | iframe, redirect | 32 |
| rubicon | rubicon | iframe, redirect | 52 |
| pubmatic | pubmatic | iframe, redirect | 76 |
| ix | ix | redirect | 10 |
| criteo | criteo | redirect | 91 |
| openx | openx | iframe, redirect | 69 |
| triplelift | triplelift | redirect | 28 |
| sharethrough | sharethrough | iframe | 80 |
| sovrn | sovrn | redirect | 13 |
| 33across | 33across | iframe | 58 |
| adform | adform | redirect | 50 |
| beachfront | beachfront | iframe, redirect | - |
| conversant | conversant | redirect | 24 |
| gumgum | gumgum | redirect | 61 |
| improvedigital | improvedigital | redirect | 253 |
| medianet | medianet | redirect | 142 |
| outbrain | outbrain | redirect | 164 |
| smartadserver | smartadserver | redirect | 45 |
| spotx | spotx | redirect | 165 |
| taboola | taboola | redirect | 42 |
| teads | teads | redirect | 132 |
| unruly | unruly | redirect | 36 |

---

## Example Request/Response

### Request

```http
POST /cookie_sync HTTP/1.1
Content-Type: application/json

{
    "bidders": ["appnexus", "rubicon", "pubmatic"],
    "gdpr": 1,
    "gdpr_consent": "BONciguONcjGKADACHENAOLS1rAHDAFAAEAASABQAMwAeACEAFw",
    "us_privacy": "1YNN",
    "limit": 5
}
```

### Response

```json
{
    "status": "ok",
    "bidder_status": [
        {
            "bidder": "appnexus",
            "usersync": {
                "url": "https://ib.adnxs.com/getuid?https://pbs.example.com/setuid?bidder=appnexus&uid=$UID",
                "type": "iframe",
                "supportCORS": false
            }
        },
        {
            "bidder": "rubicon",
            "usersync": {
                "url": "https://pixel.rubiconproject.com/exchange/sync.php?p=prebid&gdpr=1&gdpr_consent=BONcig...",
                "type": "iframe",
                "supportCORS": false
            }
        },
        {
            "bidder": "pubmatic",
            "no_cookie": true
        }
    ]
}
```

---

## SetUID Endpoint

### Request

```http
GET /setuid?bidder=appnexus&uid=ABC123&gdpr=1&gdpr_consent=BONcig...
```

### Response

Returns a 1x1 pixel or empty response after setting the cookie.

---

## URL Macros

The following macros are supported in sync URL templates:

| Macro | Description |
|-------|-------------|
| `{{gdpr}}` | GDPR applies flag (0 or 1) |
| `{{gdpr_consent}}` | TCF consent string |
| `{{us_privacy}}` | CCPA/US Privacy string |
| `{{gpp}}` | GPP consent string |
| `{{gpp_sid}}` | GPP section IDs |
| `{{redirect_url}}` | URL-encoded redirect back to `/setuid` |
| `$UID` / `${UID}` / `[UID]` | Bidder's user ID macro (replaced by bidder) |

---

## Testing Strategy

1. **Unit Tests**
   - URL macro replacement
   - GDPR filtering logic
   - Cookie parsing/writing
   - Bidder selection/filtering

2. **Integration Tests**
   - Full request/response flow
   - Cookie persistence
   - CORS handling

3. **E2E Tests**
   - Test with actual Prebid.js s2sConfig
   - Verify sync pixel loading
   - Verify UID cookie updates

---

## Configuration

```go
type CookieSyncConfig struct {
    DefaultLimit     int           // Default max syncs (default: 8)
    MaxLimit         int           // Max allowed limit (default: 32)
    CookieName       string        // UID cookie name (default: "uids")
    CookieDomain     string        // Cookie domain
    CookieMaxAge     time.Duration // Cookie expiry (default: 90 days)
    ExternalURL      string        // External URL for redirect_url macro
    CooperativeSync  bool          // Enable cooperative sync by default
    GDPREnforcement  bool          // Enforce GDPR for syncing
}
```

---

## Next Steps

1. Review and approve this plan
2. Begin implementation starting with Phase 1
3. Create PR for each phase for incremental review
