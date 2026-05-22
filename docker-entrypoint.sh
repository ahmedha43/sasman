#!/bin/sh

set -e

echo "[$(date)] Starting SASMAN Unified (Native Go RADIUS + Go Backend)"

# Create necessary directories
mkdir -p /app/data /app/data/radius_db /app/data/logs

# Bypass default Docker variables if present
CLOUDFLARE_TUNNEL_ENABLED=${CLOUDFLARE_TUNNEL_ENABLED:-true}
if [ "$CLOUDFLARE_TUNNEL_ENABLED" = "true" ] && command -v cloudflared >/dev/null 2>&1; then
    echo "[$(date)] Cloudflare Tunnel enabled in supervisor."
    sed -i '/\[program:cloudflared\]/,/^priority=/ s/autostart=false/autostart=true/' /etc/supervisor/conf.d/supervisord.conf
fi

echo "[$(date)] Handing over control to Supervisord..."
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
