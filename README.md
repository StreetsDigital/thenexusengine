# The Nexus Engine

[![CI](https://github.com/StreetsDigital/thenexusengine/actions/workflows/ci.yml/badge.svg)](https://github.com/StreetsDigital/thenexusengine/actions/workflows/ci.yml)
[![Deploy](https://github.com/StreetsDigital/thenexusengine/actions/workflows/deploy.yml/badge.svg)](https://github.com/StreetsDigital/thenexusengine/actions/workflows/deploy.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python&logoColor=white)](https://python.org/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.0-85EA2D?style=flat&logo=swagger)](docs/api/openapi.yaml)

**Intelligent Prebid Server with Dynamic Demand Routing**

The Nexus Engine combines a high-performance **Go-based Prebid Server** with a **Python-based Intelligent Demand Router (IDR)** - a machine learning-powered system that optimizes bid request routing to maximize yield while minimizing latency.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              PREBID.JS / SDK                                 │
│                         (Client-side header bidding)                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                     THE NEXUS ENGINE (PBS - Go)                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         /openrtb2/auction                            │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│  │  │   Request    │  │   Privacy    │  │   Stored     │               │   │
│  │  │   Parser     │→ │   Filter     │→ │   Requests   │               │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                       │
│                                      ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                       IDR CLIENT (Go → Python)                       │   │
│  │                          HTTP POST /api/select                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                       │
└──────────────────────────────────────┼───────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                   INTELLIGENT DEMAND ROUTER (Python)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │
│  │   Request    │  │   Bidder     │  │   Partner    │                       │
│  │   Classifier │→ │   Scorer     │→ │   Selector   │                       │
│  └──────────────┘  └──────────────┘  └──────────────┘                       │
│         │                 │                 │                                │
│         └────────────────┴────────────────┘                                 │
│                          │                                                   │
│                          ▼                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │              PERFORMANCE DATABASE (Redis + TimescaleDB)              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AUCTION ENGINE (Go)                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │
│  │   Bidder     │  │    Bid       │  │   Response   │                       │
│  │   Adapters   │→ │   Collector  │→ │   Builder    │                       │
│  └──────────────┘  └──────────────┘  └──────────────┘                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              ▼                        ▼                        ▼
        ┌──────────┐            ┌──────────┐            ┌──────────┐
        │ Rubicon  │            │ AppNexus │            │ PubMatic │  ... (22 static)
        └──────────┘            └──────────┘            └──────────┘
              │                        │                        │
              └────────────────────────┼────────────────────────┘
                                       │
                        ┌──────────────┴──────────────┐
                        ▼                             ▼
                 ┌─────────────┐              ┌─────────────┐
                 │  Dynamic    │              │  Dynamic    │  ... (custom OpenRTB)
                 │  Bidder 1   │              │  Bidder N   │
                 └─────────────┘              └─────────────┘
```

## Features

- **22 Prebid Bidder Adapters** - Premium SSPs, video/native specialists, regional partners
- **Dynamic OpenRTB Bidder Integration** - Add custom demand partners without code changes
- **Intelligent Demand Routing** - ML-powered bidder selection for optimal yield
- **Privacy Compliance** - GDPR/TCF, CCPA, COPPA filtering with GVL IDs
- **Production Ready** - Auth, rate limiting, circuit breakers, structured logging
- **Full Observability** - Prometheus metrics, JSON logging, request correlation
- **CI/CD Pipeline** - GitHub Actions with automated testing and deployment
- **Load Tested** - k6 test suite for 40M+ requests/month validation

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/StreetsDigital/thenexusengine.git
cd thenexusengine

# Install Python IDR dependencies
pip install -e ".[admin]"  # With admin UI
pip install -e ".[db]"     # With database support
pip install -e ".[dev]"    # With dev tools

# Build Go PBS server
cd pbs
go mod download
go build -o ../bin/pbs-server ./cmd/server
```

### Running the Services

```bash
# Terminal 1: Start IDR Service (Python)
python run_admin.py --port 5050

# Terminal 2: Start PBS Server (Go)
./bin/pbs-server --port 8000 --idr-url http://localhost:5050
```

### Docker Compose

```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_FORMAT` | Output format (json, console) | `json` |
| `IDR_ENABLED` | Enable IDR integration | `true` |
| `IDR_TIMEOUT_MS` | IDR request timeout | `50` |
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379` |
| `REDIS_SAMPLE_RATE` | Sampling rate for Redis (cost optimization) | `0.1` |

### IDR Configuration

Edit `config/idr_config.yaml` or use the Admin UI at http://localhost:5050:

```yaml
selector:
  bypass_enabled: false    # true = disable IDR, select all bidders
  shadow_mode: false       # true = log decisions without filtering
  max_bidders: 15
  min_score_threshold: 25

scoring:
  weights:
    win_rate: 0.25
    bid_rate: 0.20
    cpm: 0.15
    floor_clearance: 0.15
    latency: 0.10
    recency: 0.10
    id_match: 0.05
```

## API Documentation

Full OpenAPI 3.0 specification available at [`docs/api/openapi.yaml`](docs/api/openapi.yaml).

### PBS Server (Go) - Port 8000

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/openrtb2/auction` | POST | OpenRTB auction endpoint |
| `/health` | GET | Health check |
| `/status` | GET | Service status |
| `/info/bidders` | GET | List available bidders |
| `/metrics` | GET | Prometheus metrics |
| `/admin/circuit-breaker` | GET | Circuit breaker status |

### Example Auction Request

```bash
curl -X POST http://localhost:8000/openrtb2/auction \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "id": "auction-123",
    "imp": [{
      "id": "imp-1",
      "banner": {"w": 300, "h": 250},
      "bidfloor": 0.50
    }],
    "site": {"domain": "example.com"},
    "device": {"ua": "Mozilla/5.0...", "ip": "192.168.1.1"}
  }'
```

### IDR Service (Python) - Port 5050

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Admin Dashboard UI |
| `/api/config` | GET/POST | Full configuration |
| `/api/select` | POST | Partner selection |
| `/api/mode/bypass` | POST | Toggle bypass mode |
| `/api/mode/shadow` | POST | Toggle shadow mode |
| `/api/bidders` | GET/POST | List/create dynamic bidders |
| `/api/bidders/<code>` | GET/PUT/DELETE | Manage specific bidder |
| `/api/bidders/<code>/test` | POST | Test bidder endpoint |
| `/api/bidders/<code>/enable` | POST | Enable bidder |
| `/api/bidders/<code>/disable` | POST | Disable bidder |
| `/health` | GET | Health check |

## Logging

Structured JSON logging with request correlation IDs.

### Go (zerolog)

```go
// Logs include: timestamp, level, service, request_id, and custom fields
{"level":"info","service":"pbs","request_id":"abc123","method":"POST","path":"/openrtb2/auction","status":200,"duration_ms":45,"time":"2024-12-11T10:00:00Z"}
```

### Python (structlog)

```python
from src.idr.logging import get_logger, LogContext

log = get_logger(__name__)
with LogContext(request_id="abc123"):
    log.info("Processing auction", bidders=15, duration_ms=12)
```

## Testing

### Unit Tests

```bash
# Run Python tests (139 tests)
pytest tests/ -v

# With coverage
pytest tests/ --cov=src --cov-report=html

# Run Go tests (88 tests)
cd pbs && go test ./...
```

### Load Testing

```bash
# Install k6
brew install k6  # macOS
# or: https://k6.io/docs/getting-started/installation/

# Run load tests
k6 run tests/load/auction.js

# With custom target
PBS_URL=https://your-server.fly.dev k6 run tests/load/auction.js
```

**Load Test Scenarios:**

| Scenario | VUs | Duration | Target |
|----------|-----|----------|--------|
| Smoke | 1 | 30s | Verify functionality |
| Load | 15 | 5min | Normal traffic (~15 RPS) |
| Stress | 75 | 3min | Peak traffic (~75 RPS) |
| Spike | 100 | 50s | Traffic spike handling |

**Thresholds:**
- p95 response time < 400ms
- p99 response time < 800ms
- Success rate > 95%

### Test Coverage

| Component | Tests | Status |
|-----------|-------|--------|
| Request Classifier | 21 | ✅ |
| Bidder Scorer | 24 | ✅ |
| Partner Selector | 32 | ✅ |
| Privacy Filter | 39 | ✅ |
| Redis Sampling | 23 | ✅ |
| Go Exchange | 35 | ✅ |
| Go FPD | 12 | ✅ |
| Go Middleware | 25 | ✅ |
| Go IDR Client | 16 | ✅ |

## CI/CD

GitHub Actions workflows for automated testing and deployment.

### Workflows

| Workflow | Trigger | Description |
|----------|---------|-------------|
| `ci.yml` | PR, Push | Tests, linting, security scan, Docker build |
| `deploy.yml` | Push to main | Deploy to Fly.io with health checks |
| `release.yml` | Version tag | Build Docker image, create release, deploy |

### Required Secrets

| Secret | Description |
|--------|-------------|
| `FLY_API_TOKEN` | Fly.io deployment token |

## Deployment

### Local Development (Docker)

```bash
docker compose up -d
```

See [docs/docker-setup.md](docs/docker-setup.md) for detailed configuration.

### Production (Fly.io)

```bash
# Deploy
fly deploy

# Scale
fly scale count 3
fly scale memory 1024
```

**Production Features:**
- Auto-scaling based on CPU/memory
- Health check monitoring
- Redis sampling (10% to reduce costs)
- Configured for 40M+ requests/month

## Project Structure

```
thenexusengine/
├── .github/
│   ├── workflows/              # CI/CD pipelines
│   │   ├── ci.yml              # Test and build
│   │   ├── deploy.yml          # Fly.io deployment
│   │   └── release.yml         # Release automation
│   ├── dependabot.yml          # Dependency updates
│   └── CODEOWNERS              # Review assignments
├── config/
│   └── idr_config.yaml         # IDR configuration
├── docs/
│   ├── api/
│   │   └── openapi.yaml        # OpenAPI 3.0 specification
│   └── docker-setup.md         # Docker documentation
├── pbs/                         # Prebid Server (Go)
│   ├── cmd/server/              # Main server entry point
│   ├── internal/
│   │   ├── openrtb/             # OpenRTB models
│   │   ├── exchange/            # Auction engine
│   │   ├── adapters/            # Bidder adapters
│   │   │   ├── ortb/            # Dynamic OpenRTB adapter
│   │   │   │   ├── ortb.go      # Generic adapter implementation
│   │   │   │   └── registry.go  # Dynamic registry with Redis refresh
│   │   │   └── ...              # 22 static bidder adapters
│   │   ├── endpoints/           # HTTP handlers
│   │   ├── middleware/          # Auth, rate limiting, metrics
│   │   ├── fpd/                 # First-party data
│   │   └── metrics/             # Prometheus metrics
│   └── pkg/
│       ├── idr/                 # IDR client + circuit breaker
│       └── logger/              # Structured logging (zerolog)
├── src/idr/                     # Intelligent Demand Router (Python)
│   ├── classifier/              # Request classification
│   ├── scorer/                  # Bidder scoring
│   ├── selector/                # Partner selection
│   ├── privacy/                 # Privacy compliance
│   ├── database/                # Redis + TimescaleDB
│   ├── bidders/                 # Dynamic bidder management
│   │   ├── models.py            # Bidder configuration models
│   │   ├── storage.py           # Redis persistence layer
│   │   └── manager.py           # CRUD operations
│   ├── admin/                   # Admin dashboard
│   ├── logging.py               # Structured logging (structlog)
│   └── models/                  # Data models
├── tests/
│   ├── load/                    # k6 load tests
│   │   └── auction.js           # Auction endpoint tests
│   ├── test_*.py                # Python unit tests
│   └── ...
├── docker/
│   └── timescaledb/
│       └── init.sql             # Database schema
├── fly.toml                     # Fly.io config
├── Dockerfile                   # Container build
├── docker-compose.yml           # Local development
└── pyproject.toml               # Python project config
```

## Supported Bidders (22)

| Category | Bidders | GVL IDs |
|----------|---------|---------|
| **Premium SSPs** | appnexus, rubicon, pubmatic, openx, ix | 32, 52, 76, 69, 10 |
| **Mid-tier** | triplelift, sovrn, sharethrough, gumgum, 33across, criteo | 28, 13, 80, 61, 58, 91 |
| **Video Specialists** | spotx, beachfront, unruly | 165, 335, 36 |
| **Native Specialists** | teads, outbrain, taboola | 132, 164, 42 |
| **Regional (EMEA)** | adform, smartadserver, improvedigital | 50, 45, 253 |
| **Additional** | medianet, conversant | 142, 24 |

## Dynamic OpenRTB Bidder Integration

Add custom demand partners without code changes using the dynamic bidder system.

### Quick Start

1. **Access the Admin UI**: Navigate to `http://localhost:5050/bidders`
2. **Create a Bidder**: Click "Add Bidder" and configure the endpoint
3. **Test Connection**: Use "Test Endpoint" to verify connectivity
4. **Enable**: Set status to "Active" to include in auctions

### Example: Create a Custom Bidder via API

```bash
curl -X POST http://localhost:5050/api/bidders \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic <credentials>" \
  -d '{
    "bidder_code": "mypartner",
    "name": "My Demand Partner",
    "endpoint": {
      "url": "https://partner.example.com/rtb/bid",
      "method": "POST",
      "timeout_ms": 200,
      "protocol_version": "2.5",
      "auth_type": "bearer",
      "auth_token": "your-api-token"
    },
    "capabilities": {
      "media_types": ["banner", "video"],
      "currencies": ["USD"],
      "site_enabled": true,
      "app_enabled": true,
      "supports_gdpr": true,
      "supports_ccpa": true
    },
    "status": "active"
  }'
```

### Features

- **OpenRTB 2.5/2.6 Support** - Full protocol compliance
- **Multiple Auth Methods** - Bearer tokens, basic auth, custom headers
- **Request Transformation** - Map fields for bidder-specific formats
- **Response Transformation** - Price adjustments, field mapping
- **Publisher/Geo Targeting** - Control bidder participation
- **Real-time Updates** - No server restart required
- **GVL Vendor IDs** - GDPR compliance support

See [OpenRTB Bidder Integration Guide](docs/ortb-bidder-integration.md) for complete documentation.

## Development Roadmap

### Completed ✅

- [x] Core IDR (classifier, scorer, selector)
- [x] PBS Core (OpenRTB, auction, exchange)
- [x] 22 Bidder adapters with GVL IDs
- [x] Dynamic OpenRTB bidder integration (custom demand sources)
- [x] Privacy compliance (GDPR/TCF, CCPA, COPPA)
- [x] Database integration (Redis + TimescaleDB)
- [x] Production hardening (auth, rate limiting, circuit breaker)
- [x] CI/CD pipeline (GitHub Actions)
- [x] Structured logging (zerolog, structlog)
- [x] API documentation (OpenAPI 3.0)
- [x] Load testing (k6)

### Future Enhancements

- [ ] Additional privacy regulations (GPP, LGPD, PIPL)
- [ ] A/B testing framework
- [ ] Machine learning model for bid prediction
- [ ] Real-time dashboard analytics
- [ ] Grafana dashboard templates

## License

Proprietary - The Nexus Engine

## Links

- [OpenAPI Documentation](docs/api/openapi.yaml)
- [Docker Setup Guide](docs/docker-setup.md)
- [OpenRTB Bidder Integration](docs/ortb-bidder-integration.md)
- [Publisher Integration Guide](docs/publisher-integration.md)
- [Prebid Server (Go)](https://github.com/prebid/prebid-server)
- [OpenRTB 2.5 Spec](https://www.iab.com/wp-content/uploads/2016/03/OpenRTB-API-Specification-Version-2-5-FINAL.pdf)
- [Prebid.js Documentation](https://docs.prebid.org/)
