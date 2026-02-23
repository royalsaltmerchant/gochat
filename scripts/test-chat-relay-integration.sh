#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "Running chat_relay integration tests..."
for attempt in 1 2; do
  if go test -count=1 -run '^TestRelayIntegration' ./chat_relay; then
    exit 0
  fi
  if [ "$attempt" -lt 2 ]; then
    echo "Integration tests failed (attempt $attempt). Retrying once..."
    sleep 1
  fi
done

echo "Integration tests failed after 2 attempts."
exit 1
