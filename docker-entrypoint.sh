#!/bin/sh

set -e

echo "[$(date)] Starting SASMAN Unified (Native Go RADIUS + Go Backend)"

# Create necessary directories
mkdir -p /app/data /app/data/radius_db /app/data/logs

# Clean up stale or architecture-mismatched lock files on startup
rm -f /app/data/lock.mdb /app/data/radius_db/lock.mdb /app/data/*.lock /app/data/radius_db/*.lock

# Configure resilient DNS resolution for MikroTik RouterOS containers (fixes ARM/RB4011 DNS timeouts)
if [ ! -s /etc/resolv.conf ] || grep -q "127.0.0.11" /etc/resolv.conf || grep -q "172.17.0.1" /etc/resolv.conf; then
    printf "nameserver 1.1.1.1\nnameserver 8.8.8.8\nnameserver 1.0.0.1\nnameserver 8.8.4.4\n" > /etc/resolv.conf 2>/dev/null || true
fi


echo "[$(date)] Handing over control to Supervisord..."
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
