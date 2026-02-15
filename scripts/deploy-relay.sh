#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/_common.sh"

echo "Deploying relay server..."
rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
    "$REPO_DIR/relay_server/auth.go" \
    "$REPO_DIR/relay_server/config.go" \
    "$REPO_DIR/relay_server/dispatch.go" \
    "$REPO_DIR/relay_server/email.go" \
    "$REPO_DIR/relay_server/events.go" \
    "$REPO_DIR/relay_server/host.go" \
    "$REPO_DIR/relay_server/main.go" \
    "$REPO_DIR/relay_server/notifications.go" \
    "$REPO_DIR/relay_server/sessions.go" \
    "$REPO_DIR/relay_server/socket.go" \
    "$REPO_DIR/relay_server/turn.go" \
    "$REPO_DIR/relay_server/types.go" \
    "$REPO_DIR/relay_server/user.go" \
    "$REPO_DIR/relay_server/call_auth.go" \
    "$REPO_DIR/relay_server/stripe.go" \
    $SERVER:/root/relay_server

rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
    "$REPO_DIR/relay_server/relay-migrations" \
    "$REPO_DIR/relay_server/static" \
    $SERVER:/root/relay_server

rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
    "$REPO_DIR/go.mod" "$REPO_DIR/go.sum" \
    $SERVER:/root/

echo "Building on server..."
$SSH_CMD "cd /root/relay_server && go build -o relay_server ."

echo "Restarting relay..."
$SSH_CMD "systemctl restart relay_server && systemctl status relay_server --no-pager"

echo "Deploy complete!"
