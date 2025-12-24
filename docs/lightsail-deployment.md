# AWS Lightsail Deployment Guide

Deploy The Nexus Engine on AWS Lightsail with Docker Compose.

## Prerequisites

- AWS account with Lightsail access
- SSH key pair for instance access
- Domain name (optional, for TLS)

## Instance Requirements

| Plan | vCPU | RAM | Storage | Monthly Cost | Recommendation |
|------|------|-----|---------|--------------|----------------|
| $10 | 2 | 2GB | 60GB | $10 | Development/Testing |
| $20 | 2 | 4GB | 80GB | $20 | **Production (Recommended)** |
| $40 | 2 | 8GB | 160GB | $40 | High Traffic |

**Recommended: $20/month plan (4GB RAM)** for production workloads.

## Quick Start

### Step 1: Create Lightsail Instance

1. Go to [AWS Lightsail Console](https://lightsail.aws.amazon.com/)
2. Click **Create instance**
3. Select:
   - **Region**: Choose closest to your users
   - **Platform**: Linux/Unix
   - **Blueprint**: Ubuntu 22.04 LTS
   - **Instance plan**: $20/month (4GB RAM) recommended
4. Add your SSH key
5. Name your instance (e.g., `nexus-engine-prod`)
6. Click **Create instance**

### Step 2: Configure Firewall

1. Click on your instance
2. Go to **Networking** tab
3. Under **IPv4 Firewall**, add rules:

| Application | Protocol | Port | Source |
|-------------|----------|------|--------|
| Custom | TCP | 8000 | Any (0.0.0.0/0) |
| Custom | TCP | 5050 | Any (0.0.0.0/0) |

### Step 3: Connect and Deploy

SSH into your instance:

```bash
ssh -i your-key.pem ubuntu@<your-instance-ip>
```

Run the deployment script:

```bash
# Download and run deployment script
curl -fsSL https://raw.githubusercontent.com/StreetsDigital/thenexusengine/main/scripts/deploy-lightsail.sh | bash
```

Or manually:

```bash
# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Clone repository
git clone https://github.com/StreetsDigital/thenexusengine.git /opt/nexus-engine
cd /opt/nexus-engine

# Create environment file
cat > .env << EOF
POSTGRES_PASSWORD=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)
SECRET_KEY=$(openssl rand -hex 32)
LOG_LEVEL=info
LOG_FORMAT=json
IDR_ENABLED=true
IDR_TIMEOUT_MS=50
REDIS_SAMPLE_RATE=0.1

# Privacy enforcement
PBS_ENFORCE_GDPR=true
PBS_ENFORCE_COPPA=true
PBS_ENFORCE_CCPA=true
PBS_PRIVACY_STRICT_MODE=false

# Authentication
AUTH_ENABLED=true
API_KEY_DEFAULT=$(openssl rand -hex 32)

# Admin authentication (REQUIRED - change these!)
ADMIN_USERS=admin:$(openssl rand -base64 16),ops:$(openssl rand -base64 16),devops:$(openssl rand -base64 16)
EOF

# IMPORTANT: Note your admin credentials
echo "=== ADMIN CREDENTIALS ==="
grep ADMIN_USERS .env
echo "========================="
echo "Save these credentials securely!"

# Start services
docker compose -f docker-compose.prod.yml up -d
```

### Step 4: Verify Deployment

```bash
# Check service status
docker compose -f docker-compose.prod.yml ps

# Test PBS health
curl http://localhost:8000/health

# Test IDR health
curl http://localhost:5050/health

# View logs
docker compose -f docker-compose.prod.yml logs -f
```

## Service Endpoints

After deployment, your services are available at:

| Service | URL | Description |
|---------|-----|-------------|
| PBS Auction | `http://<ip>:8000/openrtb2/auction` | Main auction endpoint |
| PBS Health | `http://<ip>:8000/health` | Health check |
| PBS Metrics | `http://<ip>:8000/metrics` | Prometheus metrics |
| IDR Admin | `http://<ip>:5050` | Admin dashboard |
| IDR Health | `http://<ip>:5050/health` | Health check |

## Production Hardening

### Admin Authentication (Required)

The admin dashboard requires authentication. Configure admin users via environment variables:

```bash
# Edit your .env file
nano /opt/nexus-engine/.env
```

**Environment Variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `ADMIN_USERS` | Comma-separated `user:pass` pairs | `admin:secret123,ops:pass456` |
| `ADMIN_USER_1` | Individual admin user | `admin:secret123` |
| `ADMIN_USER_2` | Second admin user | `ops:pass456` |
| `ADMIN_USER_3` | Third admin user | `devops:pass789` |
| `SECRET_KEY` | Flask session secret (auto-generated if not set) | `your-secret-key` |
| `SESSION_COOKIE_SECURE` | Set `true` when using HTTPS | `true` |

**Example .env for 3 admin users:**

```bash
# Admin authentication
ADMIN_USERS=admin:MySecurePass123,ops:OpsTeamPass456,devops:DevOpsPass789
SECRET_KEY=your-32-char-random-secret-key-here
SESSION_COOKIE_SECURE=true
```

**Or use individual variables:**

```bash
ADMIN_USER_1=admin:MySecurePass123
ADMIN_USER_2=ops:OpsTeamPass456
ADMIN_USER_3=devops:DevOpsPass789
```

After updating credentials, restart the IDR service:

```bash
docker compose -f docker-compose.prod.yml restart idr
```

**Security Notes:**
- All API endpoints require authentication when `ADMIN_USERS` is configured
- Health endpoints (`/health`) remain public for load balancer checks
- Sessions expire after 8 hours of inactivity
- Failed login attempts are rate-limited with a 0.5s delay
- Passwords are hashed using PBKDF2 with 100,000 iterations

### Enable TLS with Let's Encrypt

```bash
# Install Certbot
sudo apt update
sudo apt install -y certbot

# Get certificate (replace with your domain)
sudo certbot certonly --standalone -d nexus.yourdomain.com

# Copy certs for nginx
sudo mkdir -p /opt/nexus-engine/nginx/certs
sudo cp /etc/letsencrypt/live/nexus.yourdomain.com/fullchain.pem /opt/nexus-engine/nginx/certs/
sudo cp /etc/letsencrypt/live/nexus.yourdomain.com/privkey.pem /opt/nexus-engine/nginx/certs/

# Create nginx config
cat > /opt/nexus-engine/nginx/nginx.conf << 'EOF'
events {
    worker_connections 1024;
}

http {
    upstream pbs {
        server pbs:8000;
    }

    upstream idr {
        server idr:5050;
    }

    server {
        listen 80;
        server_name _;
        return 301 https://$host$request_uri;
    }

    server {
        listen 443 ssl http2;
        server_name _;

        ssl_certificate /etc/nginx/certs/fullchain.pem;
        ssl_certificate_key /etc/nginx/certs/privkey.pem;

        location / {
            proxy_pass http://pbs;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }

        location /admin {
            proxy_pass http://idr;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
    }
}
EOF

# Start with nginx
docker compose -f docker-compose.prod.yml --profile with-nginx up -d
```

### Set Up Auto-Renewal

```bash
# Add cron job for certificate renewal
echo "0 3 * * * certbot renew --quiet && docker compose -f /opt/nexus-engine/docker-compose.prod.yml restart nginx" | sudo crontab -
```

### Configure Static IP

1. Go to Lightsail Console → Networking
2. Click **Create static IP**
3. Attach to your instance
4. Update DNS to point to static IP

## Monitoring

### View Logs

```bash
# All services
docker compose -f docker-compose.prod.yml logs -f

# Specific service
docker compose -f docker-compose.prod.yml logs -f pbs
docker compose -f docker-compose.prod.yml logs -f idr

# Last 100 lines
docker compose -f docker-compose.prod.yml logs --tail=100 pbs
```

### Check Resource Usage

```bash
# Docker stats
docker stats

# System resources
htop
```

### Set Up Lightsail Alarms

1. Go to instance → Metrics tab
2. Add alarms for:
   - CPU utilization > 80%
   - Network in/out thresholds
   - Status check failures

## Maintenance

### Update Application

```bash
cd /opt/nexus-engine
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build
```

### Backup Database

```bash
# Backup TimescaleDB
docker compose -f docker-compose.prod.yml exec timescaledb pg_dump -U postgres idr > backup-$(date +%Y%m%d).sql

# Backup Redis
docker compose -f docker-compose.prod.yml exec redis redis-cli BGSAVE
```

### Restart Services

```bash
# Restart all
docker compose -f docker-compose.prod.yml restart

# Restart specific service
docker compose -f docker-compose.prod.yml restart pbs
```

### Stop Services

```bash
# Stop (keeps data)
docker compose -f docker-compose.prod.yml down

# Stop and remove volumes (DELETES DATA)
docker compose -f docker-compose.prod.yml down -v
```

## Troubleshooting

### Services Won't Start

```bash
# Check logs for errors
docker compose -f docker-compose.prod.yml logs

# Rebuild images
docker compose -f docker-compose.prod.yml build --no-cache
docker compose -f docker-compose.prod.yml up -d
```

### Out of Memory

```bash
# Check memory usage
free -h

# Reduce Redis memory in docker-compose.prod.yml
# Or upgrade to larger Lightsail instance
```

### Database Connection Issues

```bash
# Check TimescaleDB is running
docker compose -f docker-compose.prod.yml ps timescaledb

# Connect to database
docker compose -f docker-compose.prod.yml exec timescaledb psql -U postgres -d idr
```

### High CPU Usage

```bash
# Check which container is using CPU
docker stats

# Check PBS logs for issues
docker compose -f docker-compose.prod.yml logs pbs | grep -i error
```

## Cost Optimization

1. **Redis Sampling**: Already enabled at 10% (`REDIS_SAMPLE_RATE=0.1`) - reduces storage costs by 90%

2. **Log Rotation**: Configured in docker-compose.prod.yml - prevents disk filling up

3. **Right-size Instance**: Start with $20/month, scale up if needed

4. **Reserved Instance**: For long-term use, consider Lightsail reserved instances (save ~40%)

## Security Checklist

- [ ] Strong POSTGRES_PASSWORD in .env
- [ ] API_KEY_DEFAULT configured for PBS authentication
- [ ] Admin users configured (ADMIN_USERS)
- [ ] Privacy enforcement enabled (GDPR, COPPA, CCPA)
- [ ] Firewall rules restrict access appropriately
- [ ] TLS enabled for production traffic
- [ ] Regular backups configured
- [ ] Monitoring alarms set up
- [ ] SSH key-only authentication (no passwords)
- [ ] Keep system and Docker updated
