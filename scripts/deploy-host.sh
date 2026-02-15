#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/_common.sh"

echo "Building host_cli for Linux..."
"$REPO_DIR/host_client/build_linux_cli.sh"

echo "Deploying to server..."
rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
    "$REPO_DIR/host_client/host_cli_dist/linux/host_cli" \
    "$REPO_DIR/systemd/parch-host.service" \
    $SERVER:/root/host_client/

echo "Installing systemd service and restarting..."
$SSH_CMD << 'REMOTE'
cp /root/host_client/parch-host.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable parch-host
systemctl restart parch-host
systemctl status parch-host --no-pager
REMOTE

echo "Deploy complete!"
