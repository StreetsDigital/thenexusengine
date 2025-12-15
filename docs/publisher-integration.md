# Publisher Integration Guide

Welcome to The Nexus Engine Prebid Server. This guide will help you integrate your website with our Intelligent Demand Router for optimized header bidding.

## Quick Start

### Step 1: Add Prebid.js to Your Site

Add the Prebid.js library to your page's `<head>`:

```html
<script async src="https://cdn.jsdelivr.net/npm/prebid.js@latest/dist/not-for-prod/prebid.js"></script>
```

> **Production**: Use a custom Prebid.js build from [Prebid Download](https://docs.prebid.org/download.html) with only the modules you need.

### Step 2: Configure Prebid.js for Server-Side Bidding

```html
<script>
  var pbjs = pbjs || {};
  pbjs.que = pbjs.que || [];

  pbjs.que.push(function() {
    // Configure S2S (Server-to-Server) bidding
    pbjs.setConfig({
      s2sConfig: {
        accountId: 'YOUR_PUBLISHER_ID',        // We'll provide this
        bidders: ['appnexus', 'rubicon', 'pubmatic', 'openx', 'ix'],
        defaultVendor: 'pbs',
        timeout: 1000,
        endpoint: {
          p1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction',
          noP1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction'
        }
      }
    });
  });
</script>
```

**Replace:**
- `YOUR_PUBLISHER_ID` - Your assigned publisher ID
- `YOUR_PBS_DOMAIN` - The Prebid Server domain (we'll provide this)

### Step 3: Define Ad Units

```html
<script>
  var adUnits = [
    {
      code: 'div-banner-ad',
      mediaTypes: {
        banner: {
          sizes: [[300, 250], [320, 50]]
        }
      },
      bids: [
        { bidder: 'appnexus', params: { placementId: 12345 } },
        { bidder: 'rubicon', params: { accountId: 1001, siteId: 2002, zoneId: 3003 } },
        { bidder: 'pubmatic', params: { publisherId: '12345', adSlot: 'div-banner-ad' } }
      ]
    }
  ];

  pbjs.que.push(function() {
    pbjs.addAdUnits(adUnits);
  });
</script>
```

### Step 4: Request Bids and Render Ads

```html
<script>
  pbjs.que.push(function() {
    pbjs.requestBids({
      bidsBackHandler: function(bidResponses) {
        // Send to your ad server (e.g., GAM)
        pbjs.setTargetingForGPTAsync();
        googletag.pubads().refresh();
      },
      timeout: 1500
    });
  });
</script>

<!-- Ad container -->
<div id="div-banner-ad"></div>
```

---

## Complete Example

Here's a full working example:

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Website</title>

  <!-- Google Publisher Tag -->
  <script async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js"></script>

  <!-- Prebid.js -->
  <script async src="https://cdn.jsdelivr.net/npm/prebid.js@latest/dist/not-for-prod/prebid.js"></script>

  <script>
    var googletag = googletag || {};
    googletag.cmd = googletag.cmd || [];
    var pbjs = pbjs || {};
    pbjs.que = pbjs.que || [];

    // Prebid Configuration
    pbjs.que.push(function() {

      // Server-side bidding config
      pbjs.setConfig({
        debug: false,  // Set to true for testing
        s2sConfig: {
          accountId: 'YOUR_PUBLISHER_ID',
          bidders: ['appnexus', 'rubicon', 'pubmatic', 'openx', 'ix', 'triplelift'],
          defaultVendor: 'pbs',
          timeout: 1000,
          endpoint: {
            p1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction',
            noP1Consent: 'https://YOUR_PBS_DOMAIN/openrtb2/auction'
          }
        },

        // Price granularity
        priceGranularity: {
          buckets: [
            { max: 5, increment: 0.05 },
            { max: 10, increment: 0.10 },
            { max: 20, increment: 0.50 }
          ]
        },

        // Currency
        currency: {
          adServerCurrency: 'USD'
        }
      });

      // Define ad units
      var adUnits = [
        {
          code: 'leaderboard',
          mediaTypes: {
            banner: { sizes: [[728, 90], [970, 90]] }
          },
          bids: [
            { bidder: 'appnexus', params: { placementId: 12345 } },
            { bidder: 'rubicon', params: { accountId: 1001, siteId: 2002, zoneId: 3003 } }
          ]
        },
        {
          code: 'sidebar-ad',
          mediaTypes: {
            banner: { sizes: [[300, 250], [300, 600]] }
          },
          bids: [
            { bidder: 'appnexus', params: { placementId: 12346 } },
            { bidder: 'pubmatic', params: { publisherId: '12345', adSlot: 'sidebar' } }
          ]
        }
      ];

      pbjs.addAdUnits(adUnits);

      // Request bids
      pbjs.requestBids({
        bidsBackHandler: initAdserver,
        timeout: 1500
      });
    });

    // Initialize ad server after bids
    function initAdserver() {
      if (pbjs.initAdserverSet) return;
      pbjs.initAdserverSet = true;

      googletag.cmd.push(function() {
        pbjs.setTargetingForGPTAsync();
        googletag.pubads().refresh();
      });
    }

    // Failsafe timeout
    setTimeout(function() {
      initAdserver();
    }, 2000);

    // GPT setup
    googletag.cmd.push(function() {
      googletag.defineSlot('/YOUR_GAM_ID/leaderboard', [[728, 90], [970, 90]], 'leaderboard')
        .addService(googletag.pubads());
      googletag.defineSlot('/YOUR_GAM_ID/sidebar', [[300, 250], [300, 600]], 'sidebar-ad')
        .addService(googletag.pubads());

      googletag.pubads().disableInitialLoad();
      googletag.pubads().enableSingleRequest();
      googletag.enableServices();
    });
  </script>
</head>
<body>
  <header>
    <!-- Leaderboard Ad -->
    <div id="leaderboard">
      <script>googletag.cmd.push(function() { googletag.display('leaderboard'); });</script>
    </div>
  </header>

  <main>
    <article>Your content here...</article>
  </main>

  <aside>
    <!-- Sidebar Ad -->
    <div id="sidebar-ad">
      <script>googletag.cmd.push(function() { googletag.display('sidebar-ad'); });</script>
    </div>
  </aside>
</body>
</html>
```

---

## Ad Format Examples

### Banner Ads

```javascript
{
  code: 'banner-ad',
  mediaTypes: {
    banner: {
      sizes: [[300, 250], [320, 50], [728, 90]]
    }
  },
  bids: [
    { bidder: 'appnexus', params: { placementId: 12345 } }
  ]
}
```

### Video Ads (Outstream)

```javascript
{
  code: 'video-ad',
  mediaTypes: {
    video: {
      context: 'outstream',
      playerSize: [640, 480],
      mimes: ['video/mp4', 'video/webm'],
      protocols: [2, 3, 5, 6],
      playbackmethod: [1, 2],
      skip: 1
    }
  },
  bids: [
    { bidder: 'appnexus', params: { placementId: 12347 } }
  ]
}
```

### Video Ads (Instream)

```javascript
{
  code: 'instream-video',
  mediaTypes: {
    video: {
      context: 'instream',
      playerSize: [640, 480],
      mimes: ['video/mp4'],
      protocols: [2, 3],
      maxduration: 30,
      linearity: 1
    }
  },
  bids: [
    { bidder: 'spotx', params: { channel_id: '123456' } }
  ]
}
```

### Native Ads

```javascript
{
  code: 'native-ad',
  mediaTypes: {
    native: {
      title: { required: true, len: 80 },
      body: { required: true, len: 200 },
      image: { required: true, sizes: [300, 250] },
      sponsoredBy: { required: true },
      clickUrl: { required: true }
    }
  },
  bids: [
    { bidder: 'triplelift', params: { inventoryCode: 'your_inv_code' } }
  ]
}
```

---

## Bidder Parameters

### AppNexus
```javascript
{ bidder: 'appnexus', params: { placementId: 12345 } }
```

### Rubicon
```javascript
{
  bidder: 'rubicon',
  params: {
    accountId: 1001,
    siteId: 2002,
    zoneId: 3003
  }
}
```

### PubMatic
```javascript
{
  bidder: 'pubmatic',
  params: {
    publisherId: '12345',
    adSlot: 'your-ad-slot-name'
  }
}
```

### OpenX
```javascript
{
  bidder: 'openx',
  params: {
    unit: '540123456',
    delDomain: 'your-publisher.openx.net'
  }
}
```

### Index Exchange
```javascript
{ bidder: 'ix', params: { siteId: '123456', size: [300, 250] } }
```

### TripleLift
```javascript
{ bidder: 'triplelift', params: { inventoryCode: 'your_inventory_code' } }
```

> **Need bidder credentials?** Contact us to get set up with each demand partner.

---

## Privacy & Consent

### GDPR (TCF 2.0)

If you use a CMP (Consent Management Platform), Prebid.js will automatically pass consent:

```javascript
pbjs.setConfig({
  consentManagement: {
    gdpr: {
      cmpApi: 'iab',
      timeout: 5000,
      defaultGdprScope: true
    }
  }
});
```

### CCPA (US Privacy)

```javascript
pbjs.setConfig({
  consentManagement: {
    usp: {
      cmpApi: 'iab',
      timeout: 1000
    }
  }
});
```

### User ID Modules

Enable user ID modules for better match rates:

```javascript
pbjs.setConfig({
  userSync: {
    userIds: [
      {
        name: 'unifiedId',
        params: { partner: 'YOUR_PARTNER_ID' },
        storage: { type: 'cookie', name: '_uid2', expires: 30 }
      },
      {
        name: 'id5Id',
        params: { partner: 1234 },
        storage: { type: 'cookie', name: 'id5id', expires: 90 }
      }
    ],
    syncDelay: 3000
  }
});
```

---

## Testing

### Enable Debug Mode

Add `?pbjs_debug=true` to your URL or set in config:

```javascript
pbjs.setConfig({ debug: true });
```

### Check Browser Console

Look for:
- Bid requests being sent
- Bid responses received
- Any errors or warnings

### Test Endpoints

Verify our server is reachable:

```bash
curl https://YOUR_PBS_DOMAIN/health
```

---

## Common Issues

### No Bids Returned

1. Check browser console for errors
2. Verify bidder parameters are correct
3. Ensure ad sizes match bidder requirements
4. Check if consent was properly passed

### Slow Page Load

1. Use `async` script loading
2. Set appropriate timeouts (1000-1500ms)
3. Reduce number of bidders per ad unit

### Ads Not Rendering

1. Verify GPT slot IDs match Prebid ad unit codes
2. Check `setTargetingForGPTAsync()` is called
3. Ensure `googletag.pubads().refresh()` is triggered

---

## Support

- **Email**: support@thenexusengine.com
- **Documentation**: https://docs.thenexusengine.com
- **Status Page**: https://status.thenexusengine.com

---

## Checklist

Before going live:

- [ ] Received publisher ID from The Nexus Engine
- [ ] Configured S2S endpoint URL
- [ ] Set up bidder accounts and received credentials
- [ ] Added ad units with correct sizes
- [ ] Configured consent management (GDPR/CCPA)
- [ ] Tested in debug mode
- [ ] Verified ads rendering correctly
- [ ] Set appropriate timeouts
