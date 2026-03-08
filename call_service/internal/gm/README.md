# internal/gm

Owns GM marketplace domain logic:
- GM profiles
- Listings
- Sessions
- Bookings
- Reviews
- Domain validation and lifecycle state transitions

This package should not call Stripe directly unless via billing abstractions.
