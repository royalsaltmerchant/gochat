#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$REPO_DIR/chat_relay/dist/chat_relay"
REMOTE_DIR="/root/go_chat/chat_relay"
STAGE_DIR="$(mktemp -d)"
source "$SCRIPT_DIR/_common.sh"
trap 'rm -rf "$STAGE_DIR"' EXIT

echo "Building chat_relay for Linux..."
"$REPO_DIR/scripts/build-chat-relay.sh"

echo "Preparing deploy bundle..."
cp "$BIN" "$STAGE_DIR/chat_relay"
cp -R "$REPO_DIR/chat_relay/static" "$STAGE_DIR/static"
cp "$REPO_DIR/systemd/chat-relay.service" "$STAGE_DIR/chat-relay.service"

echo "Ensuring remote directory exists..."
$SSH_CMD "mkdir -p $REMOTE_DIR"

echo "Syncing chat relay bundle and removing stale remote files (DB/env preserved)..."
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
cp /root/go_chat/chat_relay/chat-relay.service /etc/systemd/system/chat-relay.service
systemctl daemon-reload
systemctl enable chat-relay
systemctl restart chat-relay
systemctl status chat-relay --no-pager
REMOTE

echo "Deploy complete!"
