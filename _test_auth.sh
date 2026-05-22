#!/bin/sh
apk add --no-cache freeradius-utils 2>&1 | tail -1
sqlite3 /app/data/radius.db "INSERT OR IGNORE INTO nas (nasname, shortname, type, secret) VALUES ('127.0.0.1', 'local', 'other', 'testing123');"
sqlite3 /app/data/radius.db "INSERT OR IGNORE INTO radcheck (username, attribute, op, value) VALUES ('testuser', 'Cleartext-Password', ':=', 'testpass');"
echo '--- HUP radiusd to pick up new NAS ---'
pkill -HUP radiusd
sleep 2
echo '--- running radtest ---'
radtest testuser testpass 127.0.0.1 0 testing123 2>&1 | tail -5
echo '--- radius.log last lines ---'
tail -n 15 /var/log/radius/radius.log
