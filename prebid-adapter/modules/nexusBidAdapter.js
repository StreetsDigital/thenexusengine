/**
 * @module modules/nexusBidAdapter
 * @description Bidder adapter for The Nexus Engine Prebid Server
 * @author StreetsDigital
 */

import { registerBidder } from '../src/adapters/bidderFactory.js';
import { BANNER, VIDEO, NATIVE } from '../src/mediaTypes.js';
import { deepAccess, deepSetValue, isArray, isPlainObject, logWarn, logInfo, generateUUID } from '../src/utils.js';
import { ortbConverter } from '../libraries/ortbConverter/converter.js';
import { config } from '../src/config.js';

const BIDDER_CODE = 'nexus';
const ENDPOINT_URL = 'https://pbs.nexusengine.io/openrtb2/auction';
const COOKIE_SYNC_URL = 'https://pbs.nexusengine.io/cookie_sync';
const USER_SYNC_URL = 'https://pbs.nexusengine.io/setuid';
const GVLID = null; // To be assigned by IAB upon registration
const VERSION = '1.0.0';

/**
 * Container/Edge Compute Hook System
 *
 * This architecture allows for containerized logic to be injected into the bid flow.
 * In the initial implementation, hooks are no-ops but provide extension points for:
 * - Pre-request transformation (e.g., audience enrichment)
 * - Post-response processing (e.g., brand safety filtering)
 * - Real-time bid modification (e.g., floor price adjustment)
 * - Custom analytics/logging
 *
 * Future implementations can register container functions via:
 * nexusBidAdapter.registerContainerHook('preRequest', containerFn)
 */
const containerHooks = {
  /**
   * Called before building the OpenRTB request
   * @param {Object} bidRequests - Array of valid bid requests
   * @param {Object} bidderRequest - The bidder request object
   * @returns {Object} Modified or original bid data
   */
  preRequest: [],

  /**
   * Called after receiving server response, before interpretation
   * @param {Object} serverResponse - Raw server response
   * @param {Object} request - Original request object
   * @returns {Object} Modified or original response
   */
  postResponse: [],

  /**
   * Called for each interpreted bid before returning to Prebid
   * @param {Object} bid - Interpreted bid object
   * @returns {Object} Modified or original bid
   */
  bidTransform: [],

  /**
   * Called when a bid wins the auction
   * @param {Object} bid - Winning bid object
   */
  onWin: [],

  /**
   * Called on bid timeout
   * @param {Object} timeoutData - Timeout information
   */
  onTimeout: []
};

/**
 * Execute container hooks of a specific type
 * @param {string} hookType - Type of hook to execute
 * @param {*} data - Data to pass through hooks
 * @returns {*} Transformed data after all hooks
 */
function executeHooks(hookType, data) {
  const hooks = containerHooks[hookType] || [];
  if (hooks.length === 0) {
    return data;
  }

  let result = data;
  for (const hook of hooks) {
    try {
      const hookResult = hook(result);
      if (hookResult !== undefined) {
        result = hookResult;
      }
    } catch (e) {
      logWarn(`${BIDDER_CODE}: Container hook error in ${hookType}:`, e);
    }
  }
  return result;
}

/**
 * Register a container hook
 * @param {string} hookType - Type of hook (preRequest, postResponse, bidTransform, onWin, onTimeout)
 * @param {Function} hookFn - Hook function to register
 */
export function registerContainerHook(hookType, hookFn) {
  if (!containerHooks[hookType]) {
    logWarn(`${BIDDER_CODE}: Unknown hook type: ${hookType}`);
    return false;
  }
  if (typeof hookFn !== 'function') {
    logWarn(`${BIDDER_CODE}: Hook must be a function`);
    return false;
  }
  containerHooks[hookType].push(hookFn);
  logInfo(`${BIDDER_CODE}: Registered ${hookType} container hook`);
  return true;
}

/**
 * Unregister a container hook
 * @param {string} hookType - Type of hook
 * @param {Function} hookFn - Hook function to remove
 */
export function unregisterContainerHook(hookType, hookFn) {
  if (!containerHooks[hookType]) {
    return false;
  }
  const index = containerHooks[hookType].indexOf(hookFn);
  if (index > -1) {
    containerHooks[hookType].splice(index, 1);
    return true;
  }
  return false;
}

/**
 * Clear all container hooks (useful for testing)
 */
export function clearContainerHooks() {
  Object.keys(containerHooks).forEach(type => {
    containerHooks[type] = [];
  });
}

/**
 * ORTB Converter configuration for request/response transformation
 */
const converter = ortbConverter({
  context: {
    netRevenue: true,
    ttl: 300
  },
  imp(buildImp, bidRequest, context) {
    const imp = buildImp(bidRequest, context);

    // Add Nexus-specific extensions
    const params = bidRequest.params || {};

    deepSetValue(imp, 'ext.bidder', {
      placementId: params.placementId,
      accountId: params.accountId,
      siteId: params.siteId,
      zoneId: params.zoneId
    });

    // Container hook extension point for impression-level customization
    deepSetValue(imp, 'ext.nexus', {
      containerEnabled: params.containerEnabled || false,
      containerConfig: params.containerConfig || {}
    });

    return imp;
  },
  request(buildRequest, imps, bidderRequest, context) {
    const request = buildRequest(imps, bidderRequest, context);

    // Add source information
    deepSetValue(request, 'source.ext.prebid', {
      version: '$prebid.version$',
      adapter: BIDDER_CODE,
      adapterVersion: VERSION
    });

    // Add Nexus-specific request extensions
    const nexusConfig = config.getConfig('nexus') || {};
    deepSetValue(request, 'ext.nexus', {
      version: VERSION,
      idrEnabled: nexusConfig.idrEnabled !== false, // Intelligent Demand Router
      containerHooksEnabled: Object.values(containerHooks).some(h => h.length > 0)
    });

    return request;
  },
  bidResponse(buildBidResponse, bid, context) {
    const bidResponse = buildBidResponse(bid, context);

    // Apply container bid transform hooks
    return executeHooks('bidTransform', bidResponse);
  }
});

/**
 * Bid adapter specification
 */
export const spec = {
  code: BIDDER_CODE,
  gvlid: GVLID,
  supportedMediaTypes: [BANNER, VIDEO, NATIVE],

  /**
   * Alias for the bidder (optional)
   */
  aliases: ['nexusengine', 'tne'],

  /**
   * Validate bid request parameters
   * @param {Object} bid - Bid request to validate
   * @returns {boolean} True if valid
   */
  isBidRequestValid(bid) {
    const params = bid.params || {};

    // Require at least accountId or placementId
    if (!params.accountId && !params.placementId) {
      logWarn(`${BIDDER_CODE}: Missing required params. Need accountId or placementId`);
      return false;
    }

    // Validate media types
    const mediaTypes = bid.mediaTypes || {};
    const hasBanner = mediaTypes.banner &&
      (isArray(mediaTypes.banner.sizes) || mediaTypes.banner.sizes);
    const hasVideo = mediaTypes.video &&
      (mediaTypes.video.playerSize || mediaTypes.video.w);
    const hasNative = mediaTypes.native &&
      isPlainObject(mediaTypes.native);

    if (!hasBanner && !hasVideo && !hasNative) {
      logWarn(`${BIDDER_CODE}: No valid media type found`);
      return false;
    }

    return true;
  },

  /**
   * Build OpenRTB request to send to server
   * @param {Array} validBidRequests - Valid bid requests
   * @param {Object} bidderRequest - Bidder request context
   * @returns {Object} Server request configuration
   */
  buildRequests(validBidRequests, bidderRequest) {
    if (!validBidRequests || validBidRequests.length === 0) {
      return [];
    }

    // Execute pre-request container hooks
    const hookData = executeHooks('preRequest', {
      bidRequests: validBidRequests,
      bidderRequest: bidderRequest
    });

    const processedBidRequests = hookData.bidRequests || validBidRequests;
    const processedBidderRequest = hookData.bidderRequest || bidderRequest;

    // Use ORTB converter to build the request
    const ortbRequest = converter.toORTB({
      bidRequests: processedBidRequests,
      bidderRequest: processedBidderRequest
    });

    // Determine endpoint URL
    let endpoint = ENDPOINT_URL;
    const nexusConfig = config.getConfig('nexus') || {};
    if (nexusConfig.endpoint) {
      endpoint = nexusConfig.endpoint;
    }
    // Allow per-request endpoint override
    const firstBid = processedBidRequests[0];
    if (firstBid.params && firstBid.params.endpoint) {
      endpoint = firstBid.params.endpoint;
    }

    return {
      method: 'POST',
      url: endpoint,
      data: ortbRequest,
      options: {
        contentType: 'application/json',
        withCredentials: true
      },
      bidderRequest: processedBidderRequest
    };
  },

  /**
   * Interpret server response and extract bids
   * @param {Object} serverResponse - Server response
   * @param {Object} request - Original request
   * @returns {Array} Array of bid responses
   */
  interpretResponse(serverResponse, request) {
    const body = serverResponse.body;

    if (!body || !body.seatbid || !isArray(body.seatbid)) {
      return [];
    }

    // Execute post-response container hooks
    const processedResponse = executeHooks('postResponse', {
      response: body,
      request: request
    });

    const responseBody = processedResponse.response || body;

    // Use ORTB converter to interpret response
    const bids = converter.fromORTB({
      response: responseBody,
      request: request.data
    }).bids;

    return bids;
  },

  /**
   * Get user syncs from server response
   * @param {Object} syncOptions - Sync configuration
   * @param {Array} serverResponses - Server responses
   * @param {Object} gdprConsent - GDPR consent data
   * @param {string} uspConsent - US Privacy consent string
   * @param {Object} gppConsent - GPP consent data
   * @returns {Array} User sync pixels/iframes
   */
  getUserSyncs(syncOptions, serverResponses, gdprConsent, uspConsent, gppConsent) {
    const syncs = [];

    // Check if syncs are enabled
    if (!syncOptions.iframeEnabled && !syncOptions.pixelEnabled) {
      return syncs;
    }

    // Build sync URL with privacy parameters
    let syncUrl = USER_SYNC_URL;
    const params = [];

    // Add GDPR parameters
    if (gdprConsent) {
      params.push(`gdpr=${gdprConsent.gdprApplies ? 1 : 0}`);
      if (gdprConsent.consentString) {
        params.push(`gdpr_consent=${encodeURIComponent(gdprConsent.consentString)}`);
      }
    }

    // Add US Privacy
    if (uspConsent) {
      params.push(`us_privacy=${encodeURIComponent(uspConsent)}`);
    }

    // Add GPP
    if (gppConsent && gppConsent.gppString) {
      params.push(`gpp=${encodeURIComponent(gppConsent.gppString)}`);
      if (gppConsent.applicableSections) {
        params.push(`gpp_sid=${gppConsent.applicableSections.join(',')}`);
      }
    }

    // Extract syncs from server response ext
    serverResponses.forEach(response => {
      const ext = deepAccess(response, 'body.ext.usersync') || {};
      Object.keys(ext).forEach(bidder => {
        const bidderSync = ext[bidder];
        if (bidderSync && bidderSync.syncs) {
          bidderSync.syncs.forEach(sync => {
            const type = sync.type === 'redirect' ? 'image' : 'iframe';
            if ((type === 'image' && syncOptions.pixelEnabled) ||
                (type === 'iframe' && syncOptions.iframeEnabled)) {
              syncs.push({
                type: type,
                url: sync.url
              });
            }
          });
        }
      });
    });

    // If no syncs from response, use default
    if (syncs.length === 0 && syncOptions.pixelEnabled) {
      const defaultSyncUrl = params.length > 0
        ? `${syncUrl}?${params.join('&')}`
        : syncUrl;
      syncs.push({
        type: 'image',
        url: defaultSyncUrl
      });
    }

    return syncs;
  },

  /**
   * Handle bid timeout
   * @param {Object} timeoutData - Timeout information
   */
  onTimeout(timeoutData) {
    logInfo(`${BIDDER_CODE}: Bid timeout`, timeoutData);

    // Execute timeout container hooks
    executeHooks('onTimeout', timeoutData);

    // Optional: Send timeout beacon
    const nexusConfig = config.getConfig('nexus') || {};
    if (nexusConfig.timeoutBeacon) {
      const beacon = {
        type: 'timeout',
        bidder: BIDDER_CODE,
        timeout: timeoutData.timeout,
        auctionId: timeoutData.auctionId,
        bidId: timeoutData.bidId,
        timestamp: Date.now()
      };

      navigator.sendBeacon && navigator.sendBeacon(
        nexusConfig.timeoutBeacon,
        JSON.stringify(beacon)
      );
    }
  },

  /**
   * Handle bid won
   * @param {Object} bid - Winning bid
   */
  onBidWon(bid) {
    logInfo(`${BIDDER_CODE}: Bid won`, bid);

    // Execute win container hooks
    executeHooks('onWin', bid);

    // Fire win notification if URL provided
    if (bid.nurl) {
      const winUrl = bid.nurl
        .replace(/\$\{AUCTION_PRICE\}/g, bid.cpm)
        .replace(/\$\{AUCTION_CURRENCY\}/g, bid.currency || 'USD');

      // Use pixel for win notification
      const img = new Image();
      img.src = winUrl;
    }

    // Optional: Send win beacon
    const nexusConfig = config.getConfig('nexus') || {};
    if (nexusConfig.winBeacon) {
      const beacon = {
        type: 'win',
        bidder: BIDDER_CODE,
        cpm: bid.cpm,
        currency: bid.currency,
        auctionId: bid.auctionId,
        adUnitCode: bid.adUnitCode,
        timestamp: Date.now()
      };

      navigator.sendBeacon && navigator.sendBeacon(
        nexusConfig.winBeacon,
        JSON.stringify(beacon)
      );
    }
  },

  /**
   * Handle set targeting
   * @param {Object} bid - Bid with targeting
   */
  onSetTargeting(bid) {
    logInfo(`${BIDDER_CODE}: Set targeting`, bid.adUnitCode);
  }
};

// Export container hook functions for external use
spec.registerContainerHook = registerContainerHook;
spec.unregisterContainerHook = unregisterContainerHook;
spec.clearContainerHooks = clearContainerHooks;

registerBidder(spec);

export default spec;
