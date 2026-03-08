CREATE TABLE IF NOT EXISTS gm_profiles (
    user_id INTEGER PRIMARY KEY,
    display_name TEXT NOT NULL,
    bio TEXT NOT NULL DEFAULT '',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    stripe_connect_account_id TEXT NOT NULL DEFAULT '',
    onboarding_status TEXT NOT NULL DEFAULT 'not_started',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gm_listings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    gm_user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    system_key TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'usd',
    seat_capacity INTEGER NOT NULL CHECK (seat_capacity > 0),
    duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
    is_published INTEGER NOT NULL DEFAULT 0 CHECK (is_published IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(gm_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gm_listings_gm_user_id
    ON gm_listings(gm_user_id);

CREATE INDEX IF NOT EXISTS idx_gm_listings_published
    ON gm_listings(is_published, created_at DESC);

CREATE TABLE IF NOT EXISTS gm_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id INTEGER NOT NULL,
    gm_user_id INTEGER NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    room_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'scheduled',
    seat_capacity INTEGER NOT NULL CHECK (seat_capacity > 0),
    seats_booked INTEGER NOT NULL DEFAULT 0 CHECK (seats_booked >= 0),
    cancel_reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(listing_id) REFERENCES gm_listings(id) ON DELETE CASCADE,
    FOREIGN KEY(gm_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gm_sessions_gm_user_id
    ON gm_sessions(gm_user_id);

CREATE INDEX IF NOT EXISTS idx_gm_sessions_starts_at
    ON gm_sessions(starts_at);

CREATE INDEX IF NOT EXISTS idx_gm_sessions_status
    ON gm_sessions(status);

CREATE TABLE IF NOT EXISTS gm_bookings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    player_user_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_payment',
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'usd',
    platform_fee_bps INTEGER NOT NULL DEFAULT 0 CHECK (platform_fee_bps >= 0),
    platform_fee_cents INTEGER NOT NULL DEFAULT 0 CHECK (platform_fee_cents >= 0),
    stripe_checkout_session_id TEXT UNIQUE,
    stripe_payment_intent_id TEXT UNIQUE,
    stripe_charge_id TEXT UNIQUE,
    refund_cents INTEGER NOT NULL DEFAULT 0 CHECK (refund_cents >= 0),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(session_id, player_user_id),
    FOREIGN KEY(session_id) REFERENCES gm_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY(player_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gm_bookings_session_id
    ON gm_bookings(session_id);

CREATE INDEX IF NOT EXISTS idx_gm_bookings_player_user_id
    ON gm_bookings(player_user_id);

CREATE INDEX IF NOT EXISTS idx_gm_bookings_status
    ON gm_bookings(status);

CREATE TABLE IF NOT EXISTS gm_payout_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    booking_id INTEGER NOT NULL,
    gm_user_id INTEGER NOT NULL,
    gross_cents INTEGER NOT NULL,
    platform_fee_cents INTEGER NOT NULL,
    net_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    ledger_type TEXT NOT NULL,
    stripe_transfer_id TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(booking_id) REFERENCES gm_bookings(id) ON DELETE CASCADE,
    FOREIGN KEY(gm_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gm_payout_ledger_gm_user_id
    ON gm_payout_ledger(gm_user_id);

CREATE INDEX IF NOT EXISTS idx_gm_payout_ledger_booking_id
    ON gm_payout_ledger(booking_id);

CREATE TABLE IF NOT EXISTS gm_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    booking_id INTEGER NOT NULL UNIQUE,
    player_user_id INTEGER NOT NULL,
    gm_user_id INTEGER NOT NULL,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    review_text TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(session_id) REFERENCES gm_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY(booking_id) REFERENCES gm_bookings(id) ON DELETE CASCADE,
    FOREIGN KEY(player_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(gm_user_id) REFERENCES users(id) ON DELETE CASCADE
);
