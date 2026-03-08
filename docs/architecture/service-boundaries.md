# Service Boundaries And Codebase Organization

Status:
- Active engineering guideline.
- Date: 2026-03-08.

## Why this exists

The repo currently ships multiple products/services from one Go module:
- `chat_relay`
- `call_service` (call app backend)
- `host_client`
- `call_app` (React frontend)
- `web_client` (chat frontend)

As we add GM monetization, we need stronger isolation so each service can evolve without accidental coupling.

## Boundary Principles

1. Service boundaries first, feature boundaries second.
- Keep chat concerns in `chat_relay`.
- Keep call/auth/billing/GM concerns in `call_service`.
- Keep host-local chat persistence concerns in `host_client`.

2. New code goes into internal packages, not more flat files at service root.
- Entry points remain small.
- Business logic lives in `internal/*`.

3. Keep ownership explicit.
- Each package should have one reason to change.
- Avoid package-level globals crossing domains.

4. Route handlers should delegate.
- HTTP/WebSocket handlers parse/validate and call service methods.
- DB and Stripe logic should not be embedded directly in route functions.

5. Migrations are domain-scoped by naming.
- Example: `*_gm_*` for GM marketplace tables.
- Example: `*_call_*` for call runtime tables.

## Target Layout (Incremental)

No breaking move to `cmd/` is required immediately.
Keep current entrypoints and migrate internals gradually.

### `call_service` target shape

```text
call_service/
  main.go
  static/
  relay-migrations/
  internal/
    platform/      # env/config, router wiring helpers, shared middleware
    auth/          # call auth + token parsing helpers
    callruntime/   # room/session join authorization, call orchestration
    billing/       # Stripe integration + payment event handling
    gm/            # GM listings, sessions, bookings, payout ledger
```

### `chat_relay` target shape

```text
chat_relay/
  main.go
  internal/
    auth/
    relay/
    schema/
    limits/
```

### `host_client` target shape

```text
host_client/
  main.go
  internal/
    config/
    schema/
    sync/
    authz/
    transport/
```

## Immediate Rules For GM Monetization

All new GM marketplace logic should go under:
- `call_service/internal/gm`
- `call_service/internal/billing` (if Stripe-specific)
- `call_service/internal/callruntime` (if room access-specific)

Allowed in `call_service/main.go`:
- Route registration only.
- No business logic.

Allowed in `call_service/*.go` legacy files:
- Existing behavior maintenance while migrating.
- No new GM business logic unless part of temporary adapter glue.

## Cleanup Priorities (Low Risk First)

1. Remove committed binaries/artifacts from tracked source paths over time.
- Keep deploy outputs under `dist/` only.

2. Establish package boundaries for new work before moving old code.
- Create `internal/*` packages now.
- Add adapters from existing handlers to new package services.

3. Centralize shared concerns.
- Unified auth context extraction.
- Unified error response helper.
- Unified webhook idempotency helper.

4. Add domain-focused tests where logic moves.
- `gm` booking state transitions.
- `billing` webhook idempotency/refund handling.
- `callruntime` paid-session join authorization.

## Migration Plan

Phase 1 (now):
- Add `internal/*` package skeleton in `call_service`.
- Route all new GM code there.
- Keep old files untouched except wiring hooks.

Phase 2:
- Move Stripe marketplace logic out of `stripe.go` into `internal/billing`.
- Move session/room authorization out of `call_rooms.go` into `internal/callruntime`.

Phase 3:
- Move call auth/account logic into `internal/auth`.
- Keep thin compatibility wrappers in old files until stable.

Phase 4:
- Optional: move service entrypoints to `cmd/*` when team wants full mono-repo standardization.

## Anti-Patterns To Avoid

- Adding new `call_service/*.go` flat files for unrelated domains.
- Direct DB calls from route registration or websocket switch blocks.
- Reusing one table for unrelated billing and GM marketplace concerns.
- Coupling `chat_relay` logic into `call_service`.
