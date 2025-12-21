# Rollback Procedures

## Quick Rollback Commands

### PBS Rollback
```bash
# List recent deployments
fly releases --app pbs-prod

# Rollback to previous version
fly deploy --app pbs-prod --image registry.fly.io/pbs-prod:v1.2.3

# Or rollback to specific release number
fly releases rollback --app pbs-prod 5
```

### IDR Rollback
```bash
# List recent deployments
fly releases --app idr-prod

# Rollback to previous version
fly deploy --app idr-prod --image registry.fly.io/idr-prod:v1.2.3
```

---

## Pre-Rollback Checklist

- [ ] Confirm the issue is caused by the new deployment
- [ ] Document current error rates and symptoms
- [ ] Notify team in #platform-alerts
- [ ] Identify the last known good version
- [ ] Prepare to monitor after rollback

---

## Rollback Scenarios

### Scenario 1: Application Error After Deploy

**Symptoms**: 5xx errors spike after deployment

**Steps**:
1. Check deployment timestamp vs error spike
2. Review recent commits for potential cause
3. Execute rollback command above
4. Monitor error rate for 5 minutes
5. If stable, investigate root cause

### Scenario 2: Performance Degradation

**Symptoms**: Latency increase after deployment

**Steps**:
1. Compare latency metrics before/after deploy
2. Check for new slow code paths
3. Execute rollback
4. Profile code changes offline

### Scenario 3: Configuration Error

**Symptoms**: Startup failures, missing config

**Steps**:
1. Check Fly secrets and environment
2. Compare config between versions
3. Fix config or rollback as needed

```bash
# View current secrets
fly secrets list --app pbs-prod

# Restore a secret
fly secrets set API_KEY=value --app pbs-prod
```

---

## Database Rollback

### TimescaleDB

**Note**: Database rollbacks are complex. Prefer forward-fixing.

For schema changes:
```bash
# Check migration history
psql -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5"

# Rollback last migration (if supported)
migrate -path ./migrations -database $DATABASE_URL down 1
```

### Redis

Redis is ephemeral cache. No rollback needed - restart clears cache.

```bash
# Clear specific keys if needed
redis-cli KEYS "prebid:*" | xargs redis-cli DEL
```

---

## Feature Flag Rollback

If using feature flags:

```bash
# Disable problematic feature via IDR admin
curl -X POST localhost:8000/api/mode/bypass -d '{"enabled": true}'

# Or via config
fly secrets set IDR_BYPASS_ENABLED=true --app pbs-prod
```

---

## Verification Steps

After any rollback:

1. **Health Check**
   ```bash
   curl https://pbs.yoursite.com/health
   curl https://idr.yoursite.com/health
   ```

2. **Smoke Test**
   ```bash
   # Send test auction request
   curl -X POST https://pbs.yoursite.com/openrtb2/auction \
     -H "Content-Type: application/json" \
     -d @test/fixtures/minimal_request.json
   ```

3. **Monitor Metrics**
   - Error rate returns to baseline
   - Latency returns to baseline
   - Auction success rate stable

4. **Notify Team**
   - Post in #platform-alerts
   - Create incident ticket if not exists

---

## Rollback Contacts

| Role | Contact | When to Contact |
|------|---------|-----------------|
| On-Call Engineer | PagerDuty | All rollbacks |
| Platform Lead | Slack DM | SEV1/SEV2 |
| Database Admin | #db-support | DB rollbacks |

---

## Post-Rollback

1. Document rollback in incident ticket
2. Identify root cause in reverted code
3. Create fix PR with proper testing
4. Schedule re-deployment with monitoring
