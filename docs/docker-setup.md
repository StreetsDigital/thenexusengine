# Docker Setup Guide

This guide explains how to run The Nexus Engine using Docker.

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 4GB RAM minimum (8GB recommended)

## Quick Start

```bash
# Clone the repository
git clone https://github.com/StreetsDigital/thenexusengine.git
cd thenexusengine

# Start all services
docker compose up -d

# View logs
docker compose logs -f
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| PBS Server | 8000 | Go-based Prebid Server |
| IDR Service | 5050 | Python IDR + Admin Dashboard |
| Redis | 6379 | Real-time metrics storage |
| TimescaleDB | 5432 | Historical data storage |

## Accessing Services

- **PBS Auction Endpoint**: http://localhost:8000/openrtb2/auction
- **IDR Admin Dashboard**: http://localhost:5050
- **Health Checks**: http://localhost:8000/status, http://localhost:5050/health

## Environment Variables

Create a `.env` file in the project root to customize settings:

```bash
# .env file

# Ports
PBS_PORT=8000
IDR_PORT=5050
REDIS_PORT=6379
TIMESCALE_PORT=5432

# IDR Settings
IDR_ENABLED=true
IDR_TIMEOUT_MS=50

# Event Recording
EVENT_RECORD_ENABLED=true
EVENT_BUFFER_SIZE=100

# Database
POSTGRES_PASSWORD=nexusengine
USE_MOCK_DB=false
```

## Configuration Options

### IDR Service Configuration

The IDR service can be configured via:
1. **Admin Dashboard UI** (http://localhost:5050)
2. **Config file** (`config/idr_config.yaml`)
3. **Environment variables**

#### Via Admin Dashboard (Recommended)

Navigate to http://localhost:5050 and use the UI to configure:
- Operating mode (Normal/Shadow/Bypass)
- Max bidders per auction
- Score threshold
- Exploration settings
- Scoring weights

#### Via Config File

Edit `config/idr_config.yaml`:

```yaml
selector:
  bypass_enabled: false
  shadow_mode: false
  max_bidders: 15
  min_score_threshold: 25
  exploration_rate: 0.1
  exploration_slots: 2

scoring:
  weights:
    win_rate: 0.25
    bid_rate: 0.20
    cpm: 0.15
    floor_clearance: 0.15
    latency: 0.10
    recency: 0.10
    id_match: 0.05

database:
  redis_url: redis://redis:6379
  timescale_url: postgresql://postgres:nexusengine@timescaledb:5432/idr
```

## Development Mode

For development with hot-reloading:

```bash
# Start infrastructure only
docker compose up -d redis timescaledb

# Run Python IDR locally
pip install -e ".[admin,db]"
export REDIS_URL=redis://localhost:6379
export TIMESCALE_URL=postgresql://postgres:nexusengine@localhost:5432/idr
python run_admin.py

# Run Go PBS locally (in another terminal)
cd pbs
go run ./cmd/server --idr-url http://localhost:5050
```

## Production Deployment

For production deployments:

1. **Set secure passwords**:
   ```bash
   export POSTGRES_PASSWORD=$(openssl rand -base64 32)
   ```

2. **Enable TLS** (configure via reverse proxy like nginx/traefik)

3. **Resource limits** - Add to docker-compose.yml:
   ```yaml
   services:
     pbs:
       deploy:
         resources:
           limits:
             cpus: '2'
             memory: 2G
     idr:
       deploy:
         resources:
           limits:
             cpus: '1'
             memory: 1G
   ```

4. **Monitoring** - Add Prometheus metrics endpoint:
   - PBS: http://localhost:8000/metrics
   - IDR: http://localhost:5050/metrics

## Data Persistence

Data is persisted in Docker volumes:
- `redis-data`: Real-time metrics (last hour)
- `timescale-data`: Historical analytics

### Backup

```bash
# Backup TimescaleDB
docker compose exec timescaledb pg_dump -U postgres idr > backup.sql

# Backup Redis
docker compose exec redis redis-cli BGSAVE
docker cp $(docker compose ps -q redis):/data/dump.rdb ./redis-backup.rdb
```

### Restore

```bash
# Restore TimescaleDB
docker compose exec -T timescaledb psql -U postgres idr < backup.sql

# Restore Redis
docker cp ./redis-backup.rdb $(docker compose ps -q redis):/data/dump.rdb
docker compose restart redis
```

## Troubleshooting

### Services won't start

```bash
# Check logs
docker compose logs pbs
docker compose logs idr

# Rebuild images
docker compose build --no-cache

# Reset everything
docker compose down -v
docker compose up -d
```

### Database connection issues

```bash
# Check TimescaleDB is ready
docker compose exec timescaledb pg_isready -U postgres -d idr

# Check Redis is ready
docker compose exec redis redis-cli ping
```

### Performance issues

```bash
# Monitor resource usage
docker stats

# Scale horizontally (if using swarm/k8s)
docker compose up -d --scale pbs=3
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Docker Network                           │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐       │
│  │     PBS      │───▶│     IDR      │───▶│    Redis     │       │
│  │  (Go:8000)   │    │ (Python:5050)│    │   (:6379)    │       │
│  └──────────────┘    └──────────────┘    └──────────────┘       │
│         │                   │                                    │
│         │                   │            ┌──────────────┐       │
│         │                   └───────────▶│ TimescaleDB  │       │
│         │                                │   (:5432)    │       │
│         │                                └──────────────┘       │
│         │                                                        │
│         ▼                                                        │
│  ┌──────────────┐                                               │
│  │   Demand     │                                               │
│  │   Partners   │                                               │
│  └──────────────┘                                               │
└─────────────────────────────────────────────────────────────────┘
```

## Next Steps

1. Access the Admin Dashboard: http://localhost:5050
2. Configure bidder settings via the UI
3. Send test auction requests to http://localhost:8000/openrtb2/auction
4. Monitor performance in the Bidder Metrics panel

## Fly.io Production Deployment

For production deployment, we recommend using Fly.io instead of Docker Compose:

```bash
# Install Fly CLI
curl -L https://fly.io/install.sh | sh

# Login and deploy
fly auth login
fly deploy

# View logs
fly logs

# Scale for traffic
fly scale count 3
```

The project includes `fly.toml` pre-configured for:
- Auto-scaling based on load
- Health check monitoring
- Redis sampling (10% to reduce Upstash costs)
- Optimized for 40M+ requests/month

See the main [README.md](../README.md) for more deployment options.

## Supported Bidders

The Nexus Engine supports **22 bidder adapters** across categories:

- **Premium SSPs**: AppNexus, Rubicon, PubMatic, OpenX, Index Exchange
- **Mid-tier**: TripleLift, Sovrn, Sharethrough, GumGum, 33Across, Criteo
- **Video Specialists**: SpotX, Beachfront, Unruly
- **Native Specialists**: Teads, Outbrain, Taboola
- **Regional (EMEA)**: Adform, Smart AdServer, Improve Digital
- **Additional**: Media.net, Conversant

All adapters include GVL IDs for GDPR/TCF compliance.
