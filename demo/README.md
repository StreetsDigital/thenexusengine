# Prebid.js Demo

This demo shows how to integrate a website with The Nexus Engine PBS (Prebid Server).

## Quick Start

### Option 1: Open directly in browser
```bash
open demo/index.html
```

### Option 2: Serve locally (recommended for testing)
```bash
# Python 3
cd demo && python3 -m http.server 8080

# Then open: http://localhost:8080
```

### Option 3: Deploy to any static hosting
Upload `index.html` to:
- GitHub Pages
- Netlify
- Vercel
- Any web server

## What the Demo Shows

1. **Prebid.js Loading** - Loads the latest Prebid.js from CDN
2. **S2S Configuration** - Points to `nexus-pbs.fly.dev`
3. **Ad Units** - Two sample placements (300x250 and 320x50)
4. **Bid Requests** - Sends requests to PBS for appnexus, rubicon, pubmatic
5. **Debug Panel** - Shows real-time logs of the bidding process

## Expected Behavior

In test mode, you may see "No bids received" - this is normal because:
- Test placement IDs don't return real bids
- Bidders need real accounts/credentials
- Cookie sync may not be complete yet

## Configuration

Edit `index.html` to customize:

```javascript
pbjs.setConfig({
    s2sConfig: {
        accountId: 'YOUR_PUBLISHER_ID',  // Change this
        bidders: ['appnexus', 'rubicon', 'pubmatic'],
        endpoint: {
            p1Consent: 'https://nexus-pbs.fly.dev/openrtb2/auction',
            noP1Consent: 'https://nexus-pbs.fly.dev/openrtb2/auction'
        }
    }
});
```

## Debug Mode

Add `?pbjs_debug=true` to the URL for verbose Prebid.js logging in the browser console.

## Production Checklist

- [ ] Replace test placement IDs with real ones from your bidder accounts
- [ ] Update `accountId` to your publisher ID
- [ ] Configure GDPR/TCF consent if applicable
- [ ] Set up Google Ad Manager (GAM) for fallback
- [ ] Test on multiple devices/browsers
