# Parch

**The user-hosted messenger and call platform.**

Parch now runs chat and calls as separate services:
- `chat_relay`: host-backed chat signaling with public-key identities (no email/password auth for chat)
- `call_service`: call app auth, billing, and call runtime

---

## Architecture

```
┌─────────────────┐      ┌─────────────────┐
│   Web Client    │      │    Call App     │
│  (Vite/Browser) │      │  (React/Vite)   │
└────────┬────────┘      └────────┬────────┘
         │                        │
┌────────▼─────────┐      ┌───────▼─────────┐
│    Chat Relay    │      │  Call Service   │
│      (Go)        │      │      (Go)       │
└────────┬─────────┘      └───────┬─────────┘
         │                        │
  ┌──────▼──────┐          ┌──────▼──────┐
  │ Host Client │          │ SFU / TURN  │
  │ (Linux CLI) │          │ Call Stack  │
  └─────────────┘          └─────────────┘
```

Detailed chat auth + E2EE flow:
- `docs/chat-e2ee-architecture.md`
- `docs/chat-e2ee-key-design-decision.md`

Service boundary organization guidance:
- `docs/architecture/service-boundaries.md`

Operations runbook:
- `docs/runbooks/services.md`

### Components

| Component | Description |
|-----------|-------------|
| **Chat Relay** | Go service for host registration, chat websocket routing, and pubkey identity lookup |
| **Host Client** | Linux CLI process storing chat data locally (SQLite), manages spaces/channels/messages/invites |
| **Web Client** | Browser chat app using local public-key identity and end-to-end encryption |
| **Call Service** | Go service used by `call_app` (email auth, billing, call APIs, call static pages) |
| **Call App** | React web app for standalone video/voice calls |
| **SFU/TURN** | WebRTC infrastructure used by the call stack |

## Chat Auth + E2EE (High Level)

Chat no longer uses centralized email/password identity.

- Browser identity:
  - Ed25519 keypair for auth signatures
  - ECDH P-256 keypair for message encryption
  - Private keys are sealed locally; account transfer uses passphrase-encrypted backup export/import
- Relay auth:
  - Host client joins with `role: "host"` and completes `host_auth` by signing `parch-host-auth:<hostUUID>:<challenge>`
  - Relay only marks host online after that host-signing-key challenge succeeds
  - Browser `join_host` checks host responsiveness
  - Relay sends user challenge
  - Browser signs `parch-chat-auth:<hostUUID>:<challenge>:<encPublicKey>`
  - Relay verifies and opens session
- Capability authorization:
  - Host issues short-lived signed space capability tokens on dashboard fetch
  - Browser attaches capability token on `join_channel`, `chat`, and `get_messages`
  - Relay verifies signature/scope/expiry before routing channel actions
- Invites:
  - Done by public key
  - Host resolves public key to host-local user identity
- Messages:
  - Client encrypts to envelope JSON per message (recipient wrapped keys)
  - Relay forwards envelope
  - Host stores envelope JSON (ciphertext)
  - Clients decrypt locally
- Relay abuse controls:
  - websocket read limit
  - per-client chat rate limit
  - encrypted envelope payload and wrapped-key count limits

Account UI:
- Public key copy
- Encrypted identity backup export/import with passphrase
- `Last exported` timestamp
- `Active Devices` list for current host sessions

For the full flow with trust boundaries and examples:
- `docs/chat-e2ee-architecture.md`

---

## Environment Variables

Create a `.env` file in `call_service/` for call features and a separate env for `chat_relay`:

```bash
# call_service (call_app)
CALL_SERVICE_PORT=8000
HOST_DB_FILE=./relay.db                      # local dev; for production prefer an absolute path such as /root/host.db
JWT_SECRET=your-jwt-secret-key
TURN_URL=turn:your-turn-server:3478
TURN_SECRET=your-coturn-static-auth-secret
TURN_API_KEY=optional-api-key-for-turn-endpoint
SFU_SECRET=your-sfu-jwt-secret
EMAIL=your-email@gmail.com
EMAIL_PASSWORD=your-app-password
SMTP_HOST=smtp.gmail.com                    # optional, defaults to smtp.gmail.com
SMTP_PORT=587                               # optional, defaults to 587
PUBLIC_BASE_URL=https://parchchat.com       # optional, used in password-reset links

# chat_relay
CHAT_RELAY_PORT=8001
CHAT_DB_FILE=./chat_relay.db
CHAT_STATIC_DIR=./chat_relay/static

# host_client (official host instance)
OFFICIAL_HOST_UUID=5837a5c3-5268-45e1-9ea4-ee87d959d067
OFFICIAL_SPACE_UUID=parch-community
OFFICIAL_SPACE_NAME="Parch Community"
CHAT_RELAY_HOST=chat.parchchat.com
CHAT_RELAY_SCHEME=https
CHAT_RELAY_WS_SCHEME=wss
```

For deployed `call_service` systemd units, prefer an absolute `HOST_DB_FILE`.
Relative paths resolve from the unit working directory and can silently point
the service at a fresh empty database after a migration or directory move.

## DNS And Caddy

Recommended public DNS:
- `A`/`AAAA` `parchchat.com` -> your VPS public IP
- `A`/`AAAA` `chat.parchchat.com` -> your VPS public IP
- `A`/`AAAA` `sfu.parchchat.com` -> your VPS public IP (or SFU host IP if separate)
- Optional: `CNAME` `www.parchchat.com` -> `parchchat.com`

DNS does not include ports. Caddy listens on public `:80/:443` and proxies internally:
- `parchchat.com` -> `127.0.0.1:8001` (`chat_relay`) except `/call*` + `/ws` routed to `call_service`
- `chat.parchchat.com` -> `127.0.0.1:8001` (`chat_relay`)
- `sfu.parchchat.com` -> `127.0.0.1:7000` (SFU)

Useful Caddy commands:

```bash
# Create local ops/Caddyfile from template (if missing)
./scripts/caddy.sh init

# Pull current remote Caddyfile to ops/Caddyfile
./scripts/caddy.sh fetch

# Validate local file (requires local caddy binary)
./scripts/caddy.sh validate-local

# Apply local Caddyfile to server (remote validate + reload)
./scripts/caddy.sh apply

# Service controls
./scripts/caddy.sh status
./scripts/caddy.sh logs
./scripts/caddy.sh reload
./scripts/caddy.sh restart
```

## Local Development

### Prerequisites
- Go 1.21+
- Node.js 18+
- npm or yarn

### Chat Relay (Web Chat)

```bash
go run ./chat_relay
```

Integration tests:

```bash
./scripts/test-chat-relay-integration.sh
```

Then open:
- `http://localhost:8001/client`

### Call Service (Call App)

```bash
cd call_service
go mod tidy
go run .
```
Then open:
- `http://localhost:8000/call` (on-demand call landing)

### Call App (Video Calls)

```bash
cd call_app
npm install
npm run dev      # Development server at http://localhost:5173
npm run build    # Build to call_service/static/call/
```

**Note:** For local development against production servers, edit `src/config/endpoints.ts`:
```typescript
const isDev = false; // Set to false to use production endpoints
```

### Web Client (Browser)

```bash
cd web_client/frontend
npm install
npm run dev:web      # Vite dev server
npm run build:web    # Build to chat_relay/static/client
npm run test:e2ee    # E2EE crypto tests
```

---

## Database Schema Setup

### Host Client (SQLite)

`host_client` no longer uses migration files.
Its schema is bootstrapped directly in code on startup (idempotent `CREATE TABLE IF NOT EXISTS` + seed rows).

Default host DB path is now:
- `~/.config/ParchHost/host_chat_v2.db`

On startup, host client now checks for incompatible legacy schema in the active DB file and archives it automatically as:
- `host_chat_v2.db.legacy.<UTC timestamp>`

This prevents old DB layouts from interfering with the new decentralized host schema.

### Chat Relay (SQLite)

`chat_relay` no longer uses migration files.
Its schema is bootstrapped directly in code on startup, including compatibility column checks.

### Call Service (SQLite)

Migrations are embedded and run automatically on startup.

---

## Contact

[parchchat@gmail.com](mailto:parchchat@gmail.com)

---

## Deployment Scripts

Build and deploy from local machine:

```bash
# Call service (deploys to /root/call_service_dist, preserves DB files, restarts call-service)
./scripts/deploy-call-service.sh

# Host client (deploys to /root/host_client, preserves DB/config files, restarts parch-host)
./scripts/deploy-host.sh

# Chat relay (deploys to /root/go_chat/chat_relay, preserves DB/env files, restarts chat-relay)
./scripts/deploy-chat-relay.sh
```

If you are migrating an existing host to a fresh `chat_relay.db`, ensure the host row exists in `hosts` using the same `uuid` and `signing_public_key` from `~/.config/ParchHost/host_config.json`:

```bash
sqlite3 /root/go_chat/chat_relay/chat_relay.db \
  "INSERT OR IGNORE INTO hosts (uuid,name,signing_public_key,online) VALUES ('<host_uuid>','<host_name>','<signing_public_key>',0);"
```
