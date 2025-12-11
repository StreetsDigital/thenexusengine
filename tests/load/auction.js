/**
 * k6 Load Test for The Nexus Engine PBS
 *
 * Run with: k6 run tests/load/auction.js
 *
 * Environment variables:
 *   PBS_URL - PBS server URL (default: http://localhost:8000)
 *   API_KEY - API key for authentication (optional)
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
const auctionErrors = new Counter('auction_errors');
const auctionSuccess = new Rate('auction_success_rate');
const bidResponseTime = new Trend('bid_response_time', true);
const bidsReceived = new Counter('bids_received');
const noBids = new Counter('no_bids');

// Configuration
const PBS_URL = __ENV.PBS_URL || 'http://localhost:8000';
const API_KEY = __ENV.API_KEY || '';

// Test scenarios
export const options = {
  scenarios: {
    // Smoke test - verify basic functionality
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      startTime: '0s',
      tags: { scenario: 'smoke' },
    },

    // Load test - normal traffic
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 10 },   // Ramp up
        { duration: '3m', target: 15 },   // Sustain ~15 RPS (avg traffic)
        { duration: '1m', target: 0 },    // Ramp down
      ],
      startTime: '30s',
      tags: { scenario: 'load' },
    },

    // Stress test - peak traffic
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 30 },  // Ramp up
        { duration: '2m', target: 75 },   // Peak ~75 RPS
        { duration: '30s', target: 0 },   // Ramp down
      ],
      startTime: '5m30s',
      tags: { scenario: 'stress' },
    },

    // Spike test - sudden traffic spike
    spike: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '10s', target: 100 }, // Spike
        { duration: '30s', target: 100 }, // Hold
        { duration: '10s', target: 5 },   // Recovery
      ],
      startTime: '8m30s',
      tags: { scenario: 'spike' },
    },
  },

  thresholds: {
    // Response time thresholds
    http_req_duration: ['p(95)<400', 'p(99)<800'], // 95th < 400ms, 99th < 800ms
    'http_req_duration{scenario:smoke}': ['p(95)<200'],
    'http_req_duration{scenario:load}': ['p(95)<400'],
    'http_req_duration{scenario:stress}': ['p(95)<500'],

    // Success rate thresholds
    auction_success_rate: ['rate>0.95'], // 95% success rate

    // Error thresholds
    http_req_failed: ['rate<0.05'], // Less than 5% errors
  },
};

// Sample banner bid request
function generateBannerRequest() {
  return {
    id: `auction-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    imp: [
      {
        id: 'imp-1',
        banner: {
          w: 300,
          h: 250,
          pos: 1,
          format: [
            { w: 300, h: 250 },
            { w: 320, h: 50 },
          ],
        },
        bidfloor: 0.5,
        bidfloorcur: 'USD',
      },
    ],
    site: {
      domain: 'example.com',
      page: 'https://example.com/article/12345',
      cat: ['IAB1', 'IAB1-1'],
      publisher: {
        id: 'pub-12345',
      },
    },
    device: {
      ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
      ip: `192.168.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
      devicetype: Math.random() > 0.6 ? 4 : 2, // 40% mobile, 60% desktop
      geo: {
        country: ['US', 'GB', 'DE', 'FR', 'CA'][Math.floor(Math.random() * 5)],
      },
    },
    user: {
      id: `user-${Math.random().toString(36).substr(2, 9)}`,
    },
    at: 1,
    tmax: 500,
    cur: ['USD'],
  };
}

// Sample video bid request
function generateVideoRequest() {
  return {
    id: `auction-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    imp: [
      {
        id: 'imp-1',
        video: {
          mimes: ['video/mp4', 'video/webm'],
          minduration: 5,
          maxduration: 30,
          w: 640,
          h: 480,
          protocols: [2, 3, 5, 6],
          linearity: 1,
        },
        bidfloor: 2.0,
        bidfloorcur: 'USD',
      },
    ],
    site: {
      domain: 'video-example.com',
      page: 'https://video-example.com/watch/12345',
      cat: ['IAB1'],
      publisher: {
        id: 'pub-video-123',
      },
    },
    device: {
      ua: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
      ip: `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
      devicetype: 2,
      geo: {
        country: 'US',
      },
    },
    user: {
      id: `user-${Math.random().toString(36).substr(2, 9)}`,
    },
    at: 1,
    tmax: 1000,
    cur: ['USD'],
  };
}

// Main test function
export default function () {
  // 80% banner, 20% video
  const request = Math.random() > 0.2 ? generateBannerRequest() : generateVideoRequest();

  const headers = {
    'Content-Type': 'application/json',
  };

  if (API_KEY) {
    headers['X-API-Key'] = API_KEY;
  }

  const startTime = Date.now();

  const response = http.post(`${PBS_URL}/openrtb2/auction`, JSON.stringify(request), {
    headers: headers,
    tags: { name: 'auction' },
  });

  const duration = Date.now() - startTime;
  bidResponseTime.add(duration);

  // Check response
  const success = check(response, {
    'status is 200 or 204': (r) => r.status === 200 || r.status === 204,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'valid JSON response': (r) => {
      if (r.status === 204) return true;
      try {
        JSON.parse(r.body);
        return true;
      } catch (e) {
        return false;
      }
    },
  });

  if (success) {
    auctionSuccess.add(1);

    if (response.status === 200) {
      try {
        const body = JSON.parse(response.body);
        if (body.seatbid && body.seatbid.length > 0) {
          let bidCount = 0;
          body.seatbid.forEach((sb) => {
            bidCount += sb.bid ? sb.bid.length : 0;
          });
          bidsReceived.add(bidCount);
        } else {
          noBids.add(1);
        }
      } catch (e) {
        // Ignore parse errors for metrics
      }
    } else if (response.status === 204) {
      noBids.add(1);
    }
  } else {
    auctionSuccess.add(0);
    auctionErrors.add(1);
  }

  // Small sleep to prevent overwhelming
  sleep(Math.random() * 0.1);
}

// Setup function - runs once before test
export function setup() {
  // Health check
  const healthResponse = http.get(`${PBS_URL}/health`);

  if (healthResponse.status !== 200) {
    throw new Error(`PBS health check failed: ${healthResponse.status}`);
  }

  console.log(`PBS server is healthy at ${PBS_URL}`);

  // Get available bidders
  const biddersResponse = http.get(`${PBS_URL}/info/bidders`);
  if (biddersResponse.status === 200) {
    const bidders = JSON.parse(biddersResponse.body);
    console.log(`Available bidders: ${JSON.stringify(bidders)}`);
  }

  return { startTime: Date.now() };
}

// Teardown function - runs once after test
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`Test completed in ${duration.toFixed(2)} seconds`);
}

// Handle test summary
export function handleSummary(data) {
  return {
    'tests/load/summary.json': JSON.stringify(data, null, 2),
    stdout: textSummary(data, { indent: '  ', enableColors: true }),
  };
}

// Text summary helper
function textSummary(data, options) {
  const metrics = data.metrics;
  let output = '\n========== LOAD TEST SUMMARY ==========\n\n';

  output += `Total Requests: ${metrics.http_reqs.values.count}\n`;
  output += `Success Rate: ${(metrics.auction_success_rate?.values?.rate * 100 || 0).toFixed(2)}%\n`;
  output += `Bids Received: ${metrics.bids_received?.values?.count || 0}\n`;
  output += `No Bids: ${metrics.no_bids?.values?.count || 0}\n`;
  output += `Errors: ${metrics.auction_errors?.values?.count || 0}\n\n`;

  output += `Response Times:\n`;
  output += `  p50: ${metrics.http_req_duration?.values?.['p(50)']?.toFixed(2) || 0}ms\n`;
  output += `  p95: ${metrics.http_req_duration?.values?.['p(95)']?.toFixed(2) || 0}ms\n`;
  output += `  p99: ${metrics.http_req_duration?.values?.['p(99)']?.toFixed(2) || 0}ms\n`;
  output += `  max: ${metrics.http_req_duration?.values?.max?.toFixed(2) || 0}ms\n\n`;

  output += `Thresholds:\n`;
  for (const [name, threshold] of Object.entries(data.thresholds || {})) {
    const status = threshold.ok ? '✓' : '✗';
    output += `  ${status} ${name}\n`;
  }

  output += '\n========================================\n';

  return output;
}
