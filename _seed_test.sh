#!/bin/sh
sqlite3 /app/data/radius.db "INSERT INTO nas (nasname, shortname, type, secret) VALUES ('127.0.0.1', 'localtest', 'other', 'testing123');"
sqlite3 /app/data/radius.db "INSERT INTO radcheck (username, attribute, op, value) VALUES ('testuser', 'Cleartext-Password', ':=', 'testpass');"
echo '--- radcheck ---'
sqlite3 /app/data/radius.db 'SELECT username, attribute, value FROM radcheck;'
echo '--- nas ---'
sqlite3 /app/data/radius.db 'SELECT nasname, shortname, secret FROM nas;'
5