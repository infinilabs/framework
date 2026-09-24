#!/bin/bash
# Stop EasySearch Docker and clean up
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Stopping EasySearch..."
docker compose down -v
echo "Done."
