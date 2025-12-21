# Alerting Configuration

## Alert Thresholds

### Critical Alerts (Page Immediately)

| Alert | Metric | Threshold | Duration | Action |
|-------|--------|-----------|----------|--------|
| Service Down | up{job="pbs"} | == 0 | 1 min | Page on-call |
| High Error Rate | http_errors_total / http_requests_total | > 5% | 5 min | Page on-call |
| Auction Failure Rate | auction_failures / auction_total | > 10% | 5 min | Page on-call |

### Warning Alerts (Notify Slack)

| Alert | Metric | Threshold | Duration | Action |
|-------|--------|-----------|----------|--------|
| High Latency | histogram_quantile(0.99, auction_latency) | > 100ms | 5 min | #alerts |
| IDR Bypass Active | idr_bypass_enabled | == 1 | 1 min | #alerts |
| Memory High | container_memory_usage | > 80% | 10 min | #alerts |
| Bidder Timeout Rate | bidder_timeouts / bidder_requests | > 20% | 5 min | #alerts |

### Info Alerts (Low Priority)

| Alert | Metric | Threshold | Duration | Action |
|-------|--------|-----------|----------|--------|
| Low Bid Rate | winning_bids / auction_total | < 50% | 30 min | Dashboard |
| Cache Miss High | redis_cache_miss_rate | > 30% | 15 min | Dashboard |

---

## Prometheus Alert Rules

```yaml
# /etc/prometheus/alerts/pbs.yml
groups:
  - name: pbs-critical
    rules:
      - alert: PBSDown
        expr: up{job="pbs"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "PBS service is down"
          description: "PBS has been unreachable for > 1 minute"

      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m]))
          / sum(rate(http_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"

      - alert: AuctionFailureRate
        expr: |
          sum(rate(auction_failures_total[5m]))
          / sum(rate(auction_requests_total[5m])) > 0.10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High auction failure rate"
          description: "{{ $value | humanizePercentage }} of auctions failing"

  - name: pbs-warning
    rules:
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(auction_latency_bucket[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency is high"
          description: "P99 latency is {{ $value | humanizeDuration }}"

      - alert: IDRBypassActive
        expr: idr_bypass_enabled == 1
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "IDR bypass mode is active"
          description: "IDR is being bypassed - all bidders receiving requests"

      - alert: HighMemoryUsage
        expr: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.8
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Container using {{ $value | humanizePercentage }} of memory limit"

      - alert: BidderHighTimeout
        expr: |
          sum(rate(bidder_timeout_total[5m])) by (bidder)
          / sum(rate(bidder_requests_total[5m])) by (bidder) > 0.2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High timeout rate for {{ $labels.bidder }}"
          description: "{{ $value | humanizePercentage }} requests timing out"
```

---

## PagerDuty Integration

```yaml
# Alertmanager config
receivers:
  - name: pagerduty-critical
    pagerduty_configs:
      - service_key: ${PAGERDUTY_KEY}
        severity: critical

  - name: slack-warnings
    slack_configs:
      - api_url: ${SLACK_WEBHOOK_URL}
        channel: '#platform-alerts'
        title: '{{ .CommonAnnotations.summary }}'
        text: '{{ .CommonAnnotations.description }}'

route:
  receiver: slack-warnings
  routes:
    - match:
        severity: critical
      receiver: pagerduty-critical
    - match:
        severity: warning
      receiver: slack-warnings
```

---

## Grafana Dashboard Panels

### Overview Dashboard

1. **Request Rate** - `sum(rate(http_requests_total[1m]))`
2. **Error Rate** - `sum(rate(http_requests_total{status=~"5.."}[1m]))`
3. **P50/P95/P99 Latency** - histogram quantiles
4. **Auction Success Rate** - `winning_bids / total_auctions`
5. **Active Bidders** - count of bidders with recent requests

### Bidder Dashboard

1. **Per-Bidder Request Rate**
2. **Per-Bidder Response Rate**
3. **Per-Bidder Latency**
4. **Per-Bidder Error Rate**
5. **Per-Bidder Bid Rate**

### IDR Dashboard

1. **IDR Request Rate**
2. **IDR Latency**
3. **Circuit Breaker Status**
4. **Bypass Mode Status**
5. **Selected Bidder Distribution**

### Infrastructure Dashboard

1. **CPU Usage per Service**
2. **Memory Usage per Service**
3. **Network I/O**
4. **Database Connections**
5. **Redis Memory/Connections**

---

## Escalation Matrix

| Severity | Primary | Escalate After | Secondary |
|----------|---------|----------------|-----------|
| Critical | On-Call Engineer | 15 min | Engineering Lead |
| Warning | On-Call Engineer | 1 hour | Platform Team |
| Info | N/A | N/A | N/A |
