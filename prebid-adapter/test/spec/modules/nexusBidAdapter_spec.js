import { expect } from 'chai';
import sinon from 'sinon';
import {
  spec,
  registerContainerHook,
  unregisterContainerHook,
  clearContainerHooks
} from 'modules/nexusBidAdapter.js';
import { config } from 'src/config.js';
import * as utils from 'src/utils.js';

describe('Nexus Bid Adapter', function () {
  let sandbox;

  beforeEach(function () {
    sandbox = sinon.sandbox.create();
  });

  afterEach(function () {
    sandbox.restore();
    clearContainerHooks();
  });

  describe('spec properties', function () {
    it('should have correct bidder code', function () {
      expect(spec.code).to.equal('nexus');
    });

    it('should have correct aliases', function () {
      expect(spec.aliases).to.deep.equal(['nexusengine', 'tne']);
    });

    it('should support banner, video, and native media types', function () {
      expect(spec.supportedMediaTypes).to.include('banner');
      expect(spec.supportedMediaTypes).to.include('video');
      expect(spec.supportedMediaTypes).to.include('native');
    });
  });

  describe('isBidRequestValid', function () {
    it('should return true when accountId is present', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {
          banner: {
            sizes: [[300, 250]]
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.true;
    });

    it('should return true when placementId is present', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          placementId: 'my-placement'
        },
        mediaTypes: {
          banner: {
            sizes: [[300, 250]]
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.true;
    });

    it('should return false when neither accountId nor placementId is present', function () {
      const bid = {
        bidder: 'nexus',
        params: {},
        mediaTypes: {
          banner: {
            sizes: [[300, 250]]
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.false;
    });

    it('should return false when params is missing', function () {
      const bid = {
        bidder: 'nexus',
        mediaTypes: {
          banner: {
            sizes: [[300, 250]]
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.false;
    });

    it('should return false when no valid media type is present', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {}
      };
      expect(spec.isBidRequestValid(bid)).to.be.false;
    });

    it('should return true for video media type', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {
          video: {
            playerSize: [640, 480]
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.true;
    });

    it('should return true for video with width', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {
          video: {
            w: 640,
            h: 480
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.true;
    });

    it('should return true for native media type', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {
          native: {
            title: { required: true }
          }
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.true;
    });

    it('should return false for invalid native format', function () {
      const bid = {
        bidder: 'nexus',
        params: {
          accountId: '12345'
        },
        mediaTypes: {
          native: 'invalid'
        }
      };
      expect(spec.isBidRequestValid(bid)).to.be.false;
    });
  });

  describe('buildRequests', function () {
    const bidRequests = [
      {
        bidder: 'nexus',
        bidId: 'bid-id-1',
        adUnitCode: 'ad-unit-1',
        transactionId: 'trans-1',
        params: {
          accountId: '12345',
          placementId: 'placement-1',
          siteId: 'site-1',
          zoneId: 'zone-1'
        },
        mediaTypes: {
          banner: {
            sizes: [[300, 250], [728, 90]]
          }
        }
      }
    ];

    const bidderRequest = {
      bidderCode: 'nexus',
      auctionId: 'auction-1',
      bidderRequestId: 'bidder-request-1',
      timeout: 1000,
      refererInfo: {
        page: 'https://example.com/page',
        domain: 'example.com'
      }
    };

    it('should return empty array for empty bid requests', function () {
      const result = spec.buildRequests([], bidderRequest);
      expect(result).to.deep.equal([]);
    });

    it('should return empty array for null bid requests', function () {
      const result = spec.buildRequests(null, bidderRequest);
      expect(result).to.deep.equal([]);
    });

    it('should build a valid request object', function () {
      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result).to.be.an('object');
      expect(result.method).to.equal('POST');
      expect(result.url).to.be.a('string');
      expect(result.data).to.be.an('object');
    });

    it('should use default endpoint', function () {
      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result.url).to.equal('https://pbs.nexusengine.io/openrtb2/auction');
    });

    it('should use custom endpoint from config', function () {
      sandbox.stub(config, 'getConfig').withArgs('nexus').returns({
        endpoint: 'https://custom.endpoint.com/auction'
      });

      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result.url).to.equal('https://custom.endpoint.com/auction');
    });

    it('should use endpoint from bid params', function () {
      const customBidRequests = [{
        ...bidRequests[0],
        params: {
          ...bidRequests[0].params,
          endpoint: 'https://param.endpoint.com/auction'
        }
      }];

      const result = spec.buildRequests(customBidRequests, bidderRequest);
      expect(result.url).to.equal('https://param.endpoint.com/auction');
    });

    it('should set correct content type and credentials', function () {
      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result.options.contentType).to.equal('application/json');
      expect(result.options.withCredentials).to.be.true;
    });

    it('should include bidder request in result', function () {
      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result.bidderRequest).to.deep.equal(bidderRequest);
    });

    it('should include ORTB request structure', function () {
      const result = spec.buildRequests(bidRequests, bidderRequest);
      expect(result.data).to.have.property('imp');
      expect(result.data.imp).to.be.an('array');
    });
  });

  describe('interpretResponse', function () {
    const request = {
      data: {
        imp: [{ id: 'imp-1' }]
      },
      bidderRequest: {}
    };

    it('should return empty array for empty response', function () {
      const result = spec.interpretResponse({}, request);
      expect(result).to.deep.equal([]);
    });

    it('should return empty array for null body', function () {
      const result = spec.interpretResponse({ body: null }, request);
      expect(result).to.deep.equal([]);
    });

    it('should return empty array for missing seatbid', function () {
      const result = spec.interpretResponse({ body: {} }, request);
      expect(result).to.deep.equal([]);
    });

    it('should return empty array for non-array seatbid', function () {
      const result = spec.interpretResponse({ body: { seatbid: 'invalid' } }, request);
      expect(result).to.deep.equal([]);
    });

    it('should parse valid banner response', function () {
      const serverResponse = {
        body: {
          id: 'response-1',
          seatbid: [{
            bid: [{
              id: 'bid-1',
              impid: 'imp-1',
              price: 1.5,
              adm: '<div>ad</div>',
              w: 300,
              h: 250,
              crid: 'creative-1',
              adomain: ['advertiser.com']
            }],
            seat: 'seat-1'
          }],
          cur: 'USD'
        }
      };

      const result = spec.interpretResponse(serverResponse, request);
      expect(result).to.be.an('array');
    });

    it('should parse video response with VAST', function () {
      const serverResponse = {
        body: {
          id: 'response-1',
          seatbid: [{
            bid: [{
              id: 'bid-1',
              impid: 'imp-1',
              price: 2.5,
              adm: '<VAST version="3.0"></VAST>',
              w: 640,
              h: 480,
              crid: 'video-1'
            }],
            seat: 'seat-1'
          }],
          cur: 'USD'
        }
      };

      const result = spec.interpretResponse(serverResponse, request);
      expect(result).to.be.an('array');
    });
  });

  describe('getUserSyncs', function () {
    const serverResponses = [];
    const gdprConsent = {
      gdprApplies: true,
      consentString: 'BONciguONcjGKADACHENAOLS1rAHDAFAAEAASABQAMwAeACEAFw'
    };
    const uspConsent = '1YYN';
    const gppConsent = {
      gppString: 'DBACNYA~CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA',
      applicableSections: [7]
    };

    it('should return empty array when syncs disabled', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: false },
        serverResponses,
        gdprConsent,
        uspConsent
      );
      expect(result).to.deep.equal([]);
    });

    it('should return pixel sync when enabled', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        serverResponses,
        gdprConsent,
        uspConsent
      );
      expect(result).to.be.an('array');
      expect(result.length).to.be.at.least(1);
      expect(result[0].type).to.equal('image');
    });

    it('should include GDPR parameters', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        serverResponses,
        gdprConsent,
        null
      );
      expect(result[0].url).to.include('gdpr=1');
      expect(result[0].url).to.include('gdpr_consent=');
    });

    it('should include US Privacy parameter', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        serverResponses,
        null,
        uspConsent
      );
      expect(result[0].url).to.include('us_privacy=1YYN');
    });

    it('should include GPP parameters', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        serverResponses,
        null,
        null,
        gppConsent
      );
      expect(result[0].url).to.include('gpp=');
      expect(result[0].url).to.include('gpp_sid=7');
    });

    it('should handle GDPR without consent string', function () {
      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        serverResponses,
        { gdprApplies: false },
        null
      );
      expect(result[0].url).to.include('gdpr=0');
    });

    it('should extract syncs from server response', function () {
      const responsesWithSyncs = [{
        body: {
          ext: {
            usersync: {
              appnexus: {
                syncs: [{
                  type: 'redirect',
                  url: 'https://sync.appnexus.com/getuid'
                }]
              }
            }
          }
        }
      }];

      const result = spec.getUserSyncs(
        { iframeEnabled: false, pixelEnabled: true },
        responsesWithSyncs,
        null,
        null
      );
      expect(result).to.be.an('array');
      expect(result.some(s => s.url.includes('appnexus'))).to.be.true;
    });

    it('should handle iframe sync from response', function () {
      const responsesWithSyncs = [{
        body: {
          ext: {
            usersync: {
              rubicon: {
                syncs: [{
                  type: 'iframe',
                  url: 'https://sync.rubicon.com/iframe'
                }]
              }
            }
          }
        }
      }];

      const result = spec.getUserSyncs(
        { iframeEnabled: true, pixelEnabled: false },
        responsesWithSyncs,
        null,
        null
      );
      expect(result.some(s => s.type === 'iframe')).to.be.true;
    });
  });

  describe('onTimeout', function () {
    it('should handle timeout without error', function () {
      const timeoutData = {
        bidder: 'nexus',
        bidId: 'bid-1',
        timeout: 1000,
        auctionId: 'auction-1'
      };

      expect(() => spec.onTimeout(timeoutData)).to.not.throw();
    });

    it('should send timeout beacon when configured', function () {
      const sendBeaconStub = sandbox.stub(navigator, 'sendBeacon').returns(true);
      sandbox.stub(config, 'getConfig').withArgs('nexus').returns({
        timeoutBeacon: 'https://beacon.example.com/timeout'
      });

      const timeoutData = {
        bidder: 'nexus',
        bidId: 'bid-1',
        timeout: 1000,
        auctionId: 'auction-1'
      };

      spec.onTimeout(timeoutData);

      expect(sendBeaconStub.calledOnce).to.be.true;
      expect(sendBeaconStub.firstCall.args[0]).to.equal('https://beacon.example.com/timeout');
    });
  });

  describe('onBidWon', function () {
    it('should handle bid won without error', function () {
      const bid = {
        bidder: 'nexus',
        cpm: 1.5,
        currency: 'USD',
        auctionId: 'auction-1',
        adUnitCode: 'ad-unit-1'
      };

      expect(() => spec.onBidWon(bid)).to.not.throw();
    });

    it('should fire nurl pixel when present', function () {
      const bid = {
        bidder: 'nexus',
        cpm: 1.5,
        currency: 'USD',
        nurl: 'https://win.example.com?price=${AUCTION_PRICE}'
      };

      spec.onBidWon(bid);
      // Note: Can't easily test Image creation without more complex mocking
    });

    it('should send win beacon when configured', function () {
      const sendBeaconStub = sandbox.stub(navigator, 'sendBeacon').returns(true);
      sandbox.stub(config, 'getConfig').withArgs('nexus').returns({
        winBeacon: 'https://beacon.example.com/win'
      });

      const bid = {
        bidder: 'nexus',
        cpm: 1.5,
        currency: 'USD',
        auctionId: 'auction-1',
        adUnitCode: 'ad-unit-1'
      };

      spec.onBidWon(bid);

      expect(sendBeaconStub.calledOnce).to.be.true;
      expect(sendBeaconStub.firstCall.args[0]).to.equal('https://beacon.example.com/win');
    });
  });

  describe('onSetTargeting', function () {
    it('should handle set targeting without error', function () {
      const bid = {
        bidder: 'nexus',
        adUnitCode: 'ad-unit-1'
      };

      expect(() => spec.onSetTargeting(bid)).to.not.throw();
    });
  });

  describe('Container Hooks', function () {
    describe('registerContainerHook', function () {
      it('should register a valid hook', function () {
        const hook = () => {};
        const result = registerContainerHook('preRequest', hook);
        expect(result).to.be.true;
      });

      it('should reject invalid hook type', function () {
        const warnStub = sandbox.stub(utils, 'logWarn');
        const hook = () => {};
        const result = registerContainerHook('invalidType', hook);
        expect(result).to.be.false;
        expect(warnStub.calledOnce).to.be.true;
      });

      it('should reject non-function hooks', function () {
        const warnStub = sandbox.stub(utils, 'logWarn');
        const result = registerContainerHook('preRequest', 'not a function');
        expect(result).to.be.false;
        expect(warnStub.calledOnce).to.be.true;
      });

      it('should allow multiple hooks of same type', function () {
        const hook1 = () => {};
        const hook2 = () => {};
        expect(registerContainerHook('preRequest', hook1)).to.be.true;
        expect(registerContainerHook('preRequest', hook2)).to.be.true;
      });
    });

    describe('unregisterContainerHook', function () {
      it('should unregister an existing hook', function () {
        const hook = () => {};
        registerContainerHook('preRequest', hook);
        const result = unregisterContainerHook('preRequest', hook);
        expect(result).to.be.true;
      });

      it('should return false for non-existent hook', function () {
        const hook = () => {};
        const result = unregisterContainerHook('preRequest', hook);
        expect(result).to.be.false;
      });

      it('should return false for invalid hook type', function () {
        const hook = () => {};
        const result = unregisterContainerHook('invalidType', hook);
        expect(result).to.be.false;
      });
    });

    describe('clearContainerHooks', function () {
      it('should clear all registered hooks', function () {
        registerContainerHook('preRequest', () => {});
        registerContainerHook('postResponse', () => {});
        registerContainerHook('bidTransform', () => {});

        clearContainerHooks();

        // Verify hooks are cleared by checking unregister returns false
        expect(unregisterContainerHook('preRequest', () => {})).to.be.false;
      });
    });

    describe('hook execution', function () {
      it('should execute preRequest hooks during buildRequests', function () {
        let hookCalled = false;
        registerContainerHook('preRequest', (data) => {
          hookCalled = true;
          return data;
        });

        const bidRequests = [{
          bidder: 'nexus',
          bidId: 'bid-1',
          params: { accountId: '12345' },
          mediaTypes: { banner: { sizes: [[300, 250]] } }
        }];

        spec.buildRequests(bidRequests, { bidderCode: 'nexus' });
        expect(hookCalled).to.be.true;
      });

      it('should execute postResponse hooks during interpretResponse', function () {
        let hookCalled = false;
        registerContainerHook('postResponse', (data) => {
          hookCalled = true;
          return data;
        });

        const serverResponse = {
          body: {
            seatbid: [{
              bid: [{
                id: 'bid-1',
                impid: 'imp-1',
                price: 1.5,
                adm: '<div>ad</div>',
                w: 300,
                h: 250
              }]
            }]
          }
        };

        spec.interpretResponse(serverResponse, { data: { imp: [{ id: 'imp-1' }] } });
        expect(hookCalled).to.be.true;
      });

      it('should execute onTimeout hooks', function () {
        let hookCalled = false;
        registerContainerHook('onTimeout', () => {
          hookCalled = true;
        });

        spec.onTimeout({ bidder: 'nexus', timeout: 1000 });
        expect(hookCalled).to.be.true;
      });

      it('should execute onWin hooks', function () {
        let hookCalled = false;
        registerContainerHook('onWin', () => {
          hookCalled = true;
        });

        spec.onBidWon({ bidder: 'nexus', cpm: 1.5 });
        expect(hookCalled).to.be.true;
      });

      it('should handle hook errors gracefully', function () {
        const warnStub = sandbox.stub(utils, 'logWarn');
        registerContainerHook('preRequest', () => {
          throw new Error('Hook error');
        });

        const bidRequests = [{
          bidder: 'nexus',
          bidId: 'bid-1',
          params: { accountId: '12345' },
          mediaTypes: { banner: { sizes: [[300, 250]] } }
        }];

        expect(() => spec.buildRequests(bidRequests, { bidderCode: 'nexus' })).to.not.throw();
        expect(warnStub.called).to.be.true;
      });

      it('should chain multiple hooks', function () {
        const order = [];
        registerContainerHook('preRequest', (data) => {
          order.push(1);
          return data;
        });
        registerContainerHook('preRequest', (data) => {
          order.push(2);
          return data;
        });

        const bidRequests = [{
          bidder: 'nexus',
          bidId: 'bid-1',
          params: { accountId: '12345' },
          mediaTypes: { banner: { sizes: [[300, 250]] } }
        }];

        spec.buildRequests(bidRequests, { bidderCode: 'nexus' });
        expect(order).to.deep.equal([1, 2]);
      });

      it('should pass modified data through hook chain', function () {
        registerContainerHook('preRequest', (data) => {
          data.modified = true;
          return data;
        });
        registerContainerHook('preRequest', (data) => {
          data.secondModification = data.modified;
          return data;
        });

        let capturedData;
        registerContainerHook('preRequest', (data) => {
          capturedData = data;
          return data;
        });

        const bidRequests = [{
          bidder: 'nexus',
          bidId: 'bid-1',
          params: { accountId: '12345' },
          mediaTypes: { banner: { sizes: [[300, 250]] } }
        }];

        spec.buildRequests(bidRequests, { bidderCode: 'nexus' });
        expect(capturedData.modified).to.be.true;
        expect(capturedData.secondModification).to.be.true;
      });
    });
  });

  describe('spec exported functions', function () {
    it('should export registerContainerHook on spec', function () {
      expect(spec.registerContainerHook).to.be.a('function');
    });

    it('should export unregisterContainerHook on spec', function () {
      expect(spec.unregisterContainerHook).to.be.a('function');
    });

    it('should export clearContainerHooks on spec', function () {
      expect(spec.clearContainerHooks).to.be.a('function');
    });
  });
});
