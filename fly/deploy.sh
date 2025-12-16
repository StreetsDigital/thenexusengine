#!/bin/bash
# The Nexus Engine - Fly.io Deployment Script
#
# Prerequisites:
#   - flyctl installed: https://fly.io/docs/flyctl/install/
#   - Logged in: fly auth login
#
# Usage:
#   ./fly/deploy.sh setup    # First-time setup (creates apps, databases)
#   ./fly/deploy.sh deploy   # Deploy/update services
#   ./fly/deploy.sh status   # Check service status
#   ./fly/deploy.sh logs     # View logs

set -e

# Configuration - customize these
APP_PREFIX="nexus"
REGION="${FLY_REGION:-iad}"  # Default: US East

IDR_APP="${APP_PREFIX}-idr"
PBS_APP="${APP_PREFIX}-pbs"
PG_APP="${APP_PREFIX}-db"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_flyctl() {
    if ! command -v fly &> /dev/null; then
        log_error "flyctl not found. Install from https://fly.io/docs/flyctl/install/"
        exit 1
    fi

    if ! fly auth whoami &> /dev/null; then
        log_error "Not logged in. Run: fly auth login"
        exit 1
    fi

    log_info "Logged in as: $(fly auth whoami)"
}

setup_postgres() {
    log_info "Setting up Fly Postgres..."

    if fly apps list | grep -q "$PG_APP"; then
        log_warn "Postgres app '$PG_APP' already exists, skipping creation"
    else
        fly postgres create \
            --name "$PG_APP" \
            --region "$REGION" \
            --initial-cluster-size 1 \
            --vm-size shared-cpu-1x \
            --volume-size 1

        log_info "Postgres created. Connection string will be attached to apps."
    fi
}

setup_redis() {
    log_info "Setting up Redis..."
    echo ""
    echo "Fly.io doesn't have managed Redis. Options:"
    echo ""
    echo "1. Upstash Redis (Recommended - Free tier available)"
    echo "   - Go to: https://upstash.com/"
    echo "   - Create a Redis database"
    echo "   - Copy the REDIS_URL"
    echo ""
    echo "2. Fly Redis (Self-managed)"
    echo "   - fly apps create ${APP_PREFIX}-redis"
    echo "   - fly deploy --image redis:7-alpine -a ${APP_PREFIX}-redis"
    echo ""
    read -p "Enter your REDIS_URL (or press Enter to skip): " REDIS_URL

    if [ -n "$REDIS_URL" ]; then
        log_info "Setting REDIS_URL secret for IDR..."
        fly secrets set REDIS_URL="$REDIS_URL" -a "$IDR_APP" 2>/dev/null || true
    else
        log_warn "Skipping Redis setup. Set it later with:"
        echo "  fly secrets set REDIS_URL=redis://... -a $IDR_APP"
    fi
}

create_apps() {
    log_info "Creating Fly.io apps..."

    # Create IDR app
    if fly apps list | grep -q "$IDR_APP"; then
        log_warn "App '$IDR_APP' already exists"
    else
        fly apps create "$IDR_APP" --org personal
        log_info "Created app: $IDR_APP"
    fi

    # Create PBS app
    if fly apps list | grep -q "$PBS_APP"; then
        log_warn "App '$PBS_APP' already exists"
    else
        fly apps create "$PBS_APP" --org personal
        log_info "Created app: $PBS_APP"
    fi
}

attach_postgres() {
    log_info "Attaching Postgres to IDR..."
    fly postgres attach "$PG_APP" -a "$IDR_APP" --database-name idr 2>/dev/null || \
        log_warn "Postgres may already be attached or doesn't exist"
}

setup_secrets() {
    log_info "Setting up secrets..."

    # Generate a default API key if not provided
    DEFAULT_API_KEY="${API_KEY_DEFAULT:-$(openssl rand -hex 32)}"

    echo ""
    echo "Setting API keys for authentication..."
    fly secrets set API_KEY_DEFAULT="$DEFAULT_API_KEY" -a "$IDR_APP"
    fly secrets set API_KEY_DEFAULT="$DEFAULT_API_KEY" -a "$PBS_APP"

    echo ""
    log_info "API Key set. Save this for client access:"
    echo "  API_KEY: $DEFAULT_API_KEY"
    echo ""
}

deploy_idr() {
    log_info "Deploying IDR service..."
    fly deploy --config fly/idr.toml --remote-only
}

deploy_pbs() {
    log_info "Deploying PBS service..."
    fly deploy --config fly/pbs.toml --remote-only
}

show_status() {
    echo ""
    log_info "=== The Nexus Engine Status ==="
    echo ""

    echo "IDR Service ($IDR_APP):"
    fly status -a "$IDR_APP" 2>/dev/null || echo "  Not deployed"
    echo ""

    echo "PBS Service ($PBS_APP):"
    fly status -a "$PBS_APP" 2>/dev/null || echo "  Not deployed"
    echo ""

    echo "Postgres ($PG_APP):"
    fly status -a "$PG_APP" 2>/dev/null || echo "  Not created"
    echo ""

    log_info "=== Endpoints ==="
    echo "  PBS: https://$PBS_APP.fly.dev"
    echo "  IDR: https://$IDR_APP.fly.dev (internal: http://$IDR_APP.internal:5050)"
    echo ""
}

show_logs() {
    APP="${2:-$PBS_APP}"
    log_info "Showing logs for $APP..."
    fly logs -a "$APP"
}

run_setup() {
    log_info "=== The Nexus Engine - Fly.io Setup ==="
    echo ""

    check_flyctl
    create_apps
    setup_postgres
    setup_redis
    attach_postgres
    setup_secrets

    echo ""
    log_info "Setup complete! Now deploy with:"
    echo "  ./fly/deploy.sh deploy"
    echo ""
}

run_deploy() {
    log_info "=== Deploying The Nexus Engine ==="
    echo ""

    check_flyctl
    deploy_idr
    deploy_pbs
    show_status

    log_info "Deployment complete!"
}

# Main
case "${1:-help}" in
    setup)
        run_setup
        ;;
    deploy)
        run_deploy
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "$@"
        ;;
    idr)
        deploy_idr
        ;;
    pbs)
        deploy_pbs
        ;;
    *)
        echo "The Nexus Engine - Fly.io Deployment"
        echo ""
        echo "Usage: $0 <command>"
        echo ""
        echo "Commands:"
        echo "  setup   - First-time setup (creates apps, databases, secrets)"
        echo "  deploy  - Deploy/update all services"
        echo "  idr     - Deploy only IDR service"
        echo "  pbs     - Deploy only PBS service"
        echo "  status  - Show deployment status"
        echo "  logs    - View logs (default: PBS, or specify: logs idr)"
        echo ""
        echo "Environment variables:"
        echo "  FLY_REGION       - Deploy region (default: iad)"
        echo "  API_KEY_DEFAULT  - API key for authentication"
        echo ""
        ;;
esac
