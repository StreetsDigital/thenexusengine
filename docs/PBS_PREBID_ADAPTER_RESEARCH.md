# PBS to Prebid.js Integration Research

## Executive Summary

This document analyzes approaches to integrate the Nexus Engine PBS (Prebid Server) instance directly into Prebid.js as a bidder adapter. After thorough research, there are **three viable approaches**, each with distinct trade-offs.

---

## Your PBS Implementation Overview

The Nexus Engine PBS is a sophisticated Go-based Prebid Server with:

- **OpenRTB 2.5/2.6 compliance** - Full spec implementation
- **22+ bidder adapters** - AppNexus, Rubicon, PubMatic, Criteo, etc.
- **IDR (Intelligent Demand Router)** - ML-powered bidder selection
- **Main endpoint**: `POST /openrtb2/auction`
- **First Party Data support** - Global and bidder-specific FPD processing
- **Privacy compliance** - GDPR, CCPA, COPPA, GPP support

---

## Integration Approaches

### Approach 1: Use Standard `prebidServerBidAdapter` (Recommended)

**Description**: Use Prebid.js's built-in `prebidServerBidAdapter` module with `s2sConfig` to connect to your PBS instance.

**Implementation**:

```javascript
pbjs.setConfig({
  s2sConfig: [{
    name: 'nexusEngine',
    accountId: 'YOUR_ACCOUNT_ID',
    bidders: ['appnexus', 'pubmatic', 'rubicon'], // bidders configured in your PBS
    adapter: 'prebidServer',
    enabled: true,
    endpoint: {
      p1Consent: 'https://your-pbs.example.com/openrtb2/auction',
      noP1Consent: 'https://your-pbs.example.com/openrtb2/auction'
    },
    syncEndpoint: {
      p1Consent: 'https://your-pbs.example.com/cookie_sync',
      noP1Consent: 'https://your-pbs.example.com/cookie_sync'
    },
    timeout: 500,
    allowUnknownBidderCodes: true, // Allow PBS to return bids from any bidder
    extPrebid: {
      cache: {
        vastxml: { returnCreative: false }
      },
      targeting: {
        pricegranularity: {
          ranges: [
            { max: 5.00, increment: 0.05 },
            { max: 10.00, increment: 0.10 },
            { max: 20.00, increment: 0.50 },
            { max: 50.00, increment: 1.00 }
          ]
        }
      }
    }
  }]
});
```

**Ad Unit Configuration**:

```javascript
var adUnits = [{
  code: 'div-banner-1',
  mediaTypes: {
    banner: {
      sizes: [[300, 250], [728, 90]]
    }
  },
  bids: [
    {
      bidder: 'appnexus',
      params: { placementId: '12345' }
    },
    {
      bidder: 'pubmatic',
      params: { publisherId: '98765', adSlot: 'slot1' }
    }
  ]
}];
```

**Pros**:
- ✅ Standard Prebid.js approach - well-documented and supported
- ✅ Automatic handling of GDPR/CCPA consent
- ✅ Cookie sync support built-in
- ✅ No custom adapter code needed
- ✅ Works with existing PBS endpoint (`/openrtb2/auction`)
- ✅ Supports hybrid client-side + server-side bidding
- ✅ Your IDR intelligence is used server-side (transparent to Prebid.js)

**Cons**:
- ❌ Requires bidders to be explicitly listed in `s2sConfig.bidders`
- ❌ Less control over request/response transformation on client-side
- ❌ Must implement `/cookie_sync` endpoint if user sync is needed

**Required PBS Endpoints**:
- `/openrtb2/auction` ✅ Already implemented
- `/cookie_sync` ⚠️ May need implementation
- `/status` ✅ Already implemented

---

### Approach 2: Create a Custom "Nexus" Bidder Adapter

**Description**: Create a custom Prebid.js bidder adapter that presents your PBS as a single bidder called "nexus".

**Implementation** (`nexusBidAdapter.js`):

```javascript
import { registerBidder } from '../src/adapters/bidderFactory.js';
import { ortbConverter } from '../libraries/ortbConverter/converter.js';
import { ajax } from '../src/ajax.js';
import { deepSetValue, deepAccess } from '../src/utils.js';

const BIDDER_CODE = 'nexus';
const ENDPOINT = 'https://your-pbs.example.com/openrtb2/auction';

const converter = ortbConverter({
  context: {
    netRevenue: true,
    ttl: 300
  },
  imp(buildImp, bidRequest, context) {
    const imp = buildImp(bidRequest, context);
    // Add custom params to imp.ext
    deepSetValue(imp, 'ext.nexus', {
      publisherId: bidRequest.params.publisherId,
      siteId: bidRequest.params.siteId,
      zoneId: bidRequest.params.zoneId
    });
    return imp;
  },
  request(buildRequest, imps, bidderRequest, context) {
    const request = buildRequest(imps, bidderRequest, context);
    // Add Nexus-specific extensions
    deepSetValue(request, 'ext.prebid.targeting.pricegranularity', {
      precision: 2,
      ranges: [{ max: 50, increment: 0.01 }]
    });
    return request;
  },
  bidResponse(buildBidResponse, bid, context) {
    const bidResponse = buildBidResponse(bid, context);
    // Handle any special response processing
    bidResponse.meta = {
      ...bidResponse.meta,
      demandSource: 'nexus-pbs'
    };
    return bidResponse;
  }
});

export const spec = {
  code: BIDDER_CODE,
  supportedMediaTypes: ['banner', 'video', 'native'],

  isBidRequestValid(bid) {
    return !!(bid.params && bid.params.publisherId);
  },

  buildRequests(bidRequests, bidderRequest) {
    const data = converter.toORTB({bidRequests, bidderRequest});

    return [{
      method: 'POST',
      url: ENDPOINT,
      data: data,
      options: {
        contentType: 'application/json',
        withCredentials: true
      }
    }];
  },

  interpretResponse(serverResponse, request) {
    if (!serverResponse.body || !serverResponse.body.seatbid) {
      return [];
    }

    return converter.fromORTB({
      response: serverResponse.body,
      request: request.data
    }).bids;
  },

  getUserSyncs(syncOptions, serverResponses, gdprConsent, uspConsent) {
    const syncs = [];
    if (syncOptions.iframeEnabled) {
      syncs.push({
        type: 'iframe',
        url: `https://your-pbs.example.com/cookie_sync?gdpr=${gdprConsent?.gdprApplies ? 1 : 0}&gdpr_consent=${gdprConsent?.consentString || ''}&us_privacy=${uspConsent || ''}`
      });
    }
    return syncs;
  }
};

registerBidder(spec);
```

**Ad Unit Configuration**:

```javascript
var adUnits = [{
  code: 'div-banner-1',
  mediaTypes: {
    banner: { sizes: [[300, 250]] }
  },
  bids: [{
    bidder: 'nexus',
    params: {
      publisherId: 'pub123',
      siteId: 'site456',
      zoneId: 'zone789'
    }
  }]
}];
```

**Pros**:
- ✅ Full control over request/response transformation
- ✅ Simplified publisher integration (single bidder code)
- ✅ Can be submitted to Prebid.js repo or kept private
- ✅ Leverages ORTB converter library (less code)
- ✅ Your PBS handles all bidder orchestration

**Cons**:
- ❌ More code to maintain
- ❌ Need to handle edge cases manually
- ❌ Testing burden (80% code coverage required for PR)
- ❌ Must track Prebid.js API changes

---

### Approach 3: Runtime-Registered Private Adapter

**Description**: Use `pbjs.registerBidAdapter()` to inject a custom adapter at runtime without modifying Prebid.js build.

**Implementation**:

```javascript
// nexusAdapter.js - loaded separately from Prebid.js
(function() {
  window.pbjs = window.pbjs || { que: [] };

  window.pbjs.que.push(function() {

    const ENDPOINT = 'https://your-pbs.example.com/openrtb2/auction';

    function NexusAdapter() {
      return {
        callBids: function(bidderRequest, addBidResponse, done) {
          const openrtbRequest = buildOpenRTBRequest(bidderRequest);

          fetch(ENDPOINT, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(openrtbRequest),
            credentials: 'include'
          })
          .then(response => response.json())
          .then(ortbResponse => {
            processBidResponse(ortbResponse, bidderRequest, addBidResponse);
            done();
          })
          .catch(error => {
            console.error('Nexus bid error:', error);
            done();
          });
        }
      };
    }

    function buildOpenRTBRequest(bidderRequest) {
      const request = {
        id: bidderRequest.auctionId,
        imp: bidderRequest.bids.map((bid, index) => ({
          id: bid.bidId,
          banner: bid.mediaTypes?.banner ? {
            format: bid.mediaTypes.banner.sizes.map(s => ({ w: s[0], h: s[1] }))
          } : undefined,
          video: bid.mediaTypes?.video ? {
            ...bid.mediaTypes.video
          } : undefined,
          ext: {
            prebid: {
              bidder: bid.params
            }
          }
        })),
        site: {
          page: bidderRequest.refererInfo?.page,
          domain: bidderRequest.refererInfo?.domain
        },
        device: {
          ua: navigator.userAgent
        },
        tmax: bidderRequest.timeout
      };

      // Add consent if available
      if (bidderRequest.gdprConsent) {
        request.regs = {
          ext: { gdpr: bidderRequest.gdprConsent.gdprApplies ? 1 : 0 }
        };
        request.user = {
          ext: { consent: bidderRequest.gdprConsent.consentString }
        };
      }

      return request;
    }

    function processBidResponse(ortbResponse, bidderRequest, addBidResponse) {
      if (!ortbResponse.seatbid) return;

      ortbResponse.seatbid.forEach(seatbid => {
        seatbid.bid.forEach(bid => {
          const originalBid = bidderRequest.bids.find(b => b.bidId === bid.impid);
          if (!originalBid) return;

          const bidResponse = {
            requestId: bid.impid,
            cpm: bid.price,
            width: bid.w,
            height: bid.h,
            ad: bid.adm,
            ttl: 300,
            creativeId: bid.crid || bid.id,
            netRevenue: true,
            currency: ortbResponse.cur || 'USD',
            meta: {
              advertiserDomains: bid.adomain || []
            }
          };

          addBidResponse(originalBid.adUnitCode, bidResponse);
        });
      });
    }

    // Register the adapter
    window.pbjs.registerBidAdapter(NexusAdapter, 'nexus');

  });
})();
```

**Usage**:

```html
<!-- Load Prebid.js first -->
<script src="prebid.js"></script>
<!-- Then load your adapter -->
<script src="nexusAdapter.js"></script>
<script>
  var adUnits = [{
    code: 'div-banner-1',
    mediaTypes: { banner: { sizes: [[300, 250]] } },
    bids: [{
      bidder: 'nexus',
      params: { publisherId: '123' }
    }]
  }];
</script>
```

**Pros**:
- ✅ No Prebid.js build modification needed
- ✅ Complete control and privacy
- ✅ Easy to deploy and update independently
- ✅ Works with any Prebid.js version

**Cons**:
- ❌ Must handle all ORTB conversion manually
- ❌ Doesn't benefit from Prebid.js utilities/libraries
- ❌ More maintenance burden
- ❌ Potential for version compatibility issues

---

## Recommendation Matrix

| Criteria | Approach 1 (s2sConfig) | Approach 2 (Custom Adapter) | Approach 3 (Runtime) |
|----------|------------------------|----------------------------|---------------------|
| Implementation Effort | Low | Medium | Medium-High |
| Maintenance | Low | Medium | High |
| Flexibility | Medium | High | High |
| Standards Compliance | High | High | Medium |
| Publisher Simplicity | Medium | High | High |
| Privacy/Control | Medium | High | Highest |

---

## Detailed Recommendation

### For Most Use Cases: **Approach 1 (s2sConfig)**

Use the standard `prebidServerBidAdapter` with `s2sConfig`. This is the recommended approach because:

1. **Zero client-side adapter code** - Less to maintain
2. **Standard integration** - Publishers familiar with Prebid.js understand this pattern
3. **Your IDR runs server-side** - The intelligent bidder selection happens on your PBS, transparent to the client
4. **Privacy compliance** - Built-in handling for GDPR, CCPA, etc.
5. **Hybrid support** - Publishers can mix client-side and server-side bidders

**What you need to implement on PBS side**:
- ✅ `/openrtb2/auction` - Already done
- ⚠️ `/cookie_sync` endpoint - May need implementation
- ⚠️ Stored request support (optional) - For simplified ad unit config

### For "Nexus as a Single Bidder" Branding: **Approach 2**

If you want publishers to think of Nexus as a single demand source (hiding the complexity of your 22+ bidder integrations), create a custom adapter. This:

1. **Simplifies publisher config** - Single bidder code "nexus"
2. **Abstracts your bidder stack** - Publishers don't need to know about AppNexus, Rubicon, etc.
3. **Enables custom params** - Your own publisherId, siteId structure
4. **Can be open-sourced** - Submit to Prebid.js repo for credibility

### For Maximum Control: **Approach 3**

Use runtime registration only if:
- You can't modify Prebid.js builds
- You need complete privacy of your adapter logic
- You're deploying to publishers who already have Prebid.js

---

## Implementation Checklist

### For Approach 1 (s2sConfig)

- [ ] Verify `/openrtb2/auction` endpoint returns proper OpenRTB 2.5/2.6 responses
- [ ] Implement `/cookie_sync` endpoint (if user sync needed)
- [ ] Configure CORS headers for Prebid.js domains
- [ ] Set up proper response headers (`Content-Type: application/json`)
- [ ] Test with Prebid.js debug mode (`pbjs.setConfig({ debug: true })`)
- [ ] Document publisher integration guide
- [ ] Consider implementing stored requests for simplified config

### For Approach 2 (Custom Adapter)

- [ ] Create `modules/nexusBidAdapter.js`
- [ ] Create `modules/nexusBidAdapter.md` (documentation)
- [ ] Create `test/spec/modules/nexusBidAdapter_spec.js` (80% coverage required)
- [ ] Register with Prebid.org (if open-sourcing)
- [ ] Configure CI/CD for adapter updates

### For Approach 3 (Runtime)

- [ ] Create standalone adapter script
- [ ] Handle all media types (banner, video, native)
- [ ] Implement proper error handling
- [ ] Add user sync support
- [ ] Test across Prebid.js versions
- [ ] Document CDN deployment

---

## Cookie Sync Endpoint Implementation

If choosing Approach 1, you'll likely need a `/cookie_sync` endpoint:

```go
// Handler for /cookie_sync
func (h *Handler) CookieSync(w http.ResponseWriter, r *http.Request) {
    // Parse request
    var req CookieSyncRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Build sync URLs for requested bidders
    response := CookieSyncResponse{
        Status: "ok",
        BidderStatus: []BidderSyncStatus{},
    }

    for _, bidder := range req.Bidders {
        if syncURL, ok := h.getSyncURL(bidder); ok {
            response.BidderStatus = append(response.BidderStatus, BidderSyncStatus{
                Bidder:   bidder,
                NoCookie: true,
                UsersyncInfo: UsersyncInfo{
                    URL:         syncURL,
                    Type:        "redirect",
                    SupportCORS: true,
                },
            })
        }
    }

    json.NewEncoder(w).Encode(response)
}
```

---

## Sources

- [Prebid.js Bidder Adapter Guide](https://docs.prebid.org/dev-docs/bidder-adaptor.html)
- [Prebid Server Adapter Module](https://docs.prebid.org/dev-docs/modules/prebidServer.html)
- [Prebid.js GitHub Repository](https://github.com/prebid/Prebid.js/)
- [pbjs.registerBidAdapter API](https://docs.prebid.org/dev-docs/internal-api-reference/registerBidAdapter.html)
- [Prebid Server Use Cases with Prebid.js](https://docs.prebid.org/prebid-server/use-cases/pbs-pbjs.html)
- [ORTB Converter Library](https://github.com/prebid/Prebid.js/blob/master/libraries/ortbConverter/README.md)

---

## Next Steps

1. **Decide on approach** based on your business requirements
2. **Implement cookie_sync endpoint** if needed (Approach 1)
3. **Create adapter code** if going with Approach 2 or 3
4. **Test integration** with sample Prebid.js page
5. **Document publisher integration** for your customers
6. **Monitor and iterate** based on publisher feedback
