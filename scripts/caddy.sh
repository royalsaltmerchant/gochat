#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/_common.sh"

REMOTE_CADDYFILE="${REMOTE_CADDYFILE:-/etc/caddy/Caddyfile}"
LOCAL_CADDYFILE="${LOCAL_CADDYFILE:-$REPO_DIR/ops/Caddyfile}"
REMOTE_TMP_CADDYFILE="${REMOTE_TMP_CADDYFILE:-/tmp/Caddyfile.codex}"
LOCAL_CADDYFILE_TEMPLATE="${LOCAL_CADDYFILE_TEMPLATE:-$REPO_DIR/ops/Caddyfile.template}"

usage() {
  cat <<USAGE
Usage: $0 <command>

Commands:
  init             Ensure local ops/Caddyfile exists (from template if missing)
  fetch            Copy remote Caddyfile to local ops/Caddyfile
  backup           Create timestamped backup of remote Caddyfile
  validate-local   Validate local Caddyfile with local caddy binary
  validate-remote  Validate remote /etc/caddy/Caddyfile
  apply            Upload local Caddyfile, validate on remote, install, reload caddy
  reload           Reload caddy service
  restart          Restart caddy service
  status           Show caddy service status
  logs             Show recent caddy logs
USAGE
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

cmd="$1"

case "$cmd" in
  init)
    mkdir -p "$(dirname "$LOCAL_CADDYFILE")"
    if [[ -f "$LOCAL_CADDYFILE" ]]; then
      echo "Local Caddyfile already exists: $LOCAL_CADDYFILE"
    elif [[ -f "$LOCAL_CADDYFILE_TEMPLATE" ]]; then
      cp "$LOCAL_CADDYFILE_TEMPLATE" "$LOCAL_CADDYFILE"
      echo "Created local Caddyfile from template: $LOCAL_CADDYFILE"
    else
      echo "No local Caddyfile template found at: $LOCAL_CADDYFILE_TEMPLATE"
      exit 1
    fi
    ;;

  fetch)
    mkdir -p "$(dirname "$LOCAL_CADDYFILE")"
    scp -i ~/.ssh/id_rsa "$SERVER:$REMOTE_CADDYFILE" "$LOCAL_CADDYFILE"
    echo "Fetched $REMOTE_CADDYFILE -> $LOCAL_CADDYFILE"
    ;;

  backup)
    ts="$(date +%Y%m%d_%H%M%S)"
    $SSH_CMD "cp '$REMOTE_CADDYFILE' '${REMOTE_CADDYFILE}.backup.${ts}'"
    echo "Created remote backup: ${REMOTE_CADDYFILE}.backup.${ts}"
    ;;

  validate-local)
    if ! command -v caddy >/dev/null 2>&1; then
      echo "caddy binary not found locally"
      exit 1
    fi
    if [[ ! -f "$LOCAL_CADDYFILE" ]]; then
      echo "Local Caddyfile not found: $LOCAL_CADDYFILE"
      exit 1
    fi
    caddy validate --config "$LOCAL_CADDYFILE"
    ;;

  validate-remote)
    $SSH_CMD "caddy validate --config '$REMOTE_CADDYFILE'"
    ;;

  apply)
    if [[ ! -f "$LOCAL_CADDYFILE" ]]; then
      echo "Local Caddyfile not found: $LOCAL_CADDYFILE"
      exit 1
    fi

    scp -i ~/.ssh/id_rsa "$LOCAL_CADDYFILE" "$SERVER:$REMOTE_TMP_CADDYFILE"
    $SSH_CMD "caddy validate --config '$REMOTE_TMP_CADDYFILE'"
    $SSH_CMD "cp '$REMOTE_TMP_CADDYFILE' '$REMOTE_CADDYFILE' && rm -f '$REMOTE_TMP_CADDYFILE'"
    $SSH_CMD "systemctl reload caddy && systemctl status caddy --no-pager"
    ;;

  reload)
    $SSH_CMD "systemctl reload caddy && systemctl status caddy --no-pager"
    ;;

  restart)
    $SSH_CMD "systemctl restart caddy && systemctl status caddy --no-pager"
    ;;

  status)
    $SSH_CMD "systemctl status caddy --no-pager"
    ;;

  logs)
    $SSH_CMD "journalctl -u caddy --no-pager -n 100"
    ;;

  *)
    usage
    exit 1
    ;;
esac
