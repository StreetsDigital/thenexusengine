# The Nexus Engine

[![CI](https://github.com/StreetsDigital/thenexusengine/actions/workflows/ci.yml/badge.svg)](https://github.com/StreetsDigital/thenexusengine/actions/workflows/ci.yml)
[![Deploy](https://github.com/StreetsDigital/thenexusengine/actions/workflows/deploy.yml/badge.svg)](https://github.com/StreetsDigital/thenexusengine/actions/workflows/deploy.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python&logoColor=white)](https://python.org/)

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
        │ Rubicon  │            │ AppNexus │            │ PubMatic │  ... (22 bidders available)
        └──────────┘            └──────────┘            └──────────┘
```

## Components

### 1. Prebid Server Core (`pbs/` - Go)

| Component | Description |
|-----------|-------------|
| `cmd/server/` | Main server entry point |
| `internal/endpoints/` | HTTP endpoints (`/openrtb2/auction`, `/status`, `/info`) |
| `internal/exchange/` | Auction execution and bid collection |
| `internal/adapters/` | Bidder adapter implementations |
| `internal/openrtb/` | OpenRTB 2.5/2.6 request/response models |
| `pkg/idr/` | IDR client for Python service |

### 2. Intelligent Demand Router (`src/idr/` - Python)

| Component | Description | Status |
|-----------|-------------|--------|
| `classifier/` | Extracts features from OpenRTB requests | ✅ Complete |
| `scorer/` | Scores bidders based on historical performance | ✅ Complete |
| `selector/` | Selects optimal bidders for each auction | ✅ Complete |
| `privacy/` | GDPR/CCPA/COPPA privacy compliance filtering | ✅ Complete |
| `database/` | Redis + TimescaleDB performance storage | ✅ Complete |
| `admin/` | Web UI for configuration management | ✅ Complete |

### 3. Admin Dashboard (`src/idr/admin/`)

Web-based UI for managing IDR configuration without code changes.

```bash
python run_admin.py  # http://localhost:5050
```

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

# Run Python tests
pytest tests/ -v
```

### Running the Services

```bash
# Terminal 1: Start IDR Service (Python)
python run_admin.py --port 5050

# Terminal 2: Start PBS Server (Go)
./bin/pbs-server --port 8000 --idr-url http://localhost:5050
```

### Configuration

Edit `config/idr_config.yaml` or use the Admin UI:

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

## API Endpoints

### PBS Server (Go) - Port 8000

#### Auction Endpoint

```
POST /openrtb2/auction
Content-Type: application/json

{
  "id": "auction-123",
  "imp": [{
    "id": "imp-1",
    "banner": {"w": 300, "h": 250},
    "bidfloor": 0.50
  }],
  "site": {"domain": "example.com"},
  "device": {"ua": "...", "ip": "..."}
}
```

#### Status Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /status` | Health check |
| `GET /info/bidders` | List available bidders |
| `GET /metrics` | Prometheus metrics |

### IDR Service (Python) - Port 5050

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Admin Dashboard UI |
| `/api/config` | GET/POST | Full configuration |
| `/api/select` | POST | Partner selection (called by PBS) |
| `/api/mode/bypass` | POST | Toggle bypass mode |
| `/api/mode/shadow` | POST | Toggle shadow mode |
| `/health` | GET | Health check |

## IDR Operating Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **Normal** | Full IDR filtering (10-20 bidders) | Production |
| **Shadow** | Returns all bidders, logs what would be excluded | A/B Testing |
| **Bypass** | Returns all bidders, no IDR processing | PBS Testing |

## Development Roadmap

### Phase 1: Core IDR ✅
- [x] Request Classifier
- [x] Bidder Scorer (7-dimension scoring)
- [x] Partner Selector (with diversity & exploration)
- [x] Admin Dashboard

### Phase 2: PBS Core (Go) ✅
- [x] OpenRTB request/response models
- [x] Auction endpoint
- [x] Exchange (bid collection)
- [x] Basic bidder adapters (AppNexus, Rubicon, PubMatic)
- [x] IDR client integration

### Phase 3: Integration ✅
- [x] Performance database (Redis + TimescaleDB)
- [x] Event pipeline for learning
- [x] Real bidder endpoint connections
- [x] Redis sampling for cost optimization

### Phase 4: Production ✅
- [x] 22 bidder adapter implementations (see full list below)
- [x] Privacy compliance (GDPR/TCF, CCPA, COPPA)
- [x] Production hardening (auth, rate limiting, circuit breaker)
- [x] Metrics & monitoring (Prometheus)
- [x] Fly.io deployment configuration

### Phase 5: Future Enhancements
- [ ] Additional privacy regulations (GPP, LGPD, PIPL)
- [ ] A/B testing framework
- [ ] Machine learning model for bid prediction
- [ ] Real-time dashboard analytics

## Testing

```bash
# Run Python IDR tests (139 tests)
pytest tests/ -v

# Run specific module
pytest tests/test_bidder_scorer.py -v

# With coverage
pytest tests/ --cov=src --cov-report=html

# Build and test Go PBS (88 tests)
cd pbs
go build ./...
go test ./...
```

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

## Deployment

### Local Development (Docker)

```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f
```

See [docs/docker-setup.md](docs/docker-setup.md) for detailed Docker configuration.

### Production (Fly.io)

The project includes Fly.io configuration for production deployment:

```bash
# Deploy to Fly.io
fly deploy

# Scale for traffic
fly scale count 3  # Multiple instances
fly scale memory 1024  # Increase memory
```

**Fly.io Configuration Highlights:**
- Auto-scaling based on CPU/memory
- Health checks on `/health` endpoint
- Redis sampling to reduce Upstash costs (10% sample rate)
- Configured for 40M+ requests/month

## Project Structure

```
thenexusengine/
├── config/
│   └── idr_config.yaml         # IDR configuration
├── pbs/                         # Prebid Server (Go)
│   ├── cmd/server/              # Main server
│   ├── internal/
│   │   ├── openrtb/             # OpenRTB models
│   │   ├── exchange/            # Auction engine
│   │   ├── adapters/            # 22 Bidder adapters (see list below)
│   │   ├── endpoints/           # HTTP handlers
│   │   ├── middleware/          # Auth, rate limiting, metrics
│   │   ├── fpd/                 # First-party data handling
│   │   └── metrics/             # Prometheus metrics
│   └── pkg/idr/                 # IDR client
├── src/                         # Python source
│   └── idr/                     # Intelligent Demand Router
│       ├── classifier/          # Request classification
│       ├── scorer/              # Bidder scoring
│       ├── selector/            # Partner selection
│       ├── privacy/             # Privacy compliance (GDPR/CCPA/COPPA)
│       ├── database/            # Redis + TimescaleDB integration
│       ├── admin/               # Admin dashboard
│       └── models/              # Data models
├── tests/                       # Python test suite
├── fly.toml                     # Fly.io deployment config
├── Dockerfile                   # Container build
├── docker-compose.yml           # Local development
├── run_admin.py                 # Admin UI launcher
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

## License

Proprietary - The Nexus Engine

## Links

- [Prebid Server (Go)](https://github.com/prebid/prebid-server)
- [OpenRTB 2.5 Spec](https://www.iab.com/wp-content/uploads/2016/03/OpenRTB-API-Specification-Version-2-5-FINAL.pdf)
- [Prebid.js Documentation](https://docs.prebid.org/)
