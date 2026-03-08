#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$REPO_DIR/dist"
BIN="$OUT_DIR/call_service"
REMOTE_DIR="/root/call_service_dist"
STAGE_DIR="$(mktemp -d)"
source "$SCRIPT_DIR/_common.sh"
trap 'rm -rf "$STAGE_DIR"' EXIT

mkdir -p "$OUT_DIR"

echo "Building call service locally (linux/amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BIN" "$REPO_DIR/call_service"

echo "Preparing deploy bundle..."
cp "$BIN" "$STAGE_DIR/call_service"
cp -R "$REPO_DIR/call_service/relay-migrations" "$STAGE_DIR/relay-migrations"
cp -R "$REPO_DIR/call_service/static" "$STAGE_DIR/static"
cp "$REPO_DIR/systemd/call-service.service" "$STAGE_DIR/call-service.service"

echo "Ensuring remote directory exists..."
$SSH_CMD "mkdir -p $REMOTE_DIR"

echo "Syncing call service bundle and removing stale remote files (DB files preserved)..."
rsync -av --progress --delete \
    --filter='P .env' \
    --filter='P *.db' \
    --filter='P *.db-wal' \
    --filter='P *.db-shm' \
    --filter='P *.sqlite' \
    --filter='P *.sqlite-wal' \
    --filter='P *.sqlite-shm' \
    -e "ssh -i ~/.ssh/id_rsa" \
    "$STAGE_DIR"/ \
    "$SERVER:$REMOTE_DIR/"

echo "Installing systemd service and restarting..."
$SSH_CMD << 'REMOTE'
cp /root/call_service_dist/call-service.service /etc/systemd/system/call-service.service
systemctl daemon-reload
systemctl enable call-service
systemctl restart call-service
systemctl status call-service --no-pager
REMOTE

echo "Deploy complete!"
