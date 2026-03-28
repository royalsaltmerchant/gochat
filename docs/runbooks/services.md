# Services Operations Runbook

Status:
- Active runbook.
- Date: 2026-03-08.

## Scope

This runbook covers:
- local development workflows
- deployment workflows
- rollback workflows
- production cleanup and rollout tasks for `relay_server` -> `call_service`

All commands are run from repo root unless noted.

## Service Map

| Service | Purpose | Local Port | Deploy Script | Remote Dir | systemd Unit |
|---|---|---:|---|---|---|
| `chat_relay` | Chat signaling + landing/static + web chat assets | `8001` | `./scripts/deploy-chat-relay.sh` | `/root/go_chat/chat_relay` | `chat-relay` |
| `call_service` | Call auth/billing/runtime + call pages/assets | `8000` | `./scripts/deploy-call-service.sh` | `/root/call_service_dist` | `call-service` |
| `host_client` | Official host process for chat space/channel persistence | n/a | `./scripts/deploy-host.sh` | `/root/host_client` | `parch-host` |
| `call_app` | React/Vite call frontend bundle | `5173` (dev) | bundled via `npm run build` | output to `call_service/static/call` | n/a |
| `web_client/frontend` | Vite web chat frontend bundle | Vite default (dev) | bundled via `npm run build:web` | output to `chat_relay/static/client` | n/a |
| `caddy` | Public TLS edge + routing | `80/443` | `./scripts/caddy.sh` | `/etc/caddy/Caddyfile` | `caddy` |
| `ion_sfu` | SFU media router | `7000` | managed outside this repo | external binary | `ion_sfu` |
| `turn_client` | TURN for client media relay | `3478` | managed outside this repo | `/etc/turnserver_client.conf` | `turn_client` |
| `turn_sfu` | TURN for SFU relay | `3479` | managed outside this repo | `/etc/turnserver_sfu.conf` | `turn_sfu` |

## Local Development Workflows

### 1) Chat Relay + Web Chat

1. Build web chat bundle if needed:
```bash
cd web_client/frontend
npm install
npm run build:web
cd ../..
```
2. Run relay:
```bash
go run ./chat_relay
```
3. Validate:
- `http://localhost:8001/`
- `http://localhost:8001/client`
- `http://localhost:8001/healthz`

### 2) Call Service + Call App

1. Build call app bundle if needed:
```bash
cd call_app
npm install
npm run build
cd ..
```
2. Run call service:
```bash
go run ./call_service
```
3. Validate:
- `http://localhost:8000/call`
- `http://localhost:8000/call/account`
- `http://localhost:8000/call/room`

### 3) Host Client

1. Build host client:
```bash
./host_client/build_linux_cli.sh
```
2. Run locally if needed:
```bash
go run ./host_client
```

### 4) Quick Compile Checks

```bash
GOCACHE=/tmp/go-build-cache go test ./call_service ./chat_relay -run '^$'
```

## Deployment Workflows

## 1) Chat-only Release

1. Build web chat bundle:
```bash
cd web_client/frontend && npm run build:web && cd ../..
```
2. Deploy chat relay:
```bash
./scripts/deploy-chat-relay.sh
```
3. Validate:
- `https://parchchat.com/`
- `https://chat.parchchat.com/client`

## 2) Call-only Release

1. Build call app bundle:
```bash
cd call_app && npm run build && cd ..
```
2. Deploy call service:
```bash
./scripts/deploy-call-service.sh
```
3. Validate:
- `https://parchchat.com/call`
- `https://parchchat.com/call/account`
- `https://parchchat.com/call/room`

## 3) Infra Routing (Caddy) Release

1. Optional backup remote Caddyfile:
```bash
./scripts/caddy.sh backup
```
2. Validate local config:
```bash
./scripts/caddy.sh validate-local
```
3. Apply:
```bash
./scripts/caddy.sh apply
```
4. Validate:
```bash
./scripts/caddy.sh status
./scripts/caddy.sh logs
```

## Rollback Workflows

## 1) Caddy Rollback

1. Restore previous `/etc/caddy/Caddyfile` from backup on host.
2. Reload caddy:
```bash
./scripts/caddy.sh reload
```
3. Validate `parchchat.com`, `chat.parchchat.com`, and `sfu.parchchat.com`.

## 2) Service Rollback

1. Restore previous binary + static in remote service dir from your backup archive.
2. Restart service:
```bash
./scripts/call-service.sh restart
./scripts/host.sh restart
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "systemctl restart chat-relay && systemctl status chat-relay --no-pager"
```
3. Check logs:
```bash
./scripts/call-service.sh logs
./scripts/host.sh logs
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "journalctl -u chat-relay --no-pager -n 50"
```

## Production Cleanup + Rollout Tasks

Goal:
- fully cut over from legacy `relay_server` naming/layout
- keep service isolation (`chat_relay` owns root/static/client, `call_service` owns `/call*` + `/ws`)

### Phase 0: Inventory (Optional Backups)

1. Snapshot currently running units:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "systemctl list-units --type=service | egrep 'call|chat|relay|parch|ion|turn'"
```
2. Optional snapshot of unit files and key dirs:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "mkdir -p /root/ops-backup && cp -a /etc/systemd/system /root/ops-backup/systemd_$(date +%Y%m%d_%H%M%S)"
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "cp -a /etc/caddy/Caddyfile /root/ops-backup/Caddyfile.$(date +%Y%m%d_%H%M%S)"
```
3. Optional backup legacy relay dir if it exists:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "if [ -d /root/relay_dist ]; then mkdir -p /root/archive && mv /root/relay_dist /root/archive/relay_dist.$(date +%Y%m%d_%H%M%S); fi"
```

### Phase 1: Ensure Runtime Config Layout Is Correct

1. Ensure call service env file exists:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "mkdir -p /root/call_service_dist && touch /root/call_service_dist/.env"
```
2. If old env lives at `/root/.env`, copy it as baseline:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "if [ -f /root/.env ] && [ ! -s /root/call_service_dist/.env ]; then cp /root/.env /root/call_service_dist/.env; fi"
```
3. Normalize `HOST_DB_FILE` before restart. Use an absolute path in `/root/call_service_dist/.env`.
   For migrated instances, `HOST_DB_FILE=/root/host.db` may be the correct value.
   A relative value like `host.db` resolves under `/root/call_service_dist` because the unit sets `WorkingDirectory=/root/call_service_dist`, which can strand the real user DB in a different location.
4. Confirm unit points to new env/workdir:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "systemctl cat call-service"
```

### Phase 2: Deploy Services

1. Deploy chat relay:
```bash
./scripts/deploy-chat-relay.sh
```
2. Deploy call service:
```bash
./scripts/deploy-call-service.sh
```
3. Deploy host client if host schema/behavior changed:
```bash
./scripts/deploy-host.sh
```

### Phase 3: Apply Routing and Verify

1. Apply caddy config (backup optional):
```bash
./scripts/caddy.sh apply
```
2. Verify public routes:
- `https://parchchat.com/` should serve `chat_relay` landing
- `https://parchchat.com/call` should serve `call_service`
- `https://parchchat.com/client` should redirect to `https://chat.parchchat.com/client`
3. Verify service health:
```bash
./scripts/call-service.sh status
./scripts/host.sh status
./scripts/sfu.sh status
./scripts/turn-client.sh status
./scripts/turn-sfu.sh status
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "systemctl status chat-relay --no-pager"
```

### Phase 4: Decommission Legacy relay_server Artifacts

1. Stop/disable old unit only if present:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "if systemctl list-unit-files | grep -q '^relay_server\\.service'; then systemctl stop relay_server || true; systemctl disable relay_server || true; fi"
```
2. Remove old unit file and reload daemon:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "if [ -f /etc/systemd/system/relay_server.service ]; then rm -f /etc/systemd/system/relay_server.service; systemctl daemon-reload; fi"
```
3. Archive any remaining relay_server runtime artifacts:
```bash
ssh -i ~/.ssh/id_rsa root@64.23.134.139 "for d in /root/relay_server /root/relay_dist; do if [ -d \"$d\" ]; then mkdir -p /root/archive && mv \"$d\" /root/archive/$(basename \"$d\").$(date +%Y%m%d_%H%M%S); fi; done"
```

### Phase 5: DNS Cleanup Tasks

1. Ensure A/AAAA records exist for:
- `parchchat.com`
- `www.parchchat.com`
- `chat.parchchat.com`
- `sfu.parchchat.com`
2. Remove stale relay-specific records if any remain.
3. Keep short TTL during cutover window, then raise TTL after stability.

## Post-Deploy Smoke Checklist

1. `chat.parchchat.com/client` loads and websocket connects.
2. `parchchat.com` landing loads with static assets from `/static/*`.
3. `parchchat.com/call` loads and call auth endpoints respond.
4. SFU auth via `sfu.parchchat.com` succeeds for valid token and fails for invalid token.
5. No repeated restart loops in:
- `journalctl -u chat-relay`
- `journalctl -u call-service`
- `journalctl -u caddy`
