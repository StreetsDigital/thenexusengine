# On-Call Runbook

## Quick Reference

### Critical Endpoints
| Service | Health Endpoint | Port |
|---------|----------------|------|
| PBS | `/health` | 8080 |
| IDR | `/health` | 8000 |
| TimescaleDB | TCP check | 5432 |
| Redis | `PING` | 6379 |

### Key Metrics to Monitor
- **Request Latency**: P99 < 100ms for auction, P99 < 50ms for IDR
- **Error Rate**: < 1% 5xx responses
- **Auction Success Rate**: > 95%
- **Bidder Response Rate**: > 90% per bidder

---

## Common Issues and Resolutions

### 1. High Latency Alerts

**Symptoms**: P99 latency > 100ms, user reports of slow page loads

**Diagnosis**:
```bash
# Check PBS latency breakdown
curl -s localhost:8080/metrics | grep auction_latency

# Check IDR latency
curl -s localhost:8000/metrics | grep processing_time

# Check bidder-specific latency
curl -s localhost:8080/metrics | grep bidder_latency
```

**Common Causes & Fixes**:
1. **Slow bidder**: Check individual bidder latencies. Consider reducing timeout or disabling slow bidder.
2. **IDR overload**: Check IDR CPU/memory. Scale up if needed.
3. **Database slow queries**: Check TimescaleDB slow query log.
4. **Network issues**: Check cross-service latency.

### 2. High Error Rate

**Symptoms**: 5xx errors > 1%, Grafana error rate alert

**Diagnosis**:
```bash
# Check recent errors
tail -100 /var/log/pbs/error.log | jq .

# Check IDR errors
tail -100 /var/log/idr/error.log

# Check database connectivity
psql -h localhost -U postgres -c "SELECT 1"
```

**Common Causes & Fixes**:
1. **Database connection pool exhausted**: Increase pool size or check for leaks
2. **IDR unavailable**: Check circuit breaker status. PBS will fail open.
3. **Memory pressure**: Check OOM killer, scale resources

### 3. Bidder Not Responding

**Symptoms**: Specific bidder shows 0% response rate

**Diagnosis**:
```bash
# Check bidder-specific metrics
curl -s localhost:8080/metrics | grep -E "bidder_.*appnexus"

# Test bidder endpoint directly
curl -v https://ib.adnxs.com/openrtb2/prebid
```

**Resolution**:
1. Check bidder status page
2. Contact bidder partner if persistent
3. Temporarily disable bidder in config if blocking

### 4. Memory Issues

**Symptoms**: OOM kills, high memory usage alerts

**Diagnosis**:
```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head -10

# Check Go heap
curl -s localhost:8080/debug/pprof/heap > heap.out
go tool pprof heap.out
```

**Resolution**:
1. Restart affected service
2. Check for goroutine leaks
3. Scale horizontally if persistent

### 5. IDR Circuit Breaker Open

**Symptoms**: All bidders being used (bypass mode active)

**Diagnosis**:
```bash
# Check circuit breaker status
curl localhost:8080/admin/idr/status

# Check IDR health
curl localhost:8000/health
```

**Resolution**:
1. Check IDR logs for errors
2. Restart IDR if unhealthy
3. Circuit breaker will auto-recover when IDR is healthy

---

## Escalation Procedures

### Severity Levels

| Level | Criteria | Response Time | Escalation |
|-------|----------|---------------|------------|
| SEV1 | Complete outage, revenue impacting | 15 min | Page engineering lead |
| SEV2 | Degraded service, partial impact | 1 hour | Slack #alerts |
| SEV3 | Minor issues, no revenue impact | 4 hours | Ticket |

### Contact Information
- Engineering Lead: [Contact Info]
- Platform Team: #platform-support
- Database Team: #database-support

---

## Emergency Procedures

### Complete Service Restart
```bash
# Graceful restart PBS
fly machine restart --app pbs-prod

# Restart IDR
fly machine restart --app idr-prod
```

### Rollback Deployment
See: [Rollback Procedures](./rollback.md)

### Disable Problematic Bidder
```bash
# Via admin API
curl -X POST localhost:8080/admin/bidders/appnexus/disable
```

### Force IDR Bypass Mode
```bash
curl -X POST localhost:8000/api/mode/bypass -d '{"enabled": true}'
```

---

## Post-Incident

1. Document timeline in incident ticket
2. Gather relevant logs and metrics
3. Schedule post-mortem within 48 hours
4. Update this runbook if new scenario
