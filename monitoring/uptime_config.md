# Monitoring Setup Guide - The Nexus Engine

## Option 1: UptimeRobot (Free - Recommended for Basic Monitoring)

Create monitors at https://uptimerobot.com:

### PBS Health Check
- Monitor Type: HTTP(s)
- URL: `https://nexus-pbs.fly.dev/health`
- Monitoring Interval: 5 minutes
- Alert Contacts: Your email/Slack

### IDR Health Check  
- Monitor Type: HTTP(s)
- URL: `https://nexus-idr.fly.dev/health`
- Monitoring Interval: 5 minutes
- Alert Contacts: Your email/Slack

### Auction Endpoint (Keyword Monitor)
- Monitor Type: HTTP(s) - Keyword
- URL: `https://nexus-pbs.fly.dev/health`
- Keyword: `healthy`
- Monitoring Interval: 5 minutes

---

## Option 2: Better Uptime (Better UI, Free Tier Available)

https://betteruptime.com - More features, status pages

---

## Option 3: Fly.io Grafana (Built-in Metrics)

Access at: https://fly-metrics.net

Dashboards for:
- Request rate
- Response times (p50, p95, p99)
- Error rates
- CPU/Memory usage
- Network I/O

---

## Option 4: Slack Webhook Alerts

1. Create Slack Incoming Webhook:
   - Go to: https://api.slack.com/apps
   - Create App → Incoming Webhooks → Add to channel

2. Set environment variable:
   ```bash
   export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/xxx/yyy/zzz"
   ```

3. Run monitor with alerts:
   ```bash
   ./monitoring/health_monitor.sh
   ```

---

## Option 5: Cron-based Monitoring

Add to crontab (`crontab -e`):

```cron
# Health check every 5 minutes
*/5 * * * * /path/to/monitoring/health_monitor.sh >> /var/log/nexus-monitor.log 2>&1

# Daily summary at 9 AM
0 9 * * * /path/to/monitoring/health_monitor.sh | mail -s "Nexus Daily Health" your@email.com
```

---

## Critical Alerts to Configure

| Alert | Condition | Severity |
|-------|-----------|----------|
| Service Down | HTTP != 200 | Critical |
| High Latency | p95 > 500ms | Warning |
| Very High Latency | p95 > 1000ms | Critical |
| Error Rate | > 1% errors | Warning |
| Error Rate | > 5% errors | Critical |
| Memory | > 80% used | Warning |
| Memory | > 95% used | Critical |

---

## Fly.io Native Monitoring

View metrics in real-time:
```bash
# Live logs
flyctl logs -a nexus-pbs

# Machine status
flyctl status -a nexus-pbs

# Metrics dashboard
flyctl dashboard -a nexus-pbs
```
