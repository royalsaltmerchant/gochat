#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$REPO_DIR/dist"
BIN="$OUT_DIR/relay_server"
REMOTE_DIR="/root/relay_dist"
STAGE_DIR="$(mktemp -d)"
source "$SCRIPT_DIR/_common.sh"
trap 'rm -rf "$STAGE_DIR"' EXIT

mkdir -p "$OUT_DIR"

echo "Building relay server locally (linux/amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BIN" "$REPO_DIR/relay_server"

echo "Preparing deploy bundle..."
cp "$BIN" "$STAGE_DIR/relay_server"
cp -R "$REPO_DIR/relay_server/relay-migrations" "$STAGE_DIR/relay-migrations"
cp -R "$REPO_DIR/relay_server/static" "$STAGE_DIR/static"

echo "Ensuring remote directory exists..."
$SSH_CMD "mkdir -p $REMOTE_DIR"

echo "Syncing relay bundle and removing stale remote files (DB files preserved)..."
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

echo "Restarting relay..."
$SSH_CMD "systemctl restart relay_server && systemctl status relay_server --no-pager"

echo "Deploy complete!"
