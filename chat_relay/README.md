# Chat Relay

`chat_relay` is the dedicated chat signaling service, separated from `relay_server` (`call_app`).

## Responsibilities

- Host registration and host online/offline status for chat
- Chat websocket routing between web clients and host clients
- Public-key identity auth (`auth_pubkey`)
- User lookup APIs for pubkey invite flow

## Public-Key Auth

Server sends an auth challenge after `join_host`:

- `auth_challenge`

Client responds with:

- `auth_pubkey`

Payload:

```json
{
  "type": "auth_pubkey",
  "data": {
    "public_key": "<base64 raw ed25519 public key>",
    "username": "optional username",
    "challenge": "<challenge>",
    "signature": "<base64 raw ed25519 signature>"
  }
}
```

Signing input:

```text
parch-chat-auth:<hostUUID>:<challenge>
```

## API

- `GET /ws`
- `GET /healthz`
- `GET /api/host/:uuid`
- `POST /api/hosts_by_uuids`
- `POST /api/register_host`
- `POST /api/host_offline/:uuid`
- `POST /api/user_by_pubkey`
- `POST /api/user_by_id`
- `POST /api/users_by_ids`
- `GET /client`

## Environment Variables

- `CHAT_RELAY_PORT` (default `8001`)
- `CHAT_DB_FILE` (default `./chat_relay.db`)
- `CHAT_STATIC_DIR` (default `./chat_relay/static`)

## Local Run

```bash
go run ./chat_relay
```

Open:

- `http://localhost:8001/client`

## Build

```bash
./scripts/build-chat-relay.sh
```

Output:

- `chat_relay/dist/chat_relay`

## Database Bootstrap

`chat_relay` no longer depends on migration files.

On startup it now:
- Opens SQLite with foreign keys enabled
- Creates required tables/indexes if missing
- Adds any missing compatibility columns (`hosts.online`, `chat_identities.created_at`, `chat_identities.updated_at`)
