# Publisher Quick Start

Get header bidding running on your site in 5 minutes.

## Your Credentials

| Setting | Value |
|---------|-------|
| Publisher ID | `[PROVIDED BY NEXUS ENGINE]` |
| PBS Endpoint | `https://[YOUR_PBS_DOMAIN]/openrtb2/auction` |

---

## Copy & Paste Integration

Add this code to your page's `<head>` section:

```html
<!-- Prebid.js -->
<script async src="https://cdn.jsdelivr.net/npm/prebid.js@latest/dist/not-for-prod/prebid.js"></script>

<script>
var pbjs = pbjs || {};
pbjs.que = pbjs.que || [];

pbjs.que.push(function() {
  // ========================================
  // STEP 1: Configure Server-Side Bidding
  // ========================================
  pbjs.setConfig({
    s2sConfig: {
      accountId: 'YOUR_PUBLISHER_ID',  // <-- Replace this
      bidders: ['appnexus', 'rubicon', 'pubmatic', 'openx', 'ix'],
      defaultVendor: 'pbs',
      timeout: 1000,
      endpoint: {
        p1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction',  // <-- Replace this
        noP1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction'
      }
    },
    priceGranularity: 'medium'
  });

  // ========================================
  // STEP 2: Define Your Ad Slots
  // ========================================
  var adUnits = [
    {
      code: 'div-ad-1',  // Must match your div ID
      mediaTypes: {
        banner: { sizes: [[300, 250], [320, 50]] }
      },
      bids: [
        { bidder: 'appnexus', params: { placementId: 12345 } },  // <-- Your placement IDs
        { bidder: 'rubicon', params: { accountId: 1001, siteId: 2002, zoneId: 3003 } }
      ]
    }
    // Add more ad units as needed
  ];

  pbjs.addAdUnits(adUnits);

  // ========================================
  // STEP 3: Request Bids
  // ========================================
  pbjs.requestBids({
    bidsBackHandler: function() {
      // If using GAM:
      pbjs.setTargetingForGPTAsync();
      googletag.pubads().refresh();
    },
    timeout: 1500
  });
});
</script>
```

---

## Common Ad Sizes

| Placement | Sizes |
|-----------|-------|
| Leaderboard | `[[728, 90], [970, 90]]` |
| Medium Rectangle | `[[300, 250], [336, 280]]` |
| Mobile Banner | `[[320, 50], [300, 50]]` |
| Skyscraper | `[[160, 600], [300, 600]]` |
| Billboard | `[[970, 250]]` |

---

## Add More Sites

Each site needs its own ad units. Just update the `code` and bidder `params`:

```javascript
// Site 1: news.example.com
{ code: 'news-leaderboard', params: { placementId: 11111 } }

// Site 2: sports.example.com
{ code: 'sports-leaderboard', params: { placementId: 22222 } }
```

---

## Test It

1. Add `?pbjs_debug=true` to your URL
2. Open browser console (F12)
3. Look for bid requests and responses
4. Verify ads are rendering

---

## Need Help?

Contact: support@thenexusengine.com
