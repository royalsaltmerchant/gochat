# Chat System Architecture (Public-Key + E2EE)

This document explains how chat works after the hard cut away from centralized chat identity auth.

Related decision record:
- `docs/chat-e2ee-key-design-decision.md`

Scope:
- `web_client` (browser chat app)
- `chat_relay` (routing/signaling)
- `host_client` (host-owned chat data + permissions)

Out of scope:
- `relay_server` and `call_app` auth/billing/calls (unchanged)

## 1) Design Goals

- No email/password auth for chat.
- No centralized chat identity table in relay.
- Identity controlled by user key material.
- Relay passes encrypted chat payloads; host stores encrypted payloads.
- Host remains the authority for spaces/channels/membership/invites.

## 2) Components And Data Ownership

- Browser (`web_client`)
  - Generates identity locally.
  - Stores public identity metadata in `localStorage`.
  - Stores private key material in a sealed local record (device key vault path when available).
  - Signs auth challenges.
  - Encrypts/decrypts chat messages.
  - Stores optional local fallback username.

- Chat Relay (`chat_relay`)
  - Verifies auth challenge signatures.
  - Tracks live client sessions for routing.
  - Forwards membership/message events to/from host.
  - Does not persist user profiles for chat identities.

- Host Client (`host_client`)
  - Persists spaces/channels/memberships/invites/messages in SQLite.
  - Maps authenticated public keys to host-local users.
  - Enforces authorization and invite acceptance.
  - Returns space user lists with auth/encryption public keys.

## 3) Identity Model

Each browser identity now has:
- Auth keypair: Ed25519
  - `publicKey`, `privateKey`
- Encryption keypair: ECDH P-256
  - `encPublicKey`, `encPrivateKey`
- Optional local fallback username
  - Stored with local identity metadata

Identity import/export:
- Export in account UI produces passphrase-encrypted backup JSON.
- Import in account UI requires passphrase and validates key material before replacing local identity.
- Legacy plaintext identity JSON is still migration-compatible for existing local users.
- Without importing identity on a new browser/device, user appears as a new account identity.

## 4) Authentication Flow

### 4.1 Host-to-relay auth

1. Host client opens websocket and sends `join_host` with `role: "host"`.
2. Relay treats this as a host-auth candidate only when:
   - role is `host`, and
   - request has no browser `Origin` header.
3. Relay sends `host_auth_challenge`.
4. Host signs:
   - `parch-host-auth:<hostUUID>:<challenge>`
5. Host sends `host_auth` with challenge + signature.
6. Relay verifies signature using `hosts.signing_public_key`.
7. On success, relay marks that connection as the host-author socket and sets host online.

### 4.2 Browser-to-relay auth

1. Browser client opens websocket and sends `join_host`.
2. Relay host-availability gate:
   - relay sends `relay_health_check` to authenticated host-author socket
   - waits for `relay_health_check_ack` (timeout: 2 seconds)
   - if host is offline/unresponsive, relay returns `join_error`
3. Relay sends `auth_challenge`.
4. Browser signs:
   - `parch-chat-auth:<hostUUID>:<challenge>:<encPublicKey>`
5. Browser sends `auth_pubkey` payload:
   - `public_key`
   - `enc_public_key`
   - `device_id`
   - `device_name`
   - `username` (local fallback, optional)
   - `challenge`
   - `signature`
6. Relay verifies Ed25519 signature and authenticates the session.
7. Relay routes subsequent events with session identity (`user_id`, auth key, enc key, normalized username).

Security properties:
- Host online state is bound to possession of the host signing private key (not a database `author_id` value).
- Binding `enc_public_key` into browser signed challenge prevents key-substitution during auth.

## 5) Username Semantics

- Browser-level fallback username:
  - User sets it locally once (account modal).
  - Used in auth payload as default when connecting to hosts.

- Host-level username record:
  - Host stores per-identity username in `chat_users`.
  - Update Username writes to host DB and becomes authoritative for that host.

Result:
- Same identity across multiple hosts can have different usernames per host.
- Same host, multiple browsers with imported same identity resolve to same host user.

## 6) Capability Token Authorization

Host issues short-lived per-space capability tokens and relay verifies them for realtime channel actions.

Issuance:
- Tokens are issued by host in:
  - `get_dash_data_response` (all joined spaces)
  - `create_space_success` and `accept_invite_success` (newly joined spaces)
- Token TTL is currently 5 minutes.
- Claims include:
  - version
  - `host_uuid`
  - `space_uuid`
  - subject auth public key (`sub`)
  - scopes
  - expiry + issued-at
  - token id
  - optional channel scope (currently `*`)

Relay enforcement (current):
- `join_channel` requires scope: `join_channel`
- `chat` requires scope: `send_message`
- `get_messages` requires scope: `read_history`
- `create_channel` requires scope: `create_channel`
- `delete_channel` requires scope: `delete_channel`
- `invite_user` requires scope: `invite_user`
- `remove_space_user` requires scope: `remove_space_user`
- `delete_space` requires scope: `delete_space`

Relay validates:
- host signature on token
- host/space/subject binding
- expiry and issued-at sanity
- required scope and channel scope match

Client refresh behavior:
- Browser caches capabilities keyed by `space_uuid`.
- Before capability-gated actions, browser refreshes via `get_dash_data` if token is near expiry.
- On `Unauthorized ...` relay error, browser retries once after forced capability refresh.

## 7) Invite Semantics

Invite target:
- Invites are by **public key** (not email).

Current requirement:
- Target identity should have connected at least once so host can resolve that public key cleanly.

Invite acceptance:
- Accept/decline/leave/remove flows now include both:
  - `user_public_key`
  - `user_enc_public_key`
  to keep identity resolution stable across relay/host handoffs.

Official host/space:
- Host can auto-create pending invite for official space on dashboard fetch when not already joined/invited.

## 8) E2EE Message Format And Flow

### 7.1 Envelope

Messages are sent/stored as encrypted envelope JSON:
- metadata: version/algorithm/space/channel/sender keys
- `ciphertext` + `content_iv` (AES-GCM)
- `wrapped_keys[]` (one wrapped message key per recipient auth public key)
- `sig` (Ed25519 signature over canonical envelope fields)

### 7.2 Send

1. Sender gets current space members and their `public_key` + `enc_public_key`.
2. Sender creates random AES message key.
3. Sender encrypts plaintext with AES-GCM.
4. For each recipient, sender derives ECDH shared secret -> HKDF key -> wraps AES message key.
5. Sender signs canonical envelope with Ed25519 auth private key.
6. Sender sends `{ envelope }` over websocket.

### 7.3 Receive

1. Receiver verifies envelope signature against sender auth public key.
2. Receiver finds its wrapped key entry by recipient auth public key.
3. Receiver derives unwrap key (ECDH + HKDF) and unwraps message key.
4. Receiver decrypts ciphertext with AES-GCM.

Tamper detection:
- Signature failure rejects modified metadata/wrapped key fields.
- AES-GCM authentication rejects ciphertext/IV/tag tampering.

Test coverage:
- `web_client/frontend/js/lib/e2ee.test.mjs` includes:
  - message_id tamper rejection
  - ciphertext tamper rejection
  - signature tamper rejection

## 9) Storage And Trust Boundaries

- Relay sees and forwards ciphertext envelope; no plaintext.
- Host stores ciphertext envelope JSON in `messages.content`.
- Host still sees membership and identity key metadata needed for routing/invites.
- Browser is the only place that can decrypt message bodies.

Relay-side abuse controls:
- websocket read limit: 256 KiB
- chat rate limit per client session
- envelope size cap
- wrapped recipient-key count cap

Host-side storage guard:
- oversized envelopes are rejected before DB insert

## 10) What Changed Vs Legacy Chat Auth

Removed for chat path:
- email/password login
- centralized chat identity storage in relay

Kept unchanged:
- `relay_server` + `call_app` user/email auth and billing paths
- call infrastructure (SFU/TURN/call APIs)

## 11) Operational Notes

- Build web bundle into `chat_relay/static/client`.
- Deploy `chat_relay` and `host_client` from local scripts.
- Relay server deploy is optional unless landing/call_app changed.

Recommended validation after deploy:
- New browser can authenticate with generated keypair.
- Account modal shows copyable public key + encrypted backup export/import.
- Account modal shows `Last exported` and `Active Devices`.
- Invite by public key succeeds.
- Messages decrypt for joined members.
- Tampered payloads fail to decrypt/verify.
- `join_host` for browser clients fails quickly when host is offline/unresponsive.
