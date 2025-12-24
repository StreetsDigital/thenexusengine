# The Nexus Engine - Combined Dockerfile
# Multi-stage build for PBS (Go) + IDR (Python)

# ====================
# Stage 1: Build Go PBS Server
# ====================
FROM golang:1.23-alpine AS go-builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY pbs/go.mod pbs/go.sum ./pbs/
WORKDIR /build/pbs
RUN go mod download

# Copy source code
COPY pbs/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /build/pbs-server ./cmd/server

# ====================
# Stage 2: Build Python IDR Service
# ====================
FROM python:3.12-slim AS python-builder

WORKDIR /build

# Install build dependencies (including libc-dev for bitarray-hardbyte compilation)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc-dev \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY pyproject.toml ./

# Install Python dependencies
RUN pip install --no-cache-dir build wheel
RUN pip wheel --no-cache-dir -w /wheels -e ".[admin,db]"

# ====================
# Stage 3: Runtime - PBS Server (default)
# ====================
FROM alpine:3.20 AS pbs

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata wget

# Copy binary from builder
COPY --from=go-builder /build/pbs-server /app/

# Create non-root user
RUN adduser -D -u 1000 pbs && chown -R pbs:pbs /app
USER pbs

# Environment variables
ENV PBS_PORT=8000
ENV IDR_URL=http://localhost:5050
ENV IDR_ENABLED=true
ENV IDR_TIMEOUT_MS=50
ENV EVENT_RECORD_ENABLED=true
ENV LOG_LEVEL=info
ENV LOG_FORMAT=json

EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

CMD ["/app/pbs-server"]

# ====================
# Stage 4: Runtime - IDR Service
# ====================
FROM python:3.12-slim AS idr

WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq5 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy wheels and install
COPY --from=python-builder /wheels /wheels
RUN pip install --no-cache-dir /wheels/*.whl && rm -rf /wheels

# Copy source code
COPY src/ ./src/
COPY config/ ./config/
COPY run_admin.py ./

# Create non-root user
RUN useradd -m -u 1000 idr && chown -R idr:idr /app
USER idr

# Environment variables
ENV PYTHONPATH=/app
ENV PYTHONUNBUFFERED=1
ENV IDR_PORT=5050
ENV REDIS_URL=redis://localhost:6379
ENV LOG_LEVEL=info
ENV LOG_FORMAT=json

EXPOSE 5050

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:5050/health || exit 1

CMD ["python", "run_admin.py", "--host", "0.0.0.0", "--port", "5050"]
