# Python IDR Service Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install system dependencies (including libc-dev for bitarray-hardbyte compilation)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc-dev \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY pyproject.toml ./

# Install Python dependencies
RUN pip install --no-cache-dir -e ".[admin,db]"

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
ENV REDIS_URL=redis://redis:6379
# TIMESCALE_URL must be provided at runtime - no default to prevent credential leaks
# ENV TIMESCALE_URL=postgresql://user:password@host:5432/idr

EXPOSE 5050

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:5050/health')" || exit 1

CMD ["python", "run_admin.py", "--host", "0.0.0.0", "--port", "5050", "--no-debug"]
