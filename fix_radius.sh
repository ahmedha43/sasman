#!/bin/sh

set -eu

app_dir="/app"
default_site="/etc/raddb/sites-available/default"
radiusd_conf="/etc/raddb/radiusd.conf"
lmdb_mod_conf="/etc/raddb/mods-available/lmdb"
clients_conf="/etc/raddb/clients.conf"

echo "[$(date)] Configuring FreeRADIUS for LMDB..."

radius_owner() {
    user="$(awk -F: '/^(radius|freerad|radiusd):/ { print $1; exit }' /etc/passwd)"
    if [ -z "$user" ]; then
        echo "root:root"
        return
    fi

    gid="$(awk -F: -v user="$user" '$1 == user { print $4; exit }' /etc/passwd)"
    group="$(awk -F: -v gid="$gid" '$3 == gid { print $1; exit }' /etc/group)"
    if [ -z "$group" ]; then
        group="root"
    fi

    echo "${user}:${group}"
}

# Create LMDB module configuration if it doesn't exist
if [ ! -f "$lmdb_mod_conf" ]; then
    cat > "$lmdb_mod_conf" <<EOF
lmdb {
    db_dir = "/app/data/radius_db"
    db_name = "sasman.mdb"
    map_size = 104857600
}
EOF
fi

# Enable LMDB module
ln -sf /etc/raddb/mods-available/lmdb /etc/raddb/mods-enabled/lmdb

# Verify module exists
if [ ! -f "/usr/lib/freeradius/rlm_lmdb.so" ]; then
    echo "[$(date)] WARNING: /usr/lib/freeradius/rlm_lmdb.so not found. FreeRADIUS will fail to load LMDB!"
fi

# Ensure the DB directory exists
mkdir -p /app/data/radius_db
RADIUS_OWNER="$(radius_owner)"
chown -R "$RADIUS_OWNER" /app/data/radius_db
touch /app/data/radius_db/clients.conf
chown "$RADIUS_OWNER" /app/data/radius_db/clients.conf

# Initialize log file
touch /app/data/radius.log
chown "$RADIUS_OWNER" /app/data/radius.log
chmod 666 /app/data/radius.log

# Enable lmdb in accounting, authorize
# Remove SQL references first to be clean
sed -i 's/^[[:space:]]*sql[[:space:]]*$/\t# sql/g' "$default_site"

# Add lmdb to authorize (section-aware: checks only inside authorize block)
if ! awk '/^[[:space:]]*authorize[[:space:]]*\{/,/^[[:space:]]*\}/' "$default_site" | grep -q 'lmdb'; then
    awk 'BEGIN{done=0} /^[[:space:]]*authorize[[:space:]]*\{/ && !done {print; print "\tlmdb"; done=1; next} 1' \
        "$default_site" > /tmp/radius_site_tmp && mv /tmp/radius_site_tmp "$default_site"
    echo "[$(date)] Added lmdb to authorize section"
fi

# Add lmdb to accounting (section-aware: checks only inside accounting block)
if ! awk '/^[[:space:]]*accounting[[:space:]]*\{/,/^[[:space:]]*\}/' "$default_site" | grep -q 'lmdb'; then
    awk 'BEGIN{done=0} /^[[:space:]]*accounting[[:space:]]*\{/ && !done {print; print "\tlmdb"; done=1; next} 1' \
        "$default_site" > /tmp/radius_site_tmp && mv /tmp/radius_site_tmp "$default_site"
    echo "[$(date)] Added lmdb to accounting section"
fi

# Logging & Auth Conf
sed -i 's/^[[:space:]]*auth_goodpass = .*/\tauth_goodpass = yes/' "$radiusd_conf"
sed -i 's/^.*destination = .*/\tdestination = stdout/' "$radiusd_conf"
sed -i 's|file = .*|file = "/app/data/radius.log"|' "$radiusd_conf"
sed -i 's/^[[:space:]]*#\s*auth_log$/\tauth_log/' "$default_site"
sed -i 's/^[[:space:]]*auth = .*/\tauth = yes/' "$radiusd_conf"

# Disable username dot/character checks
sed -i 's/^[[:space:]]*filter_username/\t# filter_username/' "$default_site"

# Client Docker bridge for MikroTik
if ! grep -q "client docker_bridge {" "$clients_conf"; then
    RADIUS_SECRET=${RADIUS_SECRET:-123456}
    cat >> "$clients_conf" <<EOF

client docker_bridge {
    ipaddr = 0.0.0.0/0
    secret = ${RADIUS_SECRET}
    shortname = docker-bridge
    nastype = other
    require_message_authenticator = yes
    limit_proxy_state = yes
}
EOF
fi

# Include dynamic clients
if ! grep -q "INCLUDE /app/data/radius_db/clients.conf" "$clients_conf"; then
    echo "\$INCLUDE /app/data/radius_db/clients.conf" >> "$clients_conf"
fi

echo "[$(date)] RADIUS LMDB setup applied successfully."
