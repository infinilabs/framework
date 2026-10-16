#!/bin/bash
# Start EasySearch Docker for sharing module integration tests
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Starting EasySearch for sharing module tests..."
docker compose up -d

echo "Waiting for EasySearch to be healthy..."
timeout=120
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if curl -sku "admin:ShareTest_2026!" https://localhost:19200/_cluster/health 2>/dev/null | grep -q '"status"'; then
    echo "EasySearch is ready!"
    exit 0
  fi
  sleep 2
  elapsed=$((elapsed + 2))
  echo "  waiting... (${elapsed}s)"
done

echo "ERROR: EasySearch failed to start within ${timeout}s"
docker compose logs
exit 1
