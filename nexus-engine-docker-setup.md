# The Nexus Engine: Docker Deployment Guide

**Version:** 1.0  
**Author:** The Nexus Engine  
**Date:** December 2024  
**Status:** Draft

---

## Overview

This guide covers deploying The Nexus Engine PBS (Prebid Server) infrastructure using Docker and Docker Compose. The setup includes:

- Prebid Server (Go)
- Intelligent Demand Router (IDR)
- Redis (real-time cache)
- TimescaleDB (historical analytics)
- Nginx (reverse proxy / load balancer)
- Prometheus + Grafana (monitoring)

---

## Architecture

```
                                    ┌─────────────────────────────────────────────┐
                                    │              DOCKER NETWORK                 │
                                    │            (nexus-network)                  │
┌──────────────┐                    │                                             │
│   Internet   │                    │  ┌─────────────────────────────────────┐   │
│   Traffic    │────────────────────┼─▶│            NGINX                    │   │
└──────────────┘                    │  │      (nexus-nginx:80/443)           │   │
                                    │  └──────────────┬──────────────────────┘   │
                                    │                 │                           │
                                    │    ┌────────────┴────────────┐              │
                                    │    ▼                         ▼              │
                                    │  ┌──────────────┐  ┌──────────────┐        │
                                    │  │  PBS Node 1  │  │  PBS Node 2  │        │
                                    │  │    :8000     │  │    :8000     │        │
                                    │  └──────┬───────┘  └──────┬───────┘        │
                                    │         │                 │                 │
                                    │         └────────┬────────┘                 │
                                    │                  ▼                          │
                                    │         ┌──────────────┐                    │
                                    │         │     IDR      │                    │
                                    │         │    :8001     │                    │
                                    │         └──────┬───────┘                    │
                                    │                │                            │
                                    │    ┌───────────┴───────────┐                │
                                    │    ▼                       ▼                │
                                    │  ┌──────────────┐  ┌──────────────┐        │
                                    │  │    Redis     │  │ TimescaleDB  │        │
                                    │  │    :6379     │  │    :5432     │        │
                                    │  └──────────────┘  └──────────────┘        │
                                    │                                             │
                                    │  ┌──────────────┐  ┌──────────────┐        │
                                    │  │  Prometheus  │  │   Grafana    │        │
                                    │  │    :9090     │  │    :3000     │        │
                                    │  └──────────────┘  └──────────────┘        │
                                    └─────────────────────────────────────────────┘
```

---

## Directory Structure

```
nexus-engine/
├── docker-compose.yml
├── docker-compose.override.yml      # Local dev overrides
├── docker-compose.prod.yml          # Production overrides
├── .env                             # Environment variables
├── .env.example                     # Example env file
│
├── pbs/
│   ├── Dockerfile
│   ├── config/
│   │   ├── pbs.yaml                 # Main PBS config
│   │   ├── bidder-info/             # Bidder configurations
│   │   └── accounts/                # Publisher account configs
│   └── modules/
│       └── nexus-idr/               # IDR module source
│
├── idr/
│   ├── Dockerfile
│   ├── src/
│   │   ├── main.py
│   │   ├── classifier.py
│   │   ├── scorer.py
│   │   ├── selector.py
│   │   └── db.py
│   ├── requirements.txt
│   └── config/
│       └── idr.yaml
│
├── nginx/
│   ├── nginx.conf
│   ├── ssl/                         # SSL certificates
│   └── conf.d/
│       └── default.conf
│
├── redis/
│   └── redis.conf
│
├── timescaledb/
│   └── init/
│       └── 001-init.sql             # Schema initialization
│
├── prometheus/
│   └── prometheus.yml
│
├── grafana/
│   ├── provisioning/
│   │   ├── dashboards/
│   │   └── datasources/
│   └── dashboards/
│       └── nexus-overview.json
│
└── scripts/
    ├── setup.sh                     # Initial setup
    ├── deploy.sh                    # Deployment script
    └── backup.sh                    # Database backup
```

---

## Docker Compose Configuration

### Main docker-compose.yml

```yaml
version: '3.8'

services:
  # ===========================================
  # PREBID SERVER - Core auction engine
  # ===========================================
  pbs-1:
    build:
      context: ./pbs
      dockerfile: Dockerfile
    container_name: nexus-pbs-1
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      - PBS_HOST=0.0.0.0
      - PBS_PORT=8000
      - PBS_ADMIN_PORT=6060
      - IDR_ENDPOINT=http://idr:8001
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    volumes:
      - ./pbs/config:/etc/pbs:ro
    depends_on:
      - redis
      - idr
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/status"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G

  pbs-2:
    build:
      context: ./pbs
      dockerfile: Dockerfile
    container_name: nexus-pbs-2
    restart: unless-stopped
    ports:
      - "8002:8000"
    environment:
      - PBS_HOST=0.0.0.0
      - PBS_PORT=8000
      - PBS_ADMIN_PORT=6060
      - IDR_ENDPOINT=http://idr:8001
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    volumes:
      - ./pbs/config:/etc/pbs:ro
    depends_on:
      - redis
      - idr
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/status"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G

  # ===========================================
  # INTELLIGENT DEMAND ROUTER
  # ===========================================
  idr:
    build:
      context: ./idr
      dockerfile: Dockerfile
    container_name: nexus-idr
    restart: unless-stopped
    ports:
      - "8001:8001"
    environment:
      - IDR_HOST=0.0.0.0
      - IDR_PORT=8001
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - TIMESCALE_HOST=timescaledb
      - TIMESCALE_PORT=5432
      - TIMESCALE_DB=${TIMESCALE_DB:-nexus}
      - TIMESCALE_USER=${TIMESCALE_USER:-nexus}
      - TIMESCALE_PASSWORD=${TIMESCALE_PASSWORD}
      - LOG_LEVEL=${LOG_LEVEL:-INFO}
    volumes:
      - ./idr/config:/etc/idr:ro
    depends_on:
      redis:
        condition: service_healthy
      timescaledb:
        condition: service_healthy
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8001/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M

  # ===========================================
  # REDIS - Real-time cache & counters
  # ===========================================
  redis:
    image: redis:7-alpine
    container_name: nexus-redis
    restart: unless-stopped
    ports:
      - "6379:6379"
    command: redis-server /usr/local/etc/redis/redis.conf
    volumes:
      - ./redis/redis.conf:/usr/local/etc/redis/redis.conf:ro
      - redis-data:/data
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 1G
        reservations:
          cpus: '0.25'
          memory: 256M

  # ===========================================
  # TIMESCALEDB - Historical analytics
  # ===========================================
  timescaledb:
    image: timescale/timescaledb:latest-pg15
    container_name: nexus-timescaledb
    restart: unless-stopped
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_DB=${TIMESCALE_DB:-nexus}
      - POSTGRES_USER=${TIMESCALE_USER:-nexus}
      - POSTGRES_PASSWORD=${TIMESCALE_PASSWORD}
    volumes:
      - ./timescaledb/init:/docker-entrypoint-initdb.d:ro
      - timescale-data:/var/lib/postgresql/data
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${TIMESCALE_USER:-nexus} -d ${TIMESCALE_DB:-nexus}"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G

  # ===========================================
  # NGINX - Reverse proxy & load balancer
  # ===========================================
  nginx:
    image: nginx:alpine
    container_name: nexus-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/conf.d:/etc/nginx/conf.d:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - pbs-1
      - pbs-2
    networks:
      - nexus-network
    healthcheck:
      test: ["CMD", "nginx", "-t"]
      interval: 30s
      timeout: 10s
      retries: 3

  # ===========================================
  # PROMETHEUS - Metrics collection
  # ===========================================
  prometheus:
    image: prom/prometheus:latest
    container_name: nexus-prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=30d'
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    networks:
      - nexus-network

  # ===========================================
  # GRAFANA - Dashboards & visualization
  # ===========================================
  grafana:
    image: grafana/grafana:latest
    container_name: nexus-grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=${GRAFANA_USER:-admin}
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
      - ./grafana/dashboards:/var/lib/grafana/dashboards:ro
      - grafana-data:/var/lib/grafana
    depends_on:
      - prometheus
    networks:
      - nexus-network

# ===========================================
# NETWORKS
# ===========================================
networks:
  nexus-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16

# ===========================================
# VOLUMES
# ===========================================
volumes:
  redis-data:
  timescale-data:
  prometheus-data:
  grafana-data:
```

---

## Service Configurations

### PBS Dockerfile

**File:** `pbs/Dockerfile`

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /app

# Clone Prebid Server
RUN git clone https://github.com/prebid/prebid-server.git .

# Copy custom IDR module
COPY modules/nexus-idr ./modules/nexus-idr

# Build with IDR module
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o prebid-server .

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /app/prebid-server .
COPY config/ /etc/pbs/

EXPOSE 8000 6060

CMD ["./prebid-server", "-config", "/etc/pbs/pbs.yaml"]
```

### PBS Configuration

**File:** `pbs/config/pbs.yaml`

```yaml
# The Nexus Engine - Prebid Server Configuration

# Server settings
host_scheduler:
  host: "{{ .Env.PBS_HOST }}"
  port: {{ .Env.PBS_PORT }}
  admin_port: {{ .Env.PBS_ADMIN_PORT }}

# External endpoints
external_url: "https://pbs.nexusengine.io"

# Default timeout settings (ms)
default_timeout_ms: 800
max_timeout_ms: 1500
auction_timeout_ms: 800

# Enable compression
compression:
  response: true
  request: true

# GDPR settings
gdpr:
  enabled: true
  host_vendor_id: 0
  default_value: "1"
  tcf2:
    enabled: true
    purpose_one_treatment:
      enabled: true
      access_allowed: true

# CCPA settings
ccpa:
  enforce: true

# Cookie sync
user_sync:
  external_url: "https://pbs.nexusengine.io"
  coop_sync:
    default: true
  redirect_url: "https://pbs.nexusengine.io/setuid"

# Cache settings (use Redis)
cache:
  scheme: "http"
  host: "{{ .Env.REDIS_HOST }}"
  port: {{ .Env.REDIS_PORT }}
  default_ttl_seconds: 300

# Stored requests (PBS feature for pre-defined configs)
stored_requests:
  filesystem:
    enabled: true
    path: /etc/pbs/stored_requests
  http:
    endpoint: "{{ .Env.IDR_ENDPOINT }}/stored-requests"

# Account defaults
accounts:
  filesystem:
    enabled: true
    path: /etc/pbs/accounts

# Price floors
price_floors:
  enabled: true
  fetcher:
    enabled: true
    url: "{{ .Env.IDR_ENDPOINT }}/floors"
    timeout_ms: 100
    period_sec: 300

# Analytics
analytics:
  pubstack:
    enabled: false
  # Custom Nexus analytics
  nexus:
    enabled: true
    endpoint: "{{ .Env.IDR_ENDPOINT }}/analytics"

# Module hooks (where IDR plugs in)
hooks:
  enabled: true
  host_execution_plan:
    endpoints:
      /openrtb2/auction:
        stages:
          processed-auction-request:
            groups:
              - timeout: 5ms
                module_code: nexus-idr
                hook_impl_code: route-demand

# Metrics
metrics:
  prometheus:
    enabled: true
    port: 9100
    namespace: "nexus_pbs"

# Adapters (all enabled, IDR filters per-request)
adapters:
  appnexus:
    enabled: true
    endpoint: "https://ib.adnxs.com/openrtb2/prebid"
  rubicon:
    enabled: true
    endpoint: "https://prebid-server.rubiconproject.com/openrtb2/auction"
  pubmatic:
    enabled: true
    endpoint: "https://hbopenbid.pubmatic.com/translator"
  openx:
    enabled: true
    endpoint: "https://rtb.openx.net/openrtb/prebid"
  ix:
    enabled: true
    endpoint: "https://htlb.casalemedia.com/openrtb/pbjs"
  triplelift:
    enabled: true
    endpoint: "https://tlx.3lift.com/header/auction"
  sovrn:
    enabled: true
    endpoint: "https://ap.lijit.com/rtb/bid"
  sharethrough:
    enabled: true
    endpoint: "https://btlr.sharethrough.com/WYu2BXv1/v1"
  gumgum:
    enabled: true
    endpoint: "https://g2.gumgum.com/providers/prbds2s/bid"
  33across:
    enabled: true
    endpoint: "https://ssc.33across.com/api/v1/hb"
  criteo:
    enabled: true
    endpoint: "https://bidder.criteo.com/cdb"
  smartadserver:
    enabled: true
    endpoint: "https://ssb-global.smartadserver.com"
  adform:
    enabled: true
    endpoint: "https://adx.adform.net/adx"
  improvedigital:
    enabled: true
    endpoint: "https://ad.360yield.com/pb"
  teads:
    enabled: true
    endpoint: "https://a.teads.tv/hb/bid-request"
  unruly:
    enabled: true
    endpoint: "https://targeting.unrulymedia.com/openrtb/2.2"
  yieldmo:
    enabled: true
    endpoint: "https://ads.yieldmo.com/exchange/prebid-server"
  # Add more as needed...
```

### IDR Dockerfile

**File:** `idr/Dockerfile`

```dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source
COPY src/ ./src/
COPY config/ ./config/

# Non-root user
RUN useradd -m -u 1000 idr && chown -R idr:idr /app
USER idr

EXPOSE 8001

CMD ["python", "-m", "uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8001"]
```

### IDR Requirements

**File:** `idr/requirements.txt`

```
fastapi==0.109.0
uvicorn[standard]==0.27.0
redis==5.0.1
asyncpg==0.29.0
pydantic==2.5.3
pydantic-settings==2.1.0
python-json-logger==2.0.7
prometheus-client==0.19.0
httpx==0.26.0
orjson==3.9.12
```

### IDR Main Application

**File:** `idr/src/main.py`

```python
"""
The Nexus Engine - Intelligent Demand Router
FastAPI application entry point
"""

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from prometheus_client import make_asgi_app
import logging

from .classifier import RequestClassifier
from .scorer import BidderScorer
from .selector import PartnerSelector
from .db import Database
from .models import (
    RouteRequest,
    RouteResponse,
    HealthResponse,
    AnalyticsEvent
)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Initialize app
app = FastAPI(
    title="Nexus Engine IDR",
    description="Intelligent Demand Router for Prebid Server",
    version="1.0.0"
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Prometheus metrics endpoint
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)

# Initialize components
db = Database()
classifier = RequestClassifier()
scorer = BidderScorer(db)
selector = PartnerSelector()


@app.on_event("startup")
async def startup():
    """Initialize database connections on startup."""
    await db.connect()
    logger.info("IDR started successfully")


@app.on_event("shutdown")
async def shutdown():
    """Close connections on shutdown."""
    await db.disconnect()
    logger.info("IDR shutdown complete")


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    redis_ok = await db.redis_ping()
    postgres_ok = await db.postgres_ping()
    
    return HealthResponse(
        status="healthy" if (redis_ok and postgres_ok) else "degraded",
        redis=redis_ok,
        postgres=postgres_ok
    )


@app.post("/route", response_model=RouteResponse)
async def route_demand(request: RouteRequest):
    """
    Main routing endpoint.
    Called by PBS hook to determine which bidders to include.
    """
    try:
        # 1. Classify the request
        classified = classifier.classify(request.bid_request)
        
        # 2. Get all configured bidders
        all_bidders = request.configured_bidders
        
        # 3. Score each bidder
        scores = []
        for bidder in all_bidders:
            score = await scorer.score(bidder, classified)
            scores.append(score)
        
        # 4. Select partners
        selected = selector.select(scores, classified)
        
        # 5. Log routing decision (async, don't block)
        await db.log_routing_decision(
            auction_id=request.auction_id,
            classified=classified,
            scores=scores,
            selected=selected
        )
        
        return RouteResponse(
            auction_id=request.auction_id,
            selected_bidders=selected,
            all_scores=[s.dict() for s in scores],
            routing_time_ms=classifier.last_duration_ms
        )
        
    except Exception as e:
        logger.error(f"Routing error: {e}")
        # On error, return all bidders (fail open)
        return RouteResponse(
            auction_id=request.auction_id,
            selected_bidders=request.configured_bidders,
            error=str(e)
        )


@app.post("/analytics")
async def record_analytics(event: AnalyticsEvent):
    """
    Record auction outcomes for learning.
    Called by PBS after each auction completes.
    """
    try:
        await db.record_auction_event(event)
        return {"status": "recorded"}
    except Exception as e:
        logger.error(f"Analytics error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/floors/{publisher_id}")
async def get_floors(publisher_id: str):
    """
    Return dynamic floor prices for a publisher.
    """
    floors = await db.get_publisher_floors(publisher_id)
    return floors


@app.get("/stats/{publisher_id}")
async def get_stats(publisher_id: str, days: int = 7):
    """
    Return routing statistics for a publisher.
    """
    stats = await db.get_publisher_stats(publisher_id, days)
    return stats
```

### Nginx Configuration

**File:** `nginx/conf.d/default.conf`

```nginx
# The Nexus Engine - Nginx Configuration

# Upstream PBS nodes
upstream pbs_backend {
    least_conn;
    server pbs-1:8000 weight=1 max_fails=3 fail_timeout=30s;
    server pbs-2:8000 weight=1 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

# Rate limiting zones
limit_req_zone $binary_remote_addr zone=auction_limit:10m rate=100r/s;
limit_req_zone $binary_remote_addr zone=sync_limit:10m rate=10r/s;

# Caching
proxy_cache_path /var/cache/nginx/pbs levels=1:2 keys_zone=pbs_cache:10m max_size=100m inactive=60m;

server {
    listen 80;
    listen [::]:80;
    server_name pbs.nexusengine.io;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name pbs.nexusengine.io;
    
    # SSL certificates
    ssl_certificate /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:50m;
    ssl_session_tickets off;
    
    # Modern SSL config
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;
    
    # CORS headers
    add_header Access-Control-Allow-Origin "*" always;
    add_header Access-Control-Allow-Methods "GET, POST, OPTIONS" always;
    add_header Access-Control-Allow-Headers "Content-Type, Authorization" always;
    
    # Handle preflight
    if ($request_method = 'OPTIONS') {
        return 204;
    }
    
    # Logging
    access_log /var/log/nginx/pbs_access.log;
    error_log /var/log/nginx/pbs_error.log;
    
    # Gzip
    gzip on;
    gzip_types application/json application/javascript text/plain;
    gzip_min_length 1000;
    
    # ===========================================
    # AUCTION ENDPOINT
    # ===========================================
    location /openrtb2/auction {
        limit_req zone=auction_limit burst=200 nodelay;
        
        proxy_pass http://pbs_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 10s;
        proxy_read_timeout 10s;
        
        # Buffer settings
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 16k;
    }
    
    # ===========================================
    # COOKIE SYNC ENDPOINT
    # ===========================================
    location /cookie_sync {
        limit_req zone=sync_limit burst=20 nodelay;
        
        proxy_pass http://pbs_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    
    # ===========================================
    # SETUID ENDPOINT (cookie sync redirect)
    # ===========================================
    location /setuid {
        proxy_pass http://pbs_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    # ===========================================
    # STATUS ENDPOINT
    # ===========================================
    location /status {
        proxy_pass http://pbs_backend;
        proxy_cache pbs_cache;
        proxy_cache_valid 200 10s;
    }
    
    # ===========================================
    # INFO ENDPOINTS (cached)
    # ===========================================
    location /info {
        proxy_pass http://pbs_backend;
        proxy_cache pbs_cache;
        proxy_cache_valid 200 5m;
    }
    
    location /bidders {
        proxy_pass http://pbs_backend;
        proxy_cache pbs_cache;
        proxy_cache_valid 200 5m;
    }
}
```

### Redis Configuration

**File:** `redis/redis.conf`

```conf
# The Nexus Engine - Redis Configuration

# Network
bind 0.0.0.0
port 6379
protected-mode no

# Memory
maxmemory 512mb
maxmemory-policy allkeys-lru

# Persistence (RDB snapshots)
save 900 1
save 300 10
save 60 10000
dbfilename dump.rdb
dir /data

# Append-only file (disabled for performance)
appendonly no

# Performance tuning
tcp-keepalive 300
timeout 0

# Logging
loglevel notice
logfile ""
```

### TimescaleDB Init Script

**File:** `timescaledb/init/001-init.sql`

```sql
-- The Nexus Engine - Database Schema

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ===========================================
-- AUCTION EVENTS TABLE (main data)
-- ===========================================
CREATE TABLE auction_events (
    timestamp TIMESTAMPTZ NOT NULL,
    auction_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL,
    
    -- Request attributes
    country TEXT,
    region TEXT,
    device_type TEXT,
    os TEXT,
    browser TEXT,
    ad_format TEXT,
    ad_size TEXT,
    position TEXT,
    floor_price DECIMAL(10,4),
    
    -- Bidder response
    bidder_code TEXT NOT NULL,
    did_bid BOOLEAN NOT NULL DEFAULT FALSE,
    bid_cpm DECIMAL(10,4),
    bid_currency TEXT DEFAULT 'USD',
    response_time_ms INTEGER,
    did_win BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- User IDs present
    has_uid2 BOOLEAN DEFAULT FALSE,
    has_id5 BOOLEAN DEFAULT FALSE,
    has_liveramp BOOLEAN DEFAULT FALSE,
    
    -- Routing info
    was_selected BOOLEAN NOT NULL DEFAULT TRUE,
    routing_score DECIMAL(5,2)
);

-- Convert to hypertable (partitioned by time)
SELECT create_hypertable('auction_events', 'timestamp', chunk_time_interval => INTERVAL '1 day');

-- Indexes for common queries
CREATE INDEX idx_ae_publisher_time ON auction_events (publisher_id, timestamp DESC);
CREATE INDEX idx_ae_bidder_time ON auction_events (bidder_code, timestamp DESC);
CREATE INDEX idx_ae_lookup ON auction_events (publisher_id, country, device_type, ad_format, bidder_code);

-- ===========================================
-- ROUTING DECISIONS TABLE
-- ===========================================
CREATE TABLE routing_decisions (
    timestamp TIMESTAMPTZ NOT NULL,
    auction_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL,
    
    -- Request summary
    country TEXT,
    device_type TEXT,
    ad_format TEXT,
    
    -- Routing outcome
    total_configured INTEGER,
    total_selected INTEGER,
    routing_time_ms DECIMAL(5,2),
    
    -- Selected bidders (JSON array)
    selected_bidders JSONB,
    
    -- All scores (JSON array)
    all_scores JSONB
);

SELECT create_hypertable('routing_decisions', 'timestamp', chunk_time_interval => INTERVAL '1 day');

CREATE INDEX idx_rd_publisher_time ON routing_decisions (publisher_id, timestamp DESC);

-- ===========================================
-- MATERIALIZED VIEW: Hourly bidder performance
-- ===========================================
CREATE MATERIALIZED VIEW bidder_performance_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', timestamp) AS hour,
    publisher_id,
    country,
    device_type,
    ad_format,
    ad_size,
    bidder_code,
    
    COUNT(*) AS request_count,
    SUM(CASE WHEN did_bid THEN 1 ELSE 0 END) AS bid_count,
    SUM(CASE WHEN did_win THEN 1 ELSE 0 END) AS win_count,
    AVG(bid_cpm) FILTER (WHERE did_bid) AS avg_cpm,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms) 
        FILTER (WHERE response_time_ms IS NOT NULL) AS p95_latency,
    SUM(CASE WHEN did_bid AND bid_cpm >= floor_price THEN 1 ELSE 0 END)::FLOAT / 
        NULLIF(SUM(CASE WHEN did_bid THEN 1 ELSE 0 END), 0) AS floor_clearance_rate
        
FROM auction_events
GROUP BY 1, 2, 3, 4, 5, 6, 7
WITH NO DATA;

-- Refresh policy (every hour)
SELECT add_continuous_aggregate_policy('bidder_performance_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour'
);

-- ===========================================
-- MATERIALIZED VIEW: Daily publisher stats
-- ===========================================
CREATE MATERIALIZED VIEW publisher_stats_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', timestamp) AS day,
    publisher_id,
    
    COUNT(DISTINCT auction_id) AS total_auctions,
    COUNT(*) AS total_bid_requests,
    SUM(CASE WHEN did_bid THEN 1 ELSE 0 END) AS total_bids,
    SUM(CASE WHEN did_win THEN 1 ELSE 0 END) AS total_wins,
    AVG(bid_cpm) FILTER (WHERE did_win) AS avg_winning_cpm,
    SUM(bid_cpm) FILTER (WHERE did_win) AS total_revenue_proxy
    
FROM auction_events
GROUP BY 1, 2
WITH NO DATA;

SELECT add_continuous_aggregate_policy('publisher_stats_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day'
);

-- ===========================================
-- DATA RETENTION POLICY
-- ===========================================
-- Keep raw events for 90 days
SELECT add_retention_policy('auction_events', INTERVAL '90 days');
SELECT add_retention_policy('routing_decisions', INTERVAL '90 days');

-- ===========================================
-- PUBLISHER ACCOUNTS TABLE
-- ===========================================
CREATE TABLE publishers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    domain TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Config
    max_bidders INTEGER DEFAULT 15,
    min_score_threshold DECIMAL(5,2) DEFAULT 25.0,
    exploration_rate DECIMAL(3,2) DEFAULT 0.10,
    
    -- Always/never include bidders
    always_include_bidders TEXT[] DEFAULT '{}',
    never_include_bidders TEXT[] DEFAULT '{}',
    
    -- Floor config
    default_floor DECIMAL(10,4) DEFAULT 0.10,
    floor_currency TEXT DEFAULT 'GBP'
);

-- Insert BizBudding account
INSERT INTO publishers (id, name, domain) 
VALUES ('bizbudding_001', 'BizBudding', 'bizbudding.com');

-- ===========================================
-- HELPER FUNCTIONS
-- ===========================================

-- Get bidder metrics for scoring
CREATE OR REPLACE FUNCTION get_bidder_metrics(
    p_bidder_code TEXT,
    p_publisher_id TEXT,
    p_country TEXT DEFAULT NULL,
    p_device_type TEXT DEFAULT NULL,
    p_ad_format TEXT DEFAULT NULL
)
RETURNS TABLE (
    bid_rate DECIMAL,
    win_rate DECIMAL,
    avg_cpm DECIMAL,
    p95_latency DECIMAL,
    floor_clearance_rate DECIMAL,
    sample_size BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COALESCE(SUM(bid_count)::DECIMAL / NULLIF(SUM(request_count), 0), 0) AS bid_rate,
        COALESCE(SUM(win_count)::DECIMAL / NULLIF(SUM(bid_count), 0), 0) AS win_rate,
        COALESCE(AVG(bph.avg_cpm), 0) AS avg_cpm,
        COALESCE(AVG(bph.p95_latency), 500) AS p95_latency,
        COALESCE(AVG(bph.floor_clearance_rate), 0) AS floor_clearance_rate,
        COALESCE(SUM(request_count), 0) AS sample_size
    FROM bidder_performance_hourly bph
    WHERE bph.bidder_code = p_bidder_code
      AND bph.publisher_id = p_publisher_id
      AND bph.hour > NOW() - INTERVAL '7 days'
      AND (p_country IS NULL OR bph.country = p_country)
      AND (p_device_type IS NULL OR bph.device_type = p_device_type)
      AND (p_ad_format IS NULL OR bph.ad_format = p_ad_format);
END;
$$ LANGUAGE plpgsql;
```

### Prometheus Configuration

**File:** `prometheus/prometheus.yml`

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  # Prometheus self-monitoring
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  # PBS nodes
  - job_name: 'pbs'
    static_configs:
      - targets: ['pbs-1:9100', 'pbs-2:9100']
    metrics_path: /metrics

  # IDR
  - job_name: 'idr'
    static_configs:
      - targets: ['idr:8001']
    metrics_path: /metrics

  # Redis
  - job_name: 'redis'
    static_configs:
      - targets: ['redis:6379']

  # Nginx
  - job_name: 'nginx'
    static_configs:
      - targets: ['nginx:9113']
```

---

## Environment Variables

**File:** `.env.example`

```bash
# ===========================================
# THE NEXUS ENGINE - ENVIRONMENT CONFIG
# ===========================================

# TimescaleDB
TIMESCALE_DB=nexus
TIMESCALE_USER=nexus
TIMESCALE_PASSWORD=your_secure_password_here

# Grafana
GRAFANA_USER=admin
GRAFANA_PASSWORD=your_grafana_password_here

# Logging
LOG_LEVEL=INFO

# Domain (for SSL)
DOMAIN=pbs.nexusengine.io
```

---

## Deployment Scripts

### Setup Script

**File:** `scripts/setup.sh`

```bash
#!/bin/bash
set -e

echo "🚀 Setting up The Nexus Engine..."

# Check dependencies
command -v docker >/dev/null 2>&1 || { echo "Docker required but not installed. Aborting." >&2; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "Docker Compose required but not installed. Aborting." >&2; exit 1; }

# Create .env from example if not exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from example..."
    cp .env.example .env
    echo "⚠️  Please edit .env with your credentials before continuing"
    exit 1
fi

# Create required directories
echo "📁 Creating directories..."
mkdir -p nginx/ssl
mkdir -p pbs/config/stored_requests
mkdir -p pbs/config/accounts

# Generate self-signed SSL cert for development (replace with real cert for prod)
if [ ! -f nginx/ssl/fullchain.pem ]; then
    echo "🔐 Generating self-signed SSL certificate..."
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout nginx/ssl/privkey.pem \
        -out nginx/ssl/fullchain.pem \
        -subj "/CN=pbs.nexusengine.io"
fi

# Build images
echo "🔨 Building Docker images..."
docker-compose build

# Start services
echo "🏃 Starting services..."
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be healthy..."
sleep 10

# Check health
echo "🏥 Checking service health..."
docker-compose ps

echo ""
echo "✅ Setup complete!"
echo ""
echo "Services available at:"
echo "  - PBS:        http://localhost:8000"
echo "  - IDR:        http://localhost:8001"
echo "  - Grafana:    http://localhost:3000"
echo "  - Prometheus: http://localhost:9090"
echo ""
echo "To view logs: docker-compose logs -f"
echo "To stop:      docker-compose down"
```

### Deploy Script (Production)

**File:** `scripts/deploy.sh`

```bash
#!/bin/bash
set -e

echo "🚀 Deploying The Nexus Engine to production..."

# Pull latest code
git pull origin main

# Build new images
docker-compose -f docker-compose.yml -f docker-compose.prod.yml build

# Rolling restart (zero downtime)
echo "♻️  Rolling restart..."

# Update PBS nodes one at a time
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-deps pbs-1
sleep 30
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-deps pbs-2
sleep 30

# Update IDR
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-deps idr

# Update Nginx (reload config)
docker-compose -f docker-compose.yml -f docker-compose.prod.yml exec nginx nginx -s reload

echo "✅ Deployment complete!"

# Health check
echo "🏥 Running health checks..."
curl -sf http://localhost:8000/status || echo "⚠️  PBS health check failed"
curl -sf http://localhost:8001/health || echo "⚠️  IDR health check failed"

echo "Done."
```

---

## Quick Start Commands

```bash
# Clone and setup
git clone https://github.com/nexus-engine/nexus-pbs.git
cd nexus-pbs
cp .env.example .env
# Edit .env with your credentials

# Start everything
docker-compose up -d

# View logs
docker-compose logs -f

# Check status
docker-compose ps

# Stop everything
docker-compose down

# Stop and remove volumes (full reset)
docker-compose down -v

# Rebuild after code changes
docker-compose build --no-cache
docker-compose up -d

# Scale PBS nodes
docker-compose up -d --scale pbs=4

# Enter a container for debugging
docker-compose exec pbs-1 sh
docker-compose exec idr bash

# View database
docker-compose exec timescaledb psql -U nexus -d nexus
```

---

## Production Checklist

- [ ] Replace self-signed SSL cert with real certificate
- [ ] Set strong passwords in `.env`
- [ ] Configure firewall rules (only expose 80/443)
- [ ] Set up log rotation
- [ ] Configure backup schedule for TimescaleDB
- [ ] Set up alerting in Grafana
- [ ] Load test before going live
- [ ] Configure CDN in front of Nginx (CloudFlare, etc.)
- [ ] Set up CI/CD pipeline for deployments

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | Dec 2024 | The Nexus Engine | Initial draft |
