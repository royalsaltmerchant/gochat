# internal/billing

Owns payment and payout integration logic:
- Stripe Checkout session creation for marketplace bookings
- Stripe Connect onboarding and account readiness checks
- Webhook event handling and idempotency guards
- Refund/dispute ledger adjustments

Domain rules (GM session authorization, seat accounting) should remain in `internal/gm` or `internal/callruntime`.
