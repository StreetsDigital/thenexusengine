#!/bin/bash
# The Nexus Engine - Health Monitor
# Run via cron or external monitoring service

set -e

# Configuration
PBS_URL="https://nexus-pbs.fly.dev"
IDR_URL="https://nexus-idr.fly.dev"
SLACK_WEBHOOK="${SLACK_WEBHOOK_URL:-}"

# Thresholds (seconds, with decimals)
LATENCY_WARN=0.5
LATENCY_CRIT=1.0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_ok() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_crit() { echo -e "${RED}[CRIT]${NC} $1"; }

send_slack_alert() {
    local message="$1"
    local severity="$2"
    
    if [ -n "$SLACK_WEBHOOK" ]; then
        local color="#36a64f"
        [ "$severity" = "warn" ] && color="#ff9900"
        [ "$severity" = "crit" ] && color="#ff0000"
        
        curl -s -X POST "$SLACK_WEBHOOK" \
            -H 'Content-Type: application/json' \
            -d "{\"attachments\":[{\"color\":\"$color\",\"title\":\"Nexus Engine Alert\",\"text\":\"$message\",\"ts\":$(date +%s)}]}" > /dev/null
    fi
}

check_endpoint() {
    local name="$1"
    local url="$2"
    
    local result=$(curl -s -o /dev/null -w "%{http_code} %{time_total}" --connect-timeout 10 --max-time 30 "$url")
    local http_code=$(echo "$result" | awk '{print $1}')
    local latency=$(echo "$result" | awk '{print $2}')
    local latency_ms=$(echo "$latency * 1000" | bc | cut -d. -f1)
    
    if [ "$http_code" != "200" ]; then
        log_crit "$name: HTTP $http_code (${latency_ms}ms)"
        send_slack_alert "$name is DOWN! HTTP $http_code" "crit"
        return 1
    elif (( $(echo "$latency > $LATENCY_CRIT" | bc -l) )); then
        log_crit "$name: ${latency_ms}ms (threshold: $(echo "$LATENCY_CRIT * 1000" | bc)ms)"
        send_slack_alert "$name high latency: ${latency_ms}ms" "crit"
        return 1
    elif (( $(echo "$latency > $LATENCY_WARN" | bc -l) )); then
        log_warn "$name: ${latency_ms}ms (threshold: $(echo "$LATENCY_WARN * 1000" | bc)ms)"
        send_slack_alert "$name elevated latency: ${latency_ms}ms" "warn"
        return 0
    else
        log_ok "$name: ${latency_ms}ms"
        return 0
    fi
}

check_auction() {
    local result=$(curl -s -o /tmp/auction_result.json -w "%{http_code} %{time_total}" \
        --connect-timeout 10 --max-time 30 \
        -X POST "$PBS_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{"id":"monitor","imp":[{"id":"1","banner":{"w":300,"h":250}}],"site":{"page":"https://test.com","publisher":{"id":"monitor"}},"tmax":500}')
    
    local http_code=$(echo "$result" | awk '{print $1}')
    local latency=$(echo "$result" | awk '{print $2}')
    local latency_ms=$(echo "$latency * 1000" | bc | cut -d. -f1)
    
    if [ "$http_code" != "200" ]; then
        log_crit "Auction: HTTP $http_code (${latency_ms}ms)"
        send_slack_alert "Auction endpoint DOWN! HTTP $http_code" "crit"
        return 1
    elif ! grep -q '"id"' /tmp/auction_result.json 2>/dev/null; then
        log_crit "Auction: Invalid response"
        send_slack_alert "Auction endpoint returning invalid response" "crit"
        return 1
    elif (( $(echo "$latency > $LATENCY_CRIT" | bc -l) )); then
        log_crit "Auction: ${latency_ms}ms (threshold: $(echo "$LATENCY_CRIT * 1000" | bc)ms)"
        send_slack_alert "Auction high latency: ${latency_ms}ms" "crit"
        return 1
    elif (( $(echo "$latency > $LATENCY_WARN" | bc -l) )); then
        log_warn "Auction: ${latency_ms}ms"
        return 0
    else
        log_ok "Auction: ${latency_ms}ms"
        return 0
    fi
}

# Main
echo "=== Nexus Engine Health Check $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
echo ""

ERRORS=0

check_endpoint "PBS Health" "$PBS_URL/health" || ((ERRORS++))
check_endpoint "IDR Health" "$IDR_URL/health" || ((ERRORS++))
check_auction || ((ERRORS++))

echo ""
if [ "$ERRORS" -gt 0 ]; then
    log_crit "Health check completed with $ERRORS error(s)"
    exit 1
else
    log_ok "All systems operational"
    exit 0
fi
