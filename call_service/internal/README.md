# call_service internal packages

This directory defines the target domain boundaries for new `call_service` work.

Current policy:
- New GM marketplace logic should be implemented in `internal/gm`.
- Stripe payment and payout logic should be implemented in `internal/billing`.
- Paid-session room access and join-token validation should be implemented in `internal/callruntime`.
- Cross-cutting setup/middleware utilities belong in `internal/platform`.

`main.go` should only wire routes and bootstrap dependencies.
