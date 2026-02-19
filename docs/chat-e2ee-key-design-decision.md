# Chat E2EE Key Design Decision

Date: 2026-02-17
Status: Accepted (current production behavior)
Scope: `web_client`, `chat_relay`, `host_client`

## Decision

Parch chat uses:

- Long-lived identity keys per browser identity:
  - Ed25519 keypair for authentication/signatures
  - ECDH P-256 keypair for message-key wrapping
- Per-message random AES-GCM content keys for message ciphertext.

This means the ECDH keypair is identity-bound and persisted, not ephemeral per message/session.

## Why This Was Chosen

1. Async group messaging reliability.
   Store-and-forward envelopes can be created and decrypted without live handshake/session state.
2. Simpler system behavior.
   No prekey inventory management, ratchet state, skipped-key handling, or session repair logic.
3. Stable host identity mapping.
   Invite/member flows are keyed by persistent public keys.
4. Lower operational risk for current stage.
   Fewer moving parts across browser, relay, and host DB.

## Trade-Offs

### Benefits

- Works well for offline recipients in multi-member spaces.
- Straightforward implementation and debugging.
- Low protocol/state complexity across devices and services.

### Costs

- No full forward secrecy for historical wrapped message keys.
- No post-compromise security recovery semantics from a ratchet.
- If a long-term encryption private key is compromised, stored historical envelopes may be at higher risk than in a ratcheting design.

## Alternatives Considered

1. Sender-ephemeral ECDH per message.
   Improves some properties, but does not provide full Signal-level guarantees by itself.
2. Async prekey bootstrap (identity key + signed prekey + one-time prekeys).
   Stronger offline-init security but adds server/host lifecycle complexity.
3. Full X3DH + Double Ratchet.
   Best security properties, highest implementation and maintenance complexity.

## Revisit Triggers

Re-evaluate this decision if any of the following becomes true:

- Product/security requirements explicitly require forward secrecy and post-compromise security.
- Multi-device encrypted session consistency becomes a top priority.
- Team is ready to absorb protocol/state complexity and migration overhead.

## Incremental Upgrade Path (When Prioritized)

1. Add envelope versioning and sender-ephemeral public key support.
2. Add signed prekeys and one-time prekeys for offline session bootstrap.
3. Add per-peer/device ratcheting session state and recovery handling.

## Code References

- Identity key generation/persistence:
  - `web_client/frontend/js/lib/identityManager.js`
- E2EE envelope encrypt/decrypt and key wrapping:
  - `web_client/frontend/js/lib/e2ee.js`
- Auth challenge binds `encPublicKey`:
  - `web_client/frontend/js/lib/socketConn.js`
  - `chat_relay/auth_pubkey.go`
