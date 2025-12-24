# The Nexus Engine - Production Deployment Guide

This guide covers deploying The Nexus Engine to Fly.io for production use.

## Prerequisites

1. **Fly.io CLI** installed: https://fly.io/docs/flyctl/install/
2. **Fly.io account** created and logged in: `fly auth login`
3. **Upstash account** for Redis (free tier available): https://upstash.com

## Architecture Overview

```
                     ┌─────────────────┐
                     │   Fly.io Edge   │
                     └────────┬────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼                               ▼
    ┌─────────────────┐             ┌─────────────────┐
    │    PBS Server   │────────────▶│   IDR Service   │
    │   (Go, :8000)   │   internal  │ (Python, :5050) │
    └─────────────────┘             └────────┬────────┘
                                             │
                            ┌────────────────┼────────────────┐
                            ▼                                 ▼
                   ┌─────────────────┐              ┌─────────────────┐
                   │  Upstash Redis  │              │  Fly Postgres   │
                   │   (external)    │              │  (TimescaleDB)  │
                   └─────────────────┘              └─────────────────┘
```

## Quick Start

```bash
# 1. Setup (creates apps, databases, secrets)
./fly/deploy.sh setup

# 2. Deploy services
./fly/deploy.sh deploy

# 3. Check status
./fly/deploy.sh status
```

## Detailed Setup Steps

### Step 1: Create Fly.io Apps

```bash
fly apps create nexus-pbs --org personal
fly apps create nexus-idr --org personal
```

### Step 2: Create Fly Postgres Database

```bash
fly postgres create \
    --name nexus-db \
    --region iad \
    --initial-cluster-size 1 \
    --vm-size shared-cpu-1x \
    --volume-size 1

# Attach to IDR service
fly postgres attach nexus-db -a nexus-idr --database-name idr
```

### Step 3: Setup Redis (Upstash)

1. Go to https://upstash.com and create a Redis database
2. Select the same region as your Fly.io apps (e.g., US East)
3. Copy the Redis URL (format: `redis://default:xxx@xxx.upstash.io:6379`)

```bash
fly secrets set REDIS_URL="redis://default:xxx@xxx.upstash.io:6379" -a nexus-idr
```

### Step 4: Configure Secrets

```bash
# Generate API key for authentication
API_KEY=$(openssl rand -hex 32)
fly secrets set API_KEY_DEFAULT="$API_KEY" -a nexus-pbs
fly secrets set API_KEY_DEFAULT="$API_KEY" -a nexus-idr

# Generate session secret for admin dashboard
SECRET_KEY=$(openssl rand -hex 32)
fly secrets set SECRET_KEY="$SECRET_KEY" -a nexus-idr

# Configure admin users (required for dashboard access)
# Generate bcrypt hash for your password:
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt()).decode())"

# Set admin users (format: username:bcrypt_hash)
fly secrets set ADMIN_USERS="admin:\$2b\$12\$xxxxx" -a nexus-idr
```

### Step 5: Deploy Services

```bash
# Deploy IDR first (PBS depends on it)
fly deploy --config fly/idr.toml --remote-only

# Deploy PBS
fly deploy --config fly/pbs.toml --remote-only
```

## Configuration Reference

### PBS Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PBS_PORT` | `8000` | HTTP port |
| `IDR_URL` | `http://nexus-idr.internal:5050` | IDR service URL |
| `IDR_ENABLED` | `true` | Enable IDR integration |
| `IDR_TIMEOUT_MS` | `50` | IDR request timeout |
| `AUTH_ENABLED` | `true` | Require API key authentication |
| `RATE_LIMIT_RPS` | `5000` | Rate limit per second |
| `PBS_ENFORCE_GDPR` | `true` | Enforce GDPR consent |
| `PBS_ENFORCE_COPPA` | `true` | Block COPPA-flagged requests |
| `PBS_ENFORCE_CCPA` | `true` | Enforce CCPA opt-out |
| `PBS_PRIVACY_STRICT_MODE` | `false` | Reject on invalid consent |

### IDR Service Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IDR_PORT` | `5050` | HTTP port |
| `REDIS_SAMPLE_RATE` | `0.1` | Sampling rate (10%) |
| `AUTH_ENABLED` | `true` | Require API key |
| `SESSION_COOKIE_SECURE` | `true` | HTTPS-only cookies |

### Required Secrets

| Secret | App | Description |
|--------|-----|-------------|
| `API_KEY_DEFAULT` | Both | API authentication key |
| `REDIS_URL` | IDR | Upstash Redis connection URL |
| `DATABASE_URL` | IDR | Fly Postgres (auto-attached) |
| `SECRET_KEY` | IDR | Session encryption key |
| `ADMIN_USERS` | IDR | Admin credentials (bcrypt hashed) |

## Scaling

### For 40M requests/month (~15 RPS average, ~75 RPS peak)

The default configuration handles this with:
- 2 PBS machines (256MB each, auto-scale to 4)
- 2 IDR machines (256MB each, auto-scale to 4)
- 1 Postgres machine (1GB)
- Upstash Redis (free tier: 10K commands/day)

### Scaling Up

```bash
# Increase machine count
fly scale count 4 -a nexus-pbs
fly scale count 4 -a nexus-idr

# Increase memory
fly scale memory 512 -a nexus-pbs
fly scale memory 512 -a nexus-idr
```

## Monitoring

### Health Checks

- PBS: `https://nexus-pbs.fly.dev/health`
- IDR: `https://nexus-idr.fly.dev/health`

### Prometheus Metrics

- PBS: `https://nexus-pbs.fly.dev/metrics`
- IDR: `https://nexus-idr.fly.dev/metrics`

### Logs

```bash
# View PBS logs
fly logs -a nexus-pbs

# View IDR logs
fly logs -a nexus-idr
```

## Troubleshooting

### Common Issues

1. **IDR not connecting to Redis**
   - Check `REDIS_URL` secret is set correctly
   - Verify Upstash Redis is in the same region

2. **PBS not connecting to IDR**
   - IDR must be deployed first
   - Check internal DNS: `http://nexus-idr.internal:5050`

3. **Admin dashboard inaccessible**
   - Ensure `ADMIN_USERS` and `SECRET_KEY` are set
   - Format: `username:bcrypt_hash`

4. **Authentication failures**
   - Verify `API_KEY_DEFAULT` is set on both apps
   - Include header: `X-API-Key: your-key`

### Checking Secrets

```bash
# List secrets (names only, not values)
fly secrets list -a nexus-pbs
fly secrets list -a nexus-idr
```

## Cost Estimate (Fly.io)

For 40M requests/month:

| Resource | Count | Monthly Cost |
|----------|-------|--------------|
| PBS machines (shared-cpu-1x, 256MB) | 2 | ~$3 |
| IDR machines (shared-cpu-1x, 256MB) | 2 | ~$3 |
| Postgres (1GB) | 1 | ~$5 |
| Upstash Redis (free tier) | 1 | $0 |
| **Total** | | **~$11/month** |

## Security Checklist

- [ ] API key authentication enabled (`AUTH_ENABLED=true`)
- [ ] HTTPS enforced (`force_https = true` in fly.toml)
- [ ] Admin users configured with bcrypt passwords
- [ ] Session cookies set to secure (`SESSION_COOKIE_SECURE=true`)
- [ ] Privacy enforcement enabled (GDPR, COPPA, CCPA)
- [ ] Rate limiting configured
- [ ] Secrets stored securely (not in environment)
