#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Building host_cli for Linux..."
"$SCRIPT_DIR/build_linux_cli.sh"

echo "Deploying to server..."
rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
    "$SCRIPT_DIR/host_cli_dist/linux/host_cli" \
    "$REPO_DIR/systemd/parch-host.service" \
    root@64.23.134.139:/root/host_client/

echo "Installing systemd service..."
ssh -i ~/.ssh/id_rsa root@64.23.134.139 << 'REMOTE'
cp /root/host_client/parch-host.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable parch-host
systemctl restart parch-host
systemctl status parch-host --no-pager
REMOTE

echo "Deploy complete!"
