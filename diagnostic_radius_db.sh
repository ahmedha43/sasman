#!/bin/sh
# Diagnostic script for RADIUS SQLite database issues

set -e

DB_PATH="/app/data/radius.db"
echo "=== RADIUS Database Diagnostic ==="
echo ""

echo "1. Database file status:"
ls -la "$DB_PATH" 2>/dev/null || echo "Database not found!"
echo ""

echo "2. SQLite sidecar files status:"
ls -la "${DB_PATH}-wal" 2>/dev/null || echo "No WAL file"
ls -la "${DB_PATH}-shm" 2>/dev/null || echo "No SHM file"
echo ""

if [ -f "$DB_PATH" ]; then
    echo "3. Database integrity check:"
    if command -v sqlite3 >/dev/null 2>&1; then
        sqlite3 "$DB_PATH" "PRAGMA integrity_check;" 2>&1 || echo "Integrity check failed!"
    else
        echo "sqlite3 not available"
    fi
    echo ""

    echo "4. Journal mode:"
    sqlite3 "$DB_PATH" "PRAGMA journal_mode;" 2>&1 || echo "Cannot check journal mode"
    echo ""

    echo "5. Busy timeout:"
    sqlite3 "$DB_PATH" "PRAGMA busy_timeout;" 2>&1 || echo "No busy timeout set"
    echo ""

    echo "6. Journal checkpoint status:"
    sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(RESTART);" 2>&1 || echo "Cannot checkpoint (may not be in WAL mode)"
    echo ""

    echo "7. Connection count (approximate from FreeRADIUS logs):"
    grep -c "rlm_sql_sqlite: Opening SQLite database" /app/data/radius.log 2>/dev/null || echo "No logs found"
fi

echo ""
echo "8. File permissions:"
stat -c "%a %U:%G %n" "$DB_PATH" 2>/dev/null || echo "Cannot stat file"
echo ""

echo "9. Open file handles (if lsof available):"
if command -v lsof >/dev/null 2>&1; then
    lsof "$DB_PATH" 2>/dev/null || echo "No open handles"
else
    echo "lsof not available"
fi

echo ""
echo "10. Recent RADIUS errors:"
grep -i "error\|busy\|lock\|reconnect" /app/data/radius.log | tail -10 2>/dev/null || echo "No recent errors found"
