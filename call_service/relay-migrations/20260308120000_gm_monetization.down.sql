DROP TABLE IF EXISTS gm_reviews;

DROP INDEX IF EXISTS idx_gm_payout_ledger_booking_id;
DROP INDEX IF EXISTS idx_gm_payout_ledger_gm_user_id;
DROP TABLE IF EXISTS gm_payout_ledger;

DROP INDEX IF EXISTS idx_gm_bookings_status;
DROP INDEX IF EXISTS idx_gm_bookings_player_user_id;
DROP INDEX IF EXISTS idx_gm_bookings_session_id;
DROP TABLE IF EXISTS gm_bookings;

DROP INDEX IF EXISTS idx_gm_sessions_status;
DROP INDEX IF EXISTS idx_gm_sessions_starts_at;
DROP INDEX IF EXISTS idx_gm_sessions_gm_user_id;
DROP TABLE IF EXISTS gm_sessions;

DROP INDEX IF EXISTS idx_gm_listings_published;
DROP INDEX IF EXISTS idx_gm_listings_gm_user_id;
DROP TABLE IF EXISTS gm_listings;

DROP TABLE IF EXISTS gm_profiles;
