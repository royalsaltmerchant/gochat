#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$ROOT_DIR/chat_relay/dist"

mkdir -p "$OUT_DIR"

echo "Building chat_relay (linux amd64)..."
cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT_DIR/chat_relay" ./chat_relay

echo "Build complete: $OUT_DIR/chat_relay"
ls -lh "$OUT_DIR/chat_relay"
