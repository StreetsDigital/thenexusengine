# Overview

```
Module Name: TNE Bid Adapter
Module Type: Bidder Adapter
Maintainer: dev@streetsdigital.com
```

# Description

The TNE Bid Adapter connects to The Nexus Engine Prebid Server (PBS), providing access to premium demand through a high-performance, ML-powered bidding infrastructure.

## Features

- **OpenRTB 2.5/2.6 Compliant**: Full support for the OpenRTB specification
- **Multi-Format Support**: Banner, Video (instream/outstream), and Native ad formats
- **Intelligent Demand Router (IDR)**: ML-powered bidder selection for optimal yield
- **Privacy Compliant**: Full support for GDPR (TCF 2.0), CCPA/CPRA, and GPP
- **Container Hooks**: Extensible architecture for custom processing logic (beta)

# Bid Parameters

| Parameter | Scope | Type | Description | Example |
|-----------|-------|------|-------------|---------|
| `accountId` | required* | string | Your TNE account ID | `"12345"` |
| `placementId` | required* | string | Placement identifier | `"homepage-banner"` |
| `siteId` | optional | string | Site identifier for multi-site publishers | `"site-001"` |
| `zoneId` | optional | string | Zone identifier for section targeting | `"sports"` |
| `endpoint` | optional | string | Custom PBS endpoint URL | `"https://custom.pbs.com/openrtb2/auction"` |
| `containerEnabled` | optional | boolean | Enable container hooks for this bid | `true` |
| `containerConfig` | optional | object | Configuration for container hooks | `{}` |

\* Either `accountId` or `placementId` is required.

# Configuration

## Basic Configuration

```javascript
pbjs.setConfig({
  tne: {
    endpoint: 'https://pbs.thenexusengine.io/openrtb2/auction', // Optional: custom endpoint
    idrEnabled: true // Optional: enable Intelligent Demand Router (default: true)
  }
});
```

## User Sync Configuration

```javascript
pbjs.setConfig({
  userSync: {
    filterSettings: {
      iframe: {
        bidders: ['tne'],
        filter: 'include'
      },
      image: {
        bidders: ['tne'],
        filter: 'include'
      }
    }
  }
});
```

# Examples

## Banner Ad Unit

```javascript
var adUnits = [{
  code: 'banner-div',
  mediaTypes: {
    banner: {
      sizes: [[300, 250], [728, 90], [300, 600]]
    }
  },
  bids: [{
    bidder: 'tne',
    params: {
      accountId: '12345',
      placementId: 'homepage-banner'
    }
  }]
}];
```

## Video Ad Unit (Instream)

```javascript
var adUnits = [{
  code: 'video-div',
  mediaTypes: {
    video: {
      context: 'instream',
      playerSize: [640, 480],
      mimes: ['video/mp4', 'video/webm'],
      protocols: [2, 3, 5, 6],
      playbackmethod: [1, 2],
      skip: 1,
      skipafter: 5
    }
  },
  bids: [{
    bidder: 'tne',
    params: {
      accountId: '12345',
      placementId: 'video-preroll',
      siteId: 'main-site'
    }
  }]
}];
```

## Video Ad Unit (Outstream)

```javascript
var adUnits = [{
  code: 'outstream-div',
  mediaTypes: {
    video: {
      context: 'outstream',
      playerSize: [640, 480],
      mimes: ['video/mp4'],
      protocols: [2, 3]
    }
  },
  bids: [{
    bidder: 'tne',
    params: {
      accountId: '12345',
      placementId: 'outstream-unit'
    }
  }]
}];
```

## Native Ad Unit

```javascript
var adUnits = [{
  code: 'native-div',
  mediaTypes: {
    native: {
      title: {
        required: true,
        len: 80
      },
      body: {
        required: true,
        len: 200
      },
      image: {
        required: true,
        sizes: [300, 250]
      },
      sponsoredBy: {
        required: true
      },
      clickUrl: {
        required: true
      }
    }
  },
  bids: [{
    bidder: 'tne',
    params: {
      accountId: '12345',
      placementId: 'native-feed'
    }
  }]
}];
```

## Multi-Format Ad Unit

```javascript
var adUnits = [{
  code: 'multi-format-div',
  mediaTypes: {
    banner: {
      sizes: [[300, 250], [300, 600]]
    },
    video: {
      context: 'outstream',
      playerSize: [300, 250],
      mimes: ['video/mp4']
    },
    native: {
      title: { required: true },
      body: { required: true },
      image: { required: true, sizes: [300, 250] }
    }
  },
  bids: [{
    bidder: 'tne',
    params: {
      accountId: '12345',
      placementId: 'multi-format-unit'
    }
  }]
}];
```

# Container Hooks (Beta)

The TNE adapter includes an extensible container hook system for advanced use cases. This allows you to inject custom logic at various points in the bid lifecycle.

## Hook Types

| Hook | Description | Parameters |
|------|-------------|------------|
| `preRequest` | Called before building OpenRTB request | `{bidRequests, bidderRequest}` |
| `postResponse` | Called after receiving server response | `{response, request}` |
| `bidTransform` | Called for each interpreted bid | `bid` |
| `onWin` | Called when a bid wins | `bid` |
| `onTimeout` | Called on bid timeout | `timeoutData` |

## Example: Pre-Request Hook

```javascript
// Add custom audience data to all requests
tneBidAdapter.registerContainerHook('preRequest', function(data) {
  data.bidRequests.forEach(function(bid) {
    bid.params.customAudience = getAudienceSegments();
  });
  return data;
});
```

## Example: Bid Transform Hook

```javascript
// Apply custom floor logic
tneBidAdapter.registerContainerHook('bidTransform', function(bid) {
  const floor = getCustomFloor(bid.adUnitCode);
  if (bid.cpm < floor) {
    bid.cpm = 0; // Reject bid below floor
  }
  return bid;
});
```

## Example: Win Tracking Hook

```javascript
// Custom win analytics
tneBidAdapter.registerContainerHook('onWin', function(bid) {
  analytics.track('bid_won', {
    bidder: 'tne',
    cpm: bid.cpm,
    adUnit: bid.adUnitCode
  });
});
```

# Privacy Support

## GDPR/TCF 2.0

The adapter fully supports GDPR through the TCF 2.0 framework. Consent data is automatically passed to the server.

```javascript
pbjs.setConfig({
  consentManagement: {
    gdpr: {
      cmpApi: 'iab',
      timeout: 10000,
      defaultGdprScope: true
    }
  }
});
```

## CCPA/CPRA (US Privacy)

US Privacy consent strings are automatically forwarded to the server.

```javascript
pbjs.setConfig({
  consentManagement: {
    usp: {
      cmpApi: 'iab',
      timeout: 100
    }
  }
});
```

## GPP (Global Privacy Platform)

GPP support is included for comprehensive privacy compliance.

```javascript
pbjs.setConfig({
  consentManagement: {
    gpp: {
      cmpApi: 'iab',
      timeout: 10000
    }
  }
});
```

# First Party Data

The adapter supports first party data through the standard Prebid.js configuration:

```javascript
pbjs.setConfig({
  ortb2: {
    site: {
      name: 'Example Site',
      domain: 'example.com',
      cat: ['IAB1'],
      content: {
        genre: 'news'
      }
    },
    user: {
      data: [{
        name: 'publisher-dmp',
        segment: [
          { id: 'seg-1' },
          { id: 'seg-2' }
        ]
      }]
    }
  }
});
```

# Troubleshooting

## No Bids Returned

1. Verify your `accountId` or `placementId` is correct
2. Check that your ad sizes match available inventory
3. Ensure GDPR consent is properly configured if applicable
4. Review browser console for error messages

## Slow Response Times

1. The default timeout is 1000ms. You can adjust it:
   ```javascript
   pbjs.setConfig({ bidderTimeout: 1500 });
   ```
2. Check network latency to `pbs.thenexusengine.io`

## Debug Mode

Enable Prebid debug mode to see detailed logging:

```javascript
pbjs.setConfig({ debug: true });
```

Or add `?pbjs_debug=true` to your page URL.

# Support

For technical support or integration assistance:

- Email: dev@streetsdigital.com
- Documentation: https://docs.thenexusengine.io
- GitHub: https://github.com/StreetsDigital/thenexusengine

# Change Log

## v1.0.0

- Initial release
- Banner, video, and native support
- Container hooks architecture (beta)
- Full privacy compliance (GDPR, CCPA, GPP)
