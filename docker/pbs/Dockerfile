# Go PBS Server Dockerfile
# Multi-stage build for smaller final image

# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY pbs/go.mod pbs/go.sum ./
RUN go mod download

# Copy source code
COPY pbs/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pbs-server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/pbs-server /app/

# Create non-root user
RUN adduser -D -u 1000 pbs && chown -R pbs:pbs /app
USER pbs

# Environment variables
ENV PBS_PORT=8000
ENV IDR_URL=http://idr:5050
ENV IDR_ENABLED=true
ENV IDR_TIMEOUT_MS=50
ENV EVENT_RECORD_ENABLED=true

EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8000/status || exit 1

CMD ["/app/pbs-server"]
