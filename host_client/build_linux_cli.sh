#!/bin/bash

# Build host_cli for Linux (headless mode)

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/host_cli_dist/linux"

mkdir -p "$OUTPUT_DIR"

echo "Building host_cli for Linux (amd64)..."
cd "$SCRIPT_DIR"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o "$OUTPUT_DIR/host_cli" \
    .

echo "Build complete: $OUTPUT_DIR/host_cli"
ls -lh "$OUTPUT_DIR/host_cli"
