#!/bin/bash
# The Nexus Engine - AWS Lightsail Deployment Script
# Run this on your Lightsail instance after initial setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  The Nexus Engine - Lightsail Deploy  ${NC}"
echo -e "${GREEN}========================================${NC}"

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ]; then
    echo -e "${YELLOW}Note: Some commands may require sudo${NC}"
fi

# Configuration
INSTALL_DIR="${INSTALL_DIR:-/opt/nexus-engine}"
REPO_URL="${REPO_URL:-https://github.com/StreetsDigital/thenexusengine.git}"
BRANCH="${BRANCH:-main}"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Step 1: Install Docker if not present
install_docker() {
    if command_exists docker; then
        echo -e "${GREEN}✓ Docker already installed${NC}"
        docker --version
    else
        echo -e "${YELLOW}Installing Docker...${NC}"
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        sudo usermod -aG docker $USER
        rm get-docker.sh
        echo -e "${GREEN}✓ Docker installed${NC}"
    fi
}

# Step 2: Install Docker Compose if not present
install_docker_compose() {
    if command_exists docker-compose || docker compose version >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Docker Compose already installed${NC}"
    else
        echo -e "${YELLOW}Installing Docker Compose...${NC}"
        sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
        echo -e "${GREEN}✓ Docker Compose installed${NC}"
    fi
}

# Step 3: Clone or update repository
setup_repository() {
    if [ -d "$INSTALL_DIR" ]; then
        echo -e "${YELLOW}Updating existing installation...${NC}"
        cd "$INSTALL_DIR"
        git fetch origin
        git checkout "$BRANCH"
        git pull origin "$BRANCH"
    else
        echo -e "${YELLOW}Cloning repository...${NC}"
        sudo mkdir -p "$INSTALL_DIR"
        sudo chown $USER:$USER "$INSTALL_DIR"
        git clone -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
    fi
    echo -e "${GREEN}✓ Repository ready at $INSTALL_DIR${NC}"
}

# Step 4: Create environment file
setup_environment() {
    cd "$INSTALL_DIR"

    if [ ! -f .env ]; then
        echo -e "${YELLOW}Creating environment file...${NC}"

        # Generate secure password
        POSTGRES_PASSWORD=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)

        cat > .env << EOF
# The Nexus Engine - Production Environment
# Generated on $(date)

# Database (KEEP THIS SECRET!)
POSTGRES_PASSWORD=$POSTGRES_PASSWORD

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# IDR Settings
IDR_ENABLED=true
IDR_TIMEOUT_MS=50

# Redis Sampling (10% = 90% cost reduction)
REDIS_SAMPLE_RATE=0.1

# Event Recording
EVENT_RECORD_ENABLED=true
EOF

        chmod 600 .env
        echo -e "${GREEN}✓ Environment file created${NC}"
        echo -e "${YELLOW}IMPORTANT: Save this password securely:${NC}"
        echo -e "${RED}POSTGRES_PASSWORD=$POSTGRES_PASSWORD${NC}"
    else
        echo -e "${GREEN}✓ Environment file exists${NC}"
    fi
}

# Step 5: Build and start services
start_services() {
    cd "$INSTALL_DIR"

    echo -e "${YELLOW}Building Docker images...${NC}"
    docker compose -f docker-compose.prod.yml build

    echo -e "${YELLOW}Starting services...${NC}"
    docker compose -f docker-compose.prod.yml up -d

    echo -e "${GREEN}✓ Services started${NC}"
}

# Step 6: Wait for health checks
wait_for_health() {
    echo -e "${YELLOW}Waiting for services to be healthy...${NC}"

    # Wait up to 60 seconds for services
    for i in {1..12}; do
        if curl -sf http://localhost:8000/health >/dev/null 2>&1; then
            echo -e "${GREEN}✓ PBS Server is healthy${NC}"
            break
        fi
        echo "  Waiting for PBS Server... ($i/12)"
        sleep 5
    done

    for i in {1..12}; do
        if curl -sf http://localhost:5050/health >/dev/null 2>&1; then
            echo -e "${GREEN}✓ IDR Service is healthy${NC}"
            break
        fi
        echo "  Waiting for IDR Service... ($i/12)"
        sleep 5
    done
}

# Step 7: Setup firewall
setup_firewall() {
    echo -e "${YELLOW}Firewall configuration:${NC}"
    echo "  - Port 8000: PBS Server (required)"
    echo "  - Port 5050: IDR Admin Dashboard (optional)"
    echo ""
    echo -e "${YELLOW}Configure in Lightsail Console:${NC}"
    echo "  1. Go to your instance → Networking tab"
    echo "  2. Add firewall rules for ports 8000 and 5050"
}

# Step 8: Print status
print_status() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Deployment Complete!                  ${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "Service Status:"
    docker compose -f "$INSTALL_DIR/docker-compose.prod.yml" ps
    echo ""
    echo -e "Endpoints:"
    echo -e "  PBS Auction:  http://$(curl -s ifconfig.me):8000/openrtb2/auction"
    echo -e "  PBS Health:   http://$(curl -s ifconfig.me):8000/health"
    echo -e "  IDR Admin:    http://$(curl -s ifconfig.me):5050"
    echo -e "  IDR Health:   http://$(curl -s ifconfig.me):5050/health"
    echo ""
    echo -e "${YELLOW}Useful Commands:${NC}"
    echo "  View logs:     docker compose -f $INSTALL_DIR/docker-compose.prod.yml logs -f"
    echo "  Restart:       docker compose -f $INSTALL_DIR/docker-compose.prod.yml restart"
    echo "  Stop:          docker compose -f $INSTALL_DIR/docker-compose.prod.yml down"
    echo "  Update:        cd $INSTALL_DIR && git pull && docker compose -f docker-compose.prod.yml up -d --build"
}

# Main execution
main() {
    install_docker
    install_docker_compose
    setup_repository
    setup_environment
    start_services
    wait_for_health
    setup_firewall
    print_status
}

# Run main function
main "$@"
