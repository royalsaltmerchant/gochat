# Chat Relay

`chat_relay` is the dedicated chat signaling service, separated from `relay_server` (`call_app`).

## Responsibilities

- Host registration and host online/offline status for chat
- Chat websocket routing between web clients and host clients
- Host challenge auth (`host_auth`) and browser public-key auth (`auth_pubkey`)
- Capability-token verification for channel joins/message send/history read
- Routes encrypted chat envelopes
- No persistent centralized user profile storage for chat identities
  - identity persistence lives on host client DBs and browser local identity storage

## WebSocket Authentication

### Host session auth (`host_auth`)

After `join_host` with `{ role: "host" }`, relay sends:

- `host_auth_challenge`

Host responds with:

- `host_auth`

Signing input:

```text
parch-host-auth:<hostUUID>:<challenge>
```

Relay verifies the signature with `hosts.signing_public_key` and only then marks host online.

### Browser session auth (`auth_pubkey`)

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
    "enc_public_key": "<base64 spki p256 encryption public key>",
    "username": "optional username",
    "challenge": "<challenge>",
    "signature": "<base64 raw ed25519 signature>"
  }
}
```

Signing input:

```text
parch-chat-auth:<hostUUID>:<challenge>:<encPublicKey>
```

## Capability Authorization

Host issues short-lived signed capability tokens (currently 5 minutes) in `get_dash_data_response`.

Relay requires and verifies capability tokens for:

- `join_channel` (`join_channel` scope)
- `chat` (`send_message` scope)
- `get_messages` (`read_history` scope)
- `create_channel` (`create_channel` scope)
- `delete_channel` (`delete_channel` scope)
- `invite_user` (`invite_user` scope)
- `remove_space_user` (`remove_space_user` scope)
- `delete_space` (`delete_space` scope)

Relay checks:

- token signature (host signing key)
- host/space/subject binding
- expiry and issued-at validity
- required scope and optional channel scope

## Encrypted Message Routing

- Browser sends `chat` with:
  - `data.envelope` (ciphertext payload + wrapped keys + signature)
- Relay broadcasts envelope to channel subscribers and forwards to host for persistence.
- Relay does not decrypt message bodies.

## API

- `GET /ws`
- `GET /healthz`
- `GET /api/host/:uuid`
- `POST /api/hosts_by_uuids`
- `POST /api/register_host`
- `GET /client`

## Environment Variables

- `CHAT_RELAY_PORT` (default `8001`)
- `CHAT_DB_FILE` (default `./chat_relay.db`)
- `CHAT_STATIC_DIR` (default `./chat_relay/static`)

## Local Run

```bash
go run ./chat_relay
```

## Integration Tests

```bash
./scripts/test-chat-relay-integration.sh
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
- Applies compatibility schema updates, including:
  - `hosts.online` column backfill
  - migration from legacy `hosts.author_id` schema to signing-key schema
