#!/bin/bash
# Shared config for remote scripts

SERVER="root@64.23.134.139"
SSH_CMD="ssh -i ~/.ssh/id_rsa $SERVER"

run_service_action() {
    local service="$1"
    local action="$2"

    case "$action" in
        restart)
            $SSH_CMD "systemctl restart $service && systemctl status $service --no-pager"
            ;;
        status)
            $SSH_CMD "systemctl status $service --no-pager"
            ;;
        logs)
            $SSH_CMD "journalctl -u $service --no-pager -n 50"
            ;;
        stop)
            $SSH_CMD "systemctl stop $service && echo '$service stopped'"
            ;;
        *)
            echo "Usage: $0 {restart|status|logs|stop}"
            exit 1
            ;;
    esac
}
