#!/bin/bash
# Simple RADIUS test script

USERNAME="222"
PASSWORD="222"
SECRET="123456"
SERVER="127.0.0.1"
PORT="1812"

echo "Testing RADIUS authentication..."
echo "Username: $USERNAME"
echo "Password: $PASSWORD"
echo "Server: $SERVER:$PORT"

# Create a basic RADIUS packet for testing
# This is a simplified test - in production you'd use radtest or similar

# For now, let's just check if the database is accessible
echo "Checking database access..."
sqlite3 /app/data/radius.db "SELECT username, reply FROM radpostauth ORDER BY authdate DESC LIMIT 5;" 2>/dev/null || echo "Database access failed"

echo "Test completed."