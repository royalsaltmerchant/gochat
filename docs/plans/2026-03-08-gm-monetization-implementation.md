# GM Monetization Implementation Plan

Status:
- Draft implementation reference.
- Not committed to roadmap yet.
- Date: 2026-03-08.

## Goal

Enable paid GMs to list sessions, sell seats, run calls, and receive payouts.

Primary success metrics:
- Time to first paid session for a GM: under 14 days.
- Seat fill rate for active listings: >= 60%.
- Refund/dispute rate: < 3%.

## Scope For First Launch (MVP)

Include:
- GM profile setup.
- Public game listings.
- Seat booking with Stripe Checkout.
- Session room access bound to paid booking.
- Basic cancellation/refund rules.
- Stripe Connect payout onboarding for GMs.
- GM earnings and bookings dashboard.

Exclude:
- Discovery ranking algorithm.
- Complex promotion tooling.
- Team co-host payout splitting.
- Deep reputation/trust systems (beyond basic reviews).

## Architecture Fit (Current Repo)

Current relevant components:
- Backend: `call_service` (Gin + SQLite + Stripe) already handles auth and checkout.
- Calls: `call_app` + `call_service/call_rooms.go` for room signaling.
- Existing payments: subscription/credit pack in `call_service/stripe.go`.

Implementation approach:
- Keep monetization backend in `call_service`.
- Reuse existing user auth (`extractUserIDFromAuth`) for GM and player actions.
- Add a separate marketplace payment flow from existing credit/subscription flows.
- Bind room join authorization to booking/session state.

## Data Model (SQLite Migrations)

Add migration: `call_service/relay-migrations/20260308XXXXXX_gm_monetization.up.sql`

### 1) GM profiles

`gm_profiles`
- `user_id INTEGER PRIMARY KEY` (FK users.id)
- `display_name TEXT NOT NULL`
- `bio TEXT NOT NULL DEFAULT ''`
- `timezone TEXT NOT NULL DEFAULT 'UTC'`
- `stripe_connect_account_id TEXT`
- `onboarding_status TEXT NOT NULL DEFAULT 'not_started'`  // not_started, pending, active, restricted
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

### 2) Listings

`gm_listings`
- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `gm_user_id INTEGER NOT NULL` (FK users.id)
- `title TEXT NOT NULL`
- `system_key TEXT NOT NULL` // dnd5e, pf2e, etc
- `description TEXT NOT NULL DEFAULT ''`
- `price_cents INTEGER NOT NULL`
- `currency TEXT NOT NULL DEFAULT 'usd'`
- `seat_capacity INTEGER NOT NULL`
- `duration_minutes INTEGER NOT NULL`
- `is_published INTEGER NOT NULL DEFAULT 0`
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

Indexes:
- `idx_gm_listings_gm_user_id`
- `idx_gm_listings_published`

### 3) Sessions

`gm_sessions`
- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `listing_id INTEGER NOT NULL` (FK gm_listings.id)
- `gm_user_id INTEGER NOT NULL` (FK users.id)
- `starts_at TEXT NOT NULL` // UTC ISO time
- `ends_at TEXT NOT NULL`
- `room_id TEXT NOT NULL UNIQUE`
- `status TEXT NOT NULL DEFAULT 'scheduled'` // scheduled, live, completed, canceled
- `seat_capacity INTEGER NOT NULL`
- `seats_booked INTEGER NOT NULL DEFAULT 0`
- `cancel_reason TEXT NOT NULL DEFAULT ''`
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

Indexes:
- `idx_gm_sessions_gm_user_id`
- `idx_gm_sessions_starts_at`
- `idx_gm_sessions_status`

### 4) Bookings

`gm_bookings`
- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `session_id INTEGER NOT NULL` (FK gm_sessions.id)
- `player_user_id INTEGER NOT NULL` (FK users.id)
- `status TEXT NOT NULL DEFAULT 'pending_payment'` // pending_payment, confirmed, canceled, refunded, disputed
- `amount_cents INTEGER NOT NULL`
- `currency TEXT NOT NULL DEFAULT 'usd'`
- `stripe_checkout_session_id TEXT UNIQUE`
- `stripe_payment_intent_id TEXT UNIQUE`
- `stripe_charge_id TEXT UNIQUE`
- `refund_cents INTEGER NOT NULL DEFAULT 0`
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

Constraints:
- Unique seat per player per session: `UNIQUE(session_id, player_user_id)`

Indexes:
- `idx_gm_bookings_session_id`
- `idx_gm_bookings_player_user_id`
- `idx_gm_bookings_status`

### 5) Payout ledger (append-only)

`gm_payout_ledger`
- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `booking_id INTEGER NOT NULL` (FK gm_bookings.id)
- `gm_user_id INTEGER NOT NULL` (FK users.id)
- `gross_cents INTEGER NOT NULL`
- `platform_fee_cents INTEGER NOT NULL`
- `net_cents INTEGER NOT NULL`
- `currency TEXT NOT NULL DEFAULT 'usd'`
- `ledger_type TEXT NOT NULL` // booking_capture, refund_adjustment, dispute_adjustment, payout
- `stripe_transfer_id TEXT NOT NULL DEFAULT ''`
- `notes TEXT NOT NULL DEFAULT ''`
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

Indexes:
- `idx_gm_payout_ledger_gm_user_id`
- `idx_gm_payout_ledger_booking_id`

### 6) Optional lightweight reviews

`gm_reviews`
- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `session_id INTEGER NOT NULL` (FK gm_sessions.id)
- `booking_id INTEGER NOT NULL UNIQUE` (FK gm_bookings.id)
- `player_user_id INTEGER NOT NULL`
- `gm_user_id INTEGER NOT NULL`
- `rating INTEGER NOT NULL` // 1-5
- `review_text TEXT NOT NULL DEFAULT ''`
- `created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`

## Payment Model (Stripe Connect)

Recommended initial model:
- Stripe Connect Express accounts for GMs.
- Destination charges:
  - Player pays platform checkout.
  - Platform fee captured via `application_fee_amount`.
  - Net amount routed to GM connected account.

Why this model:
- Simple tax/reporting separation.
- Clear platform fee mechanics.
- Works with standard marketplace payout patterns.

Required env vars:
- `STRIPE_PLATFORM_FEE_BPS` (example: 1000 for 10%)
- `STRIPE_CONNECT_REFRESH_URL`
- `STRIPE_CONNECT_RETURN_URL`
- `BASE_URL` (already used)

## End-to-End Commercial Flow Reference

### GM flow (seller side)

1. GM creates account or logs in.
2. GM opens monetization onboarding.
3. Backend creates/fetches Stripe Connect Express account and stores `stripe_connect_account_id`.
4. Backend creates Stripe Account Link and redirects GM to Stripe-hosted onboarding.
5. GM completes identity + payout details on Stripe.
6. Backend refreshes onboarding status and only allows paid listings when account is eligible for charges/payouts.
7. GM creates listing with:
   - `price_cents`
   - `currency`
   - `seat_capacity`
   - `duration_minutes`
8. GM schedules session(s) for the listing.
9. GM runs session and later sees earnings/payout status in dashboard.

### Player flow (buyer side)

1. Player browses listings/sessions.
2. Player clicks "Book seat" on a session.
3. Backend creates pending booking and Stripe Checkout Session.
4. Player pays via Stripe Checkout page.
5. Stripe webhooks confirm payment; backend marks booking confirmed.
6. Seat count increments atomically on the session.
7. Before session start, player requests join token.
8. Backend verifies confirmed booking and time window, returns short-lived join token.
9. Player joins call room with token.

### Funds flow (who pays whom)

Model: destination charge with application fee.

1. Player pays via Stripe Checkout hosted by the platform.
2. Charge is processed through platform Stripe account.
3. `transfer_data[destination]` routes funds to GM connected account.
4. `application_fee_amount` captures platform fee.
5. Stripe processing fees are debited from platform balance.
6. GM receives payout to bank from connected account according to payout schedule.

### Stripe object mapping for one booking

1. Platform booking row -> Stripe Checkout Session.
2. Checkout Session -> PaymentIntent.
3. PaymentIntent -> Charge.
4. Charge + Connect params -> Transfer to GM connected account.
5. Application fee record retained by platform.
6. Connected account balance -> Payout.

## Price, Fee, and Listing Rules

### GM price definition

- GM sets listing `price_cents` and `currency`.
- Session inherits listing price at session creation time.
- Booking stores snapshot values so later listing edits do not alter existing bookings.

Snapshot fields to store on booking:
- `amount_cents`
- `currency`
- `platform_fee_bps`
- `platform_fee_cents`

### Fee strategy recommendation

Baseline recommendation for launch:
- Start with platform take rate in the 10-15% range.
- This includes video/voice infrastructure and core session tooling.

Do not only "charge more because video exists." Charge more when value exists:
- Scheduling, reminders, no-show protection, and booking workflow.
- Reliable session access controls and attendance evidence.
- Optional premium tooling (recording/transcript/high-touch support).

Suggested model:
- Default: 12% marketplace fee (simple to explain).
- High-volume GM program later: lower take rate + monthly subscription.
- Optional add-ons: fixed premium for costly features (recording/AI tooling).

## Refund and Dispute Policy Mechanics

### Refund outcomes

1. Full refund (pre-session cancel): booking -> refunded.
2. Partial refund: booking -> refunded with `refund_cents < amount_cents`.
3. No refund: booking remains completed/no_show based on policy.

### Stripe refund behavior for destination charges

When creating refunds, include:
- `reverse_transfer=true`
- `refund_application_fee=true`

Intent:
- Reverse GM transfer for refunded amount.
- Return platform fee proportionally.

Without these flags, platform can absorb losses while GM keeps transfer.

### Disputes

- On `charge.dispute.created`, mark booking disputed and add negative ledger adjustment.
- Define policy for recovering funds from GM if dispute is lost.
- Keep explicit Terms that describe dispute liability and evidence handling.

## Video Session Facilitation Flow

### Session setup

1. GM session creation generates deterministic `room_id` (example: `gm-session-<id>`).
2. Session metadata stores start/end time and seat capacity.
3. Room is not joinable without valid join token.

### Join authorization

1. Client calls `/call/api/sessions/:id/join-token`.
2. Backend validates:
   - Booking confirmed for player or GM ownership for host role.
   - Session status allows joining.
   - Current time is within allowed join window.
3. Backend signs short-lived join token with claims:
   - `session_id`, `room_id`, `user_id`, `role`, `exp`
4. Client submits join token in websocket join payload.
5. `call_rooms.go` validates token before admitting participant.

### In-session controls

Minimum controls for paid sessions:
- GM role priority.
- Remove participant.
- Lock room.
- Attendance markers (join/leave timestamps).

### Post-session operations

1. Session marked completed.
2. Attendance persisted for dispute/refund evidence.
3. Earnings ledger entries finalized.
4. Optional review prompt to player.

## Webhook Event Handling Matrix

| Stripe event | Expected action | Idempotency key |
|---|---|---|
| `checkout.session.completed` | Confirm booking, increment seats, write capture ledger | `checkout_session_id` |
| `payment_intent.succeeded` | Optional secondary consistency check/log | `payment_intent_id` |
| `charge.refunded` | Mark refunded, add negative ledger, adjust booking refund fields | `charge_id + refund_id` |
| `charge.dispute.created` | Mark disputed, add negative ledger, flag manual review | `dispute_id` |
| `account.updated` (connected account) | Refresh GM onboarding status and payout readiness | `account_id + event_id` |

Implementation rule:
- Each handler must be transaction-safe and retry-safe.

## Operational Economics Guardrails

Track these from launch:
- Gross booking volume.
- Platform fee revenue.
- Stripe fees paid by platform.
- Media infrastructure cost per paid session minute.
- Net contribution margin per booking.

Pricing decision rule:
- If contribution margin drops below target band, adjust:
  - Take rate,
  - feature gating,
  - or premium add-on pricing.

## API Surface (MVP)

Add routes in `call_service/main.go`:

### GM profile and onboarding
- `GET /call/api/gm/profile`
- `PUT /call/api/gm/profile`
- `POST /call/api/gm/connect/onboarding-link`

### Listings
- `POST /call/api/gm/listings`
- `PUT /call/api/gm/listings/:id`
- `POST /call/api/gm/listings/:id/publish`
- `GET /call/api/listings` // public browse
- `GET /call/api/listings/:id`

### Sessions
- `POST /call/api/gm/sessions`
- `PUT /call/api/gm/sessions/:id`
- `POST /call/api/gm/sessions/:id/cancel`
- `GET /call/api/gm/sessions` // GM dashboard

### Booking and checkout
- `POST /call/api/sessions/:id/book`
  - Creates checkout session.
  - Returns `url` for Stripe Checkout redirect.
- `GET /call/api/my/bookings`
- `POST /call/api/bookings/:id/cancel`

### Access control for room join
- `POST /call/api/sessions/:id/join-token`
  - Verifies booking status + session time window.
  - Returns short-lived join token tied to `session_id`, `user_id`, `role`.

## Call Access Integration

Current room join accepts optional account token and open room IDs.
For paid sessions, enforce:
- Session room ID format (example: `gm-session-<sessionId>`).
- `join_call_room` requires valid join token for that room.
- Token claims:
  - `session_id`, `room_id`, `user_id`, `role`, `exp`.
- Join denied if:
  - No confirmed booking (player role).
  - GM account mismatch (GM role).
  - Session status not joinable.
  - Join outside configured time window.

File changes:
- `call_service/call_rooms.go`: add session-aware join validation branch.
- `call_service/types.go`: add join token fields in join payload.

## Webhook Flow (State Machine)

Extend `HandleStripeWebhook`:
- On `checkout.session.completed`:
  - Find pending booking by checkout session ID.
  - Mark booking `confirmed`.
  - Increment `gm_sessions.seats_booked` atomically.
  - Insert `gm_payout_ledger` entry (`booking_capture`).
- On `charge.refunded`:
  - Mark booking `refunded`.
  - Add negative ledger adjustment.
- On `charge.dispute.created`:
  - Mark booking `disputed`.
  - Add negative ledger adjustment.

Idempotency requirements:
- Webhooks may be retried.
- Every handler must be safe to re-run using unique Stripe IDs and transaction guards.

## Reliability Requirements For Monetization

Must-haves before public launch:
- Idempotent checkout create endpoint (idempotency key per booking intent).
- DB transactions for seat reservation and booking state transitions.
- Seat overbooking prevention:
  - Use transaction with `seats_booked < seat_capacity` check and update.
- Retry-safe webhook processing.
- Audit logs for booking/payment transitions.

Operational signals:
- Checkout creation success rate.
- Payment completion rate.
- Overbooking incidents (should be zero).
- Refund/dispute rate.
- Failed payout/onboarding counts.

## Frontend Implementation Outline

Add app surfaces in `call_app`:
- `GM Dashboard`:
  - Profile setup.
  - Connect onboarding status.
  - Listings CRUD.
  - Sessions calendar list.
  - Earnings summary.
- `Session Booking UI`:
  - Public listing page.
  - Session times + seat availability.
  - Book button -> Checkout redirect.
- `My Bookings`:
  - Upcoming sessions with join button.
  - Cancellation and receipt details.

Minimal route strategy:
- Keep current static landing pages.
- Add new React routes under `/call` app shell as needed.

## Suggested File-Level Plan

Backend:
- New: `call_service/internal/gm/service.go`
- New: `call_service/internal/gm/repository.go`
- New: `call_service/internal/gm/http_handlers.go`
- New: `call_service/internal/billing/stripe_marketplace.go`
- New: `call_service/internal/billing/webhooks_marketplace.go`
- New: `call_service/internal/callruntime/session_access.go`
- Update: `call_service/main.go` (route registration/wiring only)
- Update: `call_service/stripe.go` (legacy billing kept until marketplace migration complete)
- Update: `call_service/call_rooms.go` (adapter call into `internal/callruntime` for paid-session join checks)
- New migration files in `call_service/relay-migrations/`

Frontend:
- New pages/components in `call_app/src/`:
  - `pages/GMDashboard.tsx`
  - `pages/ListingsBrowse.tsx`
  - `pages/SessionBooking.tsx`
  - `pages/MyBookings.tsx`
- New service module:
  - `services/gmApi.ts`

## 4-Week Delivery Slice (Realistic MVP)

Week 1:
- Schema migrations.
- GM profile + listing CRUD endpoints.
- Connect onboarding link endpoint.

Week 2:
- Sessions CRUD.
- Booking create endpoint + Stripe Checkout session creation.
- Public listings/session browse endpoints.

Week 3:
- Webhook booking confirmation flow.
- Booking dashboard endpoints.
- Join-token endpoint and room join enforcement.

Week 4:
- GM dashboard UI + booking UI.
- Cancellation/refund baseline.
- QA with 5-10 pilot GMs.

## Acceptance Criteria (MVP)

- GM can onboard Stripe Connect and publish a paid listing.
- Player can book and pay for a seat.
- Successful payment confirms seat and appears in GM dashboard.
- Player can join session room only with valid booking.
- GM receives payout ledger entries with platform fee breakdown.
- No double-confirmation from webhook retries.
- No seat overbooking under concurrent booking attempts.

## Risks and Mitigations

Risk: overbooking on concurrent payments.
- Mitigation: transaction-based seat accounting + reservation timeout logic.

Risk: webhook retries causing duplicate state changes.
- Mitigation: idempotency checks keyed by Stripe IDs.

Risk: GM payout onboarding friction.
- Mitigation: clear onboarding status and blocking checks before listing publish.

Risk: disputes/refunds erode trust.
- Mitigation: explicit cancellation policy + attendance timestamps.

---

If this direction is activated, next step is implementing Week 1 backend tables and endpoints behind a feature flag.
